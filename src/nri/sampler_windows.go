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
