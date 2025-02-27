package biz

import (
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	"github.com/newrelic/nri-docker/src/raw"
	gops_mem "github.com/shirou/gopsutil/mem"
)

func (mc *MetricsFetcher) memory(mem raw.Memory) Memory {
	m := Memory{
		CommitBytes:       mem.Commit,
		CommitPeakBytes:   mem.CommitPeak,
		PrivateWorkingSet: mem.PrivateWorkingSet,
	}

	var totalMemory uint64

	vmem, err := gops_mem.VirtualMemory()

	if err != nil {
		log.Warn("error getting total memory on system: %v", err)
		log.Warn("don't have total system memory, setting memory usage percent to 0")
		m.UsagePercent = 0
		return m
	}

	totalMemory = vmem.Total
	log.Debug("total memory on system: %v", totalMemory)

	if totalMemory == 0 {
		log.Warn("total system memory is reported as 0, setting memory usage percent to 0")
		m.UsagePercent = 0
		return m
	}

	// privateWorkingSet is the amount of memory that is not shared with other processes
	// and is not paged out to disk. It is the amount of memory that is unique to a process.
	// We use total memory on the system to calculate the memory usage percent because there
	// are no other metrics available to calculate the memory usage percent.
	memoryUsagePercent := float64(m.PrivateWorkingSet) / float64(totalMemory) * 100
	log.Debug("memory usage percent: %v", memoryUsagePercent)
	m.UsagePercent = memoryUsagePercent

	return m
}
