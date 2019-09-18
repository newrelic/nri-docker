package user

import (
	"math"
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-docker/src/system"
)

type CPU struct {
	CPUPercent      float64
	KernelPercent   float64
	UserPercent     float64
	UsedCores       float64
	ThrottlePeriods uint64
	ThrottledTimeMS float64
}

func (mc *MetricsCollector) CPU(metrics system.Metrics) CPU {
	var previous struct {
		Time   int64
		SysCPU system.CPU
	}
	// store current metrics to be the "previous" metrics in the next CPU sampling
	defer func() {
		previous.Time = metrics.Time.Unix()
		previous.SysCPU = metrics.CPU
		mc.store.Set(metrics.ContainerID, previous)
	}()

	cpu := CPU{}
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

	maxVal := float64(len(metrics.CPU.PercpuUsage) * 100)

	cpu.CPUPercent = calculateCPUPercentUnix(c.PreCPUStats.CPUUsage.TotalUsage, c.PreCPUStats.SystemUsage, &c.CPUStats)

	userDelta := float64(c.CPUStats.CPUUsage.UsageInUsermode - c.PreCPUStats.CPUUsage.UsageInUsermode)
	cpu.UserPercent = math.Min(maxVal, userDelta*100/duration)

	kernelDelta := float64(c.CPUStats.CPUUsage.UsageInKernelmode - c.PreCPUStats.CPUUsage.UsageInKernelmode)
	cpu.KernelPercent = math.Min(maxVal, kernelDelta*100/duration)

	cpu.UsedCores = float64(c.CPUStats.CPUUsage.TotalUsage-c.PreCPUStats.CPUUsage.TotalUsage) / duration

	cpu.ThrottlePeriods = metrics.CPU.ThrottledPeriods
	cpu.ThrottledTimeMS = float64(metrics.CPU.ThrottledTimeNS) / 1e9 // nanoseconds to second

	return cpu
}

// TODO: add other metrics such as swap and memsw limit
func (c *Cooked) Memory() Memory {
	var cache, rss float64
	if icache, ok := c.MemoryStats.Stats["cache"]; ok {
		cache = float64(icache)
	}
	if irss, ok := c.MemoryStats.Stats["rss"]; ok {
		rss = float64(irss)
	}
	return Memory{
		UsageBytes:      float64(c.MemoryStats.Usage),
		CacheUsageBytes: cache,
		RSSUsageBytes:   rss,
		MemLimitBytes:   float64(c.MemoryStats.Limit),
	}
}

func (c *Cooked) BlockingIO() BlockingIO {
	bio := BlockingIO{}
	for _, svc := range c.BlkioStats.IoServicedRecursive {
		if len(svc.Op) == 0 {
			continue
		}
		switch svc.Op[0] {
		case 'r', 'R':
			bio.TotalReadCount += float64(svc.Value)
		case 'w', 'W':
			bio.TotalWriteCount += float64(svc.Value)
		}
	}
	for _, bytes := range c.BlkioStats.IoServiceBytesRecursive {
		if len(bytes.Op) == 0 {
			continue
		}
		switch bytes.Op[0] {
		case 'r', 'R':
			bio.TotalReadBytes += float64(bytes.Value)
		case 'w', 'W':
			bio.TotalWriteBytes += float64(bytes.Value)
		}
	}
	return bio
}

func calculateCPUPercentUnix(previous, current system.CPU) float64 {
	var (
		cpuPercent = 0.0
		// calculate the change for the cpu usage of the container in between readings
		cpuDelta = float64(current.TotalUsage - previous.TotalUsage)
		// calculate the change for the entire system between readings
		systemDelta = float64(current.SystemUsage - previous.SystemUsage)
		onlineCPUs  = float64(v.OnlineCPUs)
	)

	if onlineCPUs == 0.0 {
		onlineCPUs = float64(len(v.CPUUsage.PercpuUsage))
	}

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}
	return cpuPercent
}
