package stats

import (
	"math"

	"github.com/docker/docker/api/types"
)

type Cooked types.Stats

func Cook(t types.Stats) Cooked {
	return Cooked(t)
}

// this formula is only valid for Linux. TODO: provide windows version
func (c *Cooked) CPU() (cpu, kernel, user float64) {
	// calculate the change for the cpu usage of the container in between readings
	duration := float64(c.Read.Sub(c.PreRead).Nanoseconds())
	if duration > 0 {
		maxVal := float64(len(c.CPUStats.CPUUsage.PercpuUsage) * 100)

		cpuDelta := float64(c.CPUStats.CPUUsage.TotalUsage - c.PreCPUStats.CPUUsage.TotalUsage)
		cpu = math.Min(maxVal, cpuDelta*100/duration)

		userDelta := float64(c.CPUStats.CPUUsage.UsageInUsermode - c.PreCPUStats.CPUUsage.UsageInUsermode)
		user = math.Min(maxVal, userDelta*100/duration)

		kernelDelta := float64(c.CPUStats.CPUUsage.UsageInKernelmode - c.PreCPUStats.CPUUsage.UsageInKernelmode)
		kernel = math.Min(maxVal, kernelDelta*100/duration)
	}
	return
}
