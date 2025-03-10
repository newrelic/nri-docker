package nri

import "github.com/newrelic/nri-docker/src/biz"

func memory(mem *biz.Memory) []entry {
	metrics := []entry{
		metricMemoryCommitBytes(mem.CommitBytes),
		metricMemoryCommitPeakBytes(mem.CommitPeakBytes),
		metricMemoryPrivateWorkingSet(mem.PrivateWorkingSet),
		metricMemoryUsageLimitPercent(mem.UsagePercent),
	}
	return metrics
}

func cpu(cpu *biz.CPU) []entry {
	return []entry{
		metricCPUPercent(cpu.CPUPercent),
		metricCPUKernelPercent(cpu.KernelPercent),
		metricCPUUserPercent(cpu.UserPercent),
		metricCPUProcs(cpu.NumProcs),
	}
}

func blkio(bio *biz.BlkIO) []entry {
	var entries []entry
	totalBytes := 0.0

	if bio.TotalReadCount != nil {
		entries = append(entries, metricIOReadCountNormalized(*bio.TotalReadCount))
	}
	if bio.TotalWriteCount != nil {
		entries = append(entries, metricIOWriteCountNormalized(*bio.TotalWriteCount))
	}
	if bio.TotalReadBytes != nil {
		entries = append(entries, metricIOTotalReadBytes(*bio.TotalReadBytes))
		totalBytes += *bio.TotalReadBytes
	}
	if bio.TotalWriteBytes != nil {
		entries = append(entries, metricIOTotalWriteBytes(*bio.TotalWriteBytes))
		totalBytes += *bio.TotalWriteBytes
	}

	if bio.TotalReadBytes != nil || bio.TotalWriteBytes != nil {
		entries = append(entries, metricIOTotalBytes(totalBytes))
	}

	return entries
}
