package stats

import (
	"math"

	"github.com/docker/docker/api/types"
)

// TODO: provide an alternative data structure that fits our requirements
// without so many white gaps
type Cooked types.Stats

type CPU struct {
	CPU    float64
	Kernel float64
	User   float64
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

	cpuDelta := float64(c.CPUStats.CPUUsage.TotalUsage - c.PreCPUStats.CPUUsage.TotalUsage)
	cpu.CPU = math.Min(maxVal, cpuDelta*100/duration)

	userDelta := float64(c.CPUStats.CPUUsage.UsageInUsermode - c.PreCPUStats.CPUUsage.UsageInUsermode)
	cpu.User = math.Min(maxVal, userDelta*100/duration)

	kernelDelta := float64(c.CPUStats.CPUUsage.UsageInKernelmode - c.PreCPUStats.CPUUsage.UsageInKernelmode)
	cpu.Kernel = math.Min(maxVal, kernelDelta*100/duration)

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
