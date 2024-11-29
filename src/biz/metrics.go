// Package biz provides business-value metrics from system raw metrics
package biz

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	"github.com/newrelic/infra-integrations-sdk/v3/persist"
	gops_mem "github.com/shirou/gopsutil/mem"

	"github.com/newrelic/nri-docker/src/raw"
)

var ErrExitedContainerExpired = errors.New("container exited TTL expired")
var ErrExitedContainerUnexpired = errors.New("exited containers have no metrics to fetch")

// Sample exports the valuable metrics from a container
type Sample struct {
	Pids         Pids
	Network      Network
	BlkIO        BlkIO
	CPU          CPU
	Memory       Memory
	RestartCount int
}

// Pids section of a container sample
type Pids raw.Pids

// Network section of a container sample
type Network raw.Network

// BlkIO stands for Block I/O stats
type BlkIO struct {
	TotalReadCount  *float64
	TotalWriteCount *float64
	TotalReadBytes  *float64
	TotalWriteBytes *float64
}

// CPU metrics
type CPU struct {
	CPUPercent       float64
	KernelPercent    float64
	UserPercent      float64
	UsedCores        float64
	LimitCores       float64
	UsedCoresPercent float64
	ThrottlePeriods  uint64
	ThrottledTimeMS  float64
	Shares           uint64
	NumProcs         uint32
}

// Memory metrics
type Memory struct {
	UsageBytes       uint64
	CacheUsageBytes  uint64
	RSSUsageBytes    uint64
	MemLimitBytes    uint64
	UsagePercent     float64 // Usage percent from the limit, if any
	KernelUsageBytes uint64
	SoftLimitBytes   uint64
	SwapLimitBytes   uint64

	// The swap metrics depending on swap usage are computed only if it is reported
	SwapUsageBytes        *uint64
	SwapOnlyUsageBytes    *uint64
	SwapLimitUsagePercent *float64

	// windows memory
	Commit            uint64
	CommitPeak        uint64
	PrivateWorkingSet uint64
}

// Processer defines the most essential interface of an exportable container Processer
type Processer interface {
	Process(containerID string) (Sample, error)
}

// MetricsFetcher fetches the container system-level metrics from different sources and processes it to export
// metrics with business-value
type MetricsFetcher struct {
	store              persist.Storer
	fetcher            raw.Fetcher
	inspector          raw.DockerInspector
	exitedContainerTTL time.Duration
	getRuntimeNumCPU   func() int
}

// NewProcessor creates a MetricsFetcher from implementations of its required components
func NewProcessor(store persist.Storer, fetcher raw.Fetcher, inspector raw.DockerInspector, exitedContainerTTL time.Duration) *MetricsFetcher {
	return &MetricsFetcher{
		store:              store,
		fetcher:            fetcher,
		inspector:          inspector,
		exitedContainerTTL: exitedContainerTTL,
		getRuntimeNumCPU:   runtime.NumCPU,
	}
}

// WithRuntimeNumCPUfunc changes the NumCPU counting func so MetricsFetcher is mockable
func (mc *MetricsFetcher) WithRuntimeNumCPUfunc(rcFunc func() int) {
	mc.getRuntimeNumCPU = rcFunc
}

// Process returns a metrics Sample of the container with the given ID
func (mc *MetricsFetcher) Process(containerID string) (Sample, error) {
	metrics := Sample{}

	json, err := mc.inspector.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return metrics, err
	}
	if json.ContainerJSONBase == nil {
		return metrics, errors.New("empty container inspect result")
	}

	// TODO: move logic to skip container without State to Docker specific code.
	if json.State == nil {
		log.Debug("invalid container %s JSON: missing State", containerID)
	}

	if json.State != nil && strings.ToLower(json.State.Status) == "exited" {
		expired, err := mc.isExpired(json.State.FinishedAt) //nolint: govet //shadowed err
		if err != nil {
			return metrics, fmt.Errorf("verifying container expiration: %w", err)
		}
		if expired {
			return metrics, ErrExitedContainerExpired
		}

		// There are no metrics to fetch from an exited container.
		return metrics, ErrExitedContainerUnexpired
	}

	// Fetch metrics from non exited containers
	rawMetrics, err := mc.fetcher.Fetch(json)
	if err != nil {
		return metrics, err
	}

	metrics.Network = Network(rawMetrics.Network)
	metrics.BlkIO = mc.blkIO(rawMetrics.Blkio, rawMetrics.Platform)
	metrics.CPU = mc.cpu(rawMetrics, &json)
	metrics.Pids = Pids(rawMetrics.Pids)
	metrics.Memory = mc.memory(rawMetrics.Memory, rawMetrics.Platform)
	metrics.RestartCount = json.RestartCount

	return metrics, nil
}

func (mc *MetricsFetcher) isExpired(finishedAt string) (bool, error) {
	if mc.exitedContainerTTL == 0 {
		return false, nil
	}

	exitTimestamp, err := time.Parse(time.RFC3339Nano, finishedAt)
	if err != nil {
		return false, fmt.Errorf("invalid finished_at timestamp: %s (%w)", finishedAt, err)
	}

	if time.Since(exitTimestamp) > mc.exitedContainerTTL {
		return true, nil
	}

	return false, nil
}

func (mc *MetricsFetcher) cpu(metrics raw.Metrics, json *types.ContainerJSON) CPU {
	var previous struct {
		Time int64
		CPU  raw.CPU
	}
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

	maxVal := float64(metrics.CPU.OnlineCPUsWithFallback() * 100)

	if json.Platform == "windows" {
		cpu.CPUPercent = calculateCPUPercentWindows(metrics.CPU.Read, metrics.CPU.PreRead, metrics.CPU.NumProcs, metrics.CPU.TotalUsage, previous.CPU.TotalUsage)
		cpu.NumProcs = metrics.CPU.NumProcs
	} else {
		cpu.CPUPercent = cpuPercent(previous.CPU, metrics.CPU)
	}

	userDelta := float64(metrics.CPU.UsageInUsermode - previous.CPU.UsageInUsermode)
	cpu.UserPercent = math.Min(maxVal, userDelta*100/durationNS)

	kernelDelta := float64(metrics.CPU.UsageInKernelmode - previous.CPU.UsageInKernelmode)
	cpu.KernelPercent = math.Min(maxVal, kernelDelta*100/durationNS)

	cpu.UsedCores = float64(metrics.CPU.TotalUsage-previous.CPU.TotalUsage) / durationNS

	cpu.ThrottlePeriods = metrics.CPU.ThrottledPeriods
	cpu.ThrottledTimeMS = float64(metrics.CPU.ThrottledTimeNS) / 1e9 // nanoseconds to second

	cpu.UsedCoresPercent = 100 * cpu.UsedCores / cpu.LimitCores

	cpu.Shares = metrics.CPU.Shares

	return cpu
}

func (mc *MetricsFetcher) memory(mem raw.Memory, platform string) Memory {
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

	if platform == "windows" {
		m.Commit = mem.Commit
		m.CommitPeak = mem.CommitPeak
		m.PrivateWorkingSet = mem.PrivateWorkingSet

		vmem, err := gops_mem.VirtualMemory()
		if err != nil {
			panic(err)
		}
		totalMemory := vmem.Total
		memoryUsagePercent := float64(m.PrivateWorkingSet) / float64(totalMemory) * 100
		m.UsagePercent = memoryUsagePercent
	}

	return m
}

func (mc *MetricsFetcher) blkIO(blkio raw.Blkio, platform string) BlkIO {
	bio := BlkIO{}
	for _, svc := range blkio.IoServicedRecursive {
		if len(svc.Op) == 0 {
			continue
		}
		var count = float64(svc.Value)
		switch svc.Op[0] {
		case 'r', 'R':
			if bio.TotalReadCount == nil {
				bio.TotalReadCount = new(float64)
			}
			*bio.TotalReadCount += count
		case 'w', 'W':
			if bio.TotalWriteCount == nil {
				bio.TotalWriteCount = new(float64)
			}
			*bio.TotalWriteCount += count
		}
	}
	for _, bytes := range blkio.IoServiceBytesRecursive {
		if len(bytes.Op) == 0 {
			continue
		}
		var bCount = float64(bytes.Value)
		switch bytes.Op[0] {
		case 'r', 'R':
			if bio.TotalReadBytes == nil {
				bio.TotalReadBytes = new(float64)
			}
			*bio.TotalReadBytes += bCount
		case 'w', 'W':
			if bio.TotalWriteBytes == nil {
				bio.TotalWriteBytes = new(float64)
			}
			*bio.TotalWriteBytes += bCount
		}
	}
	if platform == "windows" {
		if bio.TotalReadBytes == nil {
			bio.TotalReadBytes = new(float64)
		}
		*bio.TotalReadBytes = float64(blkio.BlkReadSizeBytes)
		if bio.TotalWriteBytes == nil {
			bio.TotalWriteBytes = new(float64)
		}
		*bio.TotalWriteBytes = float64(blkio.BlkWriteSizeBytes)
	}
	return bio
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

func calculateCPUPercentWindows(read time.Time, preread time.Time, numprocs uint32, totalUsage uint64, preTotalUsage uint64) float64 {
	// Max number of 100ns intervals between the previous time read and now
	possIntervals := uint64(read.Sub(preread).Nanoseconds()) // Start with number of ns intervals
	possIntervals /= 100                                     // Convert to number of 100ns intervals
	possIntervals *= uint64(numprocs)                        // Multiple by the number of processors

	// Intervals used
	intervalsUsed := totalUsage - preTotalUsage

	// Percentage avoiding divide-by-zero
	if possIntervals > 0 {
		return float64(intervalsUsed) / float64(possIntervals) * 100.0
	}
	return 0.00
}
