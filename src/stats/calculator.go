package stats

import (
	"math"

	"github.com/docker/docker/api/types"
)

// TODO: provide an alternative data structure that fits our requirements
// without so many white gaps
type Cooked types.Stats

type CPU struct {
	CPU             float64
	Kernel          float64
	User            float64
	UsedCores       float64
	ThrottlePeriods uint64
	ThrottledTimeMS float64
}

func calculateCPUPercentUnix(previousCPU, previousSystem uint64, v *types.CPUStats) float64 {
	var (
		cpuPercent = 0.0
		// calculate the change for the cpu usage of the container in between readings
		cpuDelta = float64(v.CPUUsage.TotalUsage) - float64(previousCPU)
		// calculate the change for the entire system between readings
		systemDelta = float64(v.SystemUsage) - float64(previousSystem)
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

// this formula is only valid for Linux. TODO: provide windows version
func (c *Cooked) CPU() CPU {
	cpu := CPU{}
	if c.PreRead.IsZero() {
		return cpu
	}

	// calculate the change for the cpu usage of the container in between readings
	duration := float64(c.Read.Sub(c.PreRead).Nanoseconds())
	if duration <= 0 {
		return cpu
	}

	maxVal := float64(len(c.CPUStats.CPUUsage.PercpuUsage) * 100)

	cpu.CPU = calculateCPUPercentUnix(c.PreCPUStats.CPUUsage.TotalUsage, c.PreCPUStats.SystemUsage, &c.CPUStats)

	userDelta := float64(c.CPUStats.CPUUsage.UsageInUsermode - c.PreCPUStats.CPUUsage.UsageInUsermode)
	cpu.User = math.Min(maxVal, userDelta*100/duration)

	kernelDelta := float64(c.CPUStats.CPUUsage.UsageInKernelmode - c.PreCPUStats.CPUUsage.UsageInKernelmode)
	cpu.Kernel = math.Min(maxVal, kernelDelta*100/duration)

	cpu.ThrottlePeriods = c.CPUStats.ThrottlingData.ThrottledPeriods
	cpu.ThrottledTimeMS = float64(c.CPUStats.ThrottlingData.ThrottledTime) / nanoSecondsPerSecond
	cpu.UsedCores = float64(c.CPUStats.CPUUsage.TotalUsage-c.PreCPUStats.CPUUsage.TotalUsage) / duration

	return cpu
}

type Memory struct {
	UsageBytes      float64
	CacheUsageBytes float64
	RSSUsageBytes   float64
	MemLimitBytes   float64
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

type BlockingIO struct {
	TotalReadCount  float64
	TotalWriteCount float64
	TotalReadBytes  float64
	TotalWriteBytes float64
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
