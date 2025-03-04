// Copyright 2025 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package biz

import (
	"math"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	"github.com/newrelic/nri-docker/src/raw"
)

func (mc *MetricsFetcher) memory(mem raw.Memory) Memory {
	memLimits := mem.UsageLimit
	// ridiculously large memory limits are set to 0 (no limit)
	if memLimits > math.MaxInt64/2 {
		memLimits = 0
	}

	usagePercent := float64(0)
	if memLimits > 0 {
		usagePercent = 100 * float64(mem.RSS) / float64(memLimits)
	}

	/* Dockers includes non-swap memory into the swap limit
	(https://docs.docker.com/config/containers/resource_constraints/#--memory-swap-details)
	convention followed for metric naming:
	* Metrics with no swap reference in the name have no swap components
	* Metrics with swap reference have memory+swap unless the contrary is specified like in memorySwapOnlyUsageBytes
	*/

	softLimit := mem.SoftLimit
	if mem.SoftLimit > math.MaxInt64/2 {
		softLimit = 0
	}

	swapLimit := mem.SwapLimit
	if mem.SwapLimit > math.MaxInt64/2 {
		swapLimit = 0
	}

	m := Memory{
		MemLimitBytes:    memLimits,
		CacheUsageBytes:  mem.Cache,
		RSSUsageBytes:    mem.RSS,
		UsageBytes:       mem.RSS,
		UsagePercent:     usagePercent,
		KernelUsageBytes: mem.KernelMemoryUsage,
		SoftLimitBytes:   softLimit,
		SwapLimitBytes:   swapLimit,
	}

	/*
			https://www.kernel.org/doc/Documentation/cgroup-v1/memory.txt

		For efficiency, as other kernel components, memory cgroup uses some optimization
		to avoid unnecessary cacheline false sharing. usage_in_bytes is affected by the
		method and doesn't show 'exact' value of memory (and swap) usage, it's a fuzz
		value for efficient access. (Of course, when necessary, it's synchronized.)
		If you want to know more exact memory usage, you should use RSS+CACHE(+SWAP)
		value in memory.stat(see 5.2).
		However, as the `docker stats` cli tool does, page cache is intentionally
		excluded to avoid misinterpretation of the output.
		Also mem.SwapUsage is parsed from memory.memsw.usage_in_bytes, which
		according to the documentation reports the sum of current memory usage
		plus swap space used by processes in the cgroup (in bytes). That's why
		Usage is subtracted from the Swap: to get the actual swap.
	*/
	if mem.SwapUsage == nil {
		return m
	}

	// We should make sure that FuzzUsage is never > that SwapUsage (when SwapUsage != 0),
	// otherwise we face an overflow since both are unsigned integers
	if *mem.SwapUsage != 0 && mem.FuzzUsage > *mem.SwapUsage {
		log.Debug("Swap metrics not collected since %d>%d", mem.FuzzUsage, *mem.SwapUsage)
		return m
	}

	var swapOnlyUsage uint64
	if *mem.SwapUsage != 0 { // for systems that have swap disabled
		swapOnlyUsage = *mem.SwapUsage - mem.FuzzUsage
	}
	swapUsage := mem.RSS + swapOnlyUsage

	swapLimitUsagePercent := float64(0)
	// Notice that swapLimit could be 0 also if the container swap has no limit (=-1)
	// This happens because is transformed into MaxInt-1 (due to the uint conversion)
	// that is then ignored since it is bigger than math.MaxInt64/2
	if swapLimit > 0 {
		swapLimitUsagePercent = 100 * float64(swapUsage) / float64(swapLimit)
	}

	m.SwapUsageBytes = &swapUsage
	m.SwapOnlyUsageBytes = &swapOnlyUsage
	m.SwapLimitUsagePercent = &swapLimitUsagePercent

	return m
}

func (mc *MetricsFetcher) cpu(metrics raw.Metrics, json *types.ContainerJSON) CPU {
	previous := StoredCPUSample{}
	// store current metrics to be the "previous" metrics in the next CPU sampling
	defer func() {
		previous.Time = metrics.Time.Unix()
		previous.CPU = metrics.CPU
		mc.store.Set(metrics.ContainerID, previous)
	}()

	cpu := CPU{}

	// Set LimitCores to first honor CPU quota if any; otherwise set it to runtime.CPU().
	if json.HostConfig != nil && json.HostConfig.NanoCPUs != 0 {
		cpu.LimitCores = float64(json.HostConfig.NanoCPUs) / 1e9
	} else {
		// TODO: if newrelic-infra is in a limited cpus container, this may report the number of cpus of the
		// 	newrelic-infra container if the container has no CPU quota
		cpu.LimitCores = float64(mc.getRuntimeNumCPU())
	}

	// Reading previous CPU stats
	if _, err := mc.store.Get(metrics.ContainerID, &previous); err != nil {
		log.Debug("could not retrieve previous CPU stats for container %v: %v", metrics.ContainerID, err.Error())
		return cpu
	}

	// calculate the change for the cpu usage of the container in between readings
	durationNS := float64(metrics.Time.Sub(time.Unix(previous.Time, 0)).Nanoseconds())
	if durationNS <= 0 {
		return cpu
	}

	//nolint: mnd
	maxVal := float64(metrics.CPU.OnlineCPUsWithFallback() * 100)

	cpu.CPUPercent = cpuPercent(previous.CPU, metrics.CPU)

	userDelta := float64(*metrics.CPU.UsageInUsermode - *previous.CPU.UsageInUsermode)
	cpu.UserPercent = math.Min(maxVal, userDelta*100/durationNS)

	kernelDelta := float64(*metrics.CPU.UsageInKernelmode - *previous.CPU.UsageInKernelmode)
	cpu.KernelPercent = math.Min(maxVal, kernelDelta*100/durationNS)

	cpu.UsedCores = float64(metrics.CPU.TotalUsage-previous.CPU.TotalUsage) / durationNS

	cpu.ThrottlePeriods = metrics.CPU.ThrottledPeriods
	cpu.ThrottledTimeMS = float64(metrics.CPU.ThrottledTimeNS) / 1e9 // nanoseconds to second

	cpu.UsedCoresPercent = 100 * cpu.UsedCores / cpu.LimitCores

	cpu.Shares = metrics.CPU.Shares

	return cpu
}

func cpuPercent(previous, current raw.CPU) float64 {
	var (
		cpuPercent = 0.0
		// calculate the change for the cpu usage of the container in between readings
		cpuDelta = float64(current.TotalUsage - previous.TotalUsage)
		// calculate the change for the entire system between readings
		systemDelta = float64(current.SystemUsage - previous.SystemUsage)
		onlineCPUs  = float64(current.OnlineCPUsWithFallback())
	)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}
	return cpuPercent
}
