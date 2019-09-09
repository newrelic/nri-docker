package stats

import "github.com/docker/docker/api/types"

type Cooked types.Stats

func Cook(t types.Stats) Cooked {
	return Cooked(t)
}

func (c *Cooked) CPU() (cpu, system, user float64) {
	duration := float64(c.Read.Sub(c.PreRead).Nanoseconds())

	// TODO: caution! windows docker returns the number in units of 100 ns while linux in units of ns
	cpu = 100 * float64(c.CPUStats.CPUUsage.TotalUsage) / duration
	system = 100 * float64(c.CPUStats.CPUUsage.UsageInKernelmode) / duration
	user = 100 * float64(c.CPUStats.CPUUsage.UsageInUsermode) / duration

	return
}
