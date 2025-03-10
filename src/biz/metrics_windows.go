package biz

import (
	"math"

	"github.com/docker/docker/api/types"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	"github.com/newrelic/nri-docker/src/raw"
	"github.com/newrelic/nri-docker/src/utils"
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

// TODO: https://new-relic.atlassian.net/browse/NR-375198
func (mc *MetricsFetcher) cpu(metrics raw.Metrics, _ *types.ContainerJSON) CPU {
	previous := StoredCPUSample{}
	// store current metrics to be the "previous" metrics in the next CPU sampling
	defer func() {
		previous.Time = metrics.Time.Unix()
		previous.CPU = metrics.CPU
		mc.store.Set(metrics.ContainerID, previous)
	}()

	cpu := CPU{}
	cpu.NumProcs = metrics.CPU.NumProcs

	// Reading previous CPU stats
	if _, err := mc.store.Get(metrics.ContainerID, &previous); err != nil {
		log.Debug("could not retrieve previous CPU stats for container %v: %v", metrics.ContainerID, err.Error())
		return cpu
	}

	// calculate the change for the cpu usage of the container in between readings
	durationIntervals := uint64(metrics.CPU.Read.Sub(metrics.CPU.PreRead).Nanoseconds()) / 100
	if durationIntervals <= 0 {
		log.Debug("duration intervals is 0, returning empty CPU metrics")
		return cpu
	}

	maxVal := float64(metrics.CPU.NumProcs * 100)

	cpu.CPUPercent = cpuPercent(previous.CPU, metrics.CPU)

	// If running in windows with Hyper-V isolation, can't check user and kernel CPU usage - set it to 0
	if metrics.CPU.UsageInUsermode == nil || previous.CPU.UsageInUsermode == nil || metrics.CPU.UsageInKernelmode == nil || previous.CPU.UsageInKernelmode == nil {
		log.Debug("could not retrieve user and kernel CPU usage for container %v, might be running in Hyper-V isolation", metrics.ContainerID)
		cpu.UserPercent = 0
		cpu.KernelPercent = 0
		return cpu
	}
	userDelta := float64(*metrics.CPU.UsageInUsermode - *previous.CPU.UsageInUsermode)
	cpu.UserPercent = math.Min(maxVal, (userDelta/float64(durationIntervals))*100)

	kernelDelta := float64(*metrics.CPU.UsageInKernelmode - *previous.CPU.UsageInKernelmode)
	cpu.KernelPercent = math.Min(maxVal, (kernelDelta/float64(durationIntervals))*100)

	return cpu
}

func cpuPercent(previous, current raw.CPU) float64 {
	// Max number of 100ns intervals between the previous time read and now
	passIntervals := uint64(current.Read.Sub(current.PreRead).Nanoseconds()) // Start with number of ns intervals
	passIntervals /= 100                                                     // Convert to number of 100ns intervals
	passIntervals *= uint64(current.NumProcs)                                // Multiple by the number of processors

	// Intervals used
	intervalsUsed := current.TotalUsage - previous.TotalUsage

	// Percentage avoiding divide-by-zero
	if passIntervals > 0 {
		log.Debug("passIntervals: %v, intervalsUsed: %v", passIntervals, intervalsUsed)
		return (float64(intervalsUsed) / float64(passIntervals)) * 100
	}
	return 0.0
}

func (mc *MetricsFetcher) blkIO(blkio raw.Blkio) BlkIO {
	bio := BlkIO{}
	bio.TotalReadBytes = utils.ToPointer(float64(blkio.ReadSizeBytes))
	bio.TotalWriteBytes = utils.ToPointer(float64(blkio.WriteSizeBytes))
	bio.TotalReadCount = utils.ToPointer(float64(blkio.ReadCountNormalized))
	bio.TotalWriteCount = utils.ToPointer(float64(blkio.WriteCountNormalized))
	return bio
}
