// Package nri uses Docker API information and sampled containers and presents it in a format that is accepted
// by the New Relic Infrastructure Agent
package nri

import "github.com/newrelic/infra-integrations-sdk/v3/data/metric"

// nolint: unused
var (
	metricCommandLine                 = metricFunc("commandLine", metric.ATTRIBUTE)
	metricContainerImage              = metricFunc("image", metric.ATTRIBUTE)
	metricContainerImageName          = metricFunc("imageName", metric.ATTRIBUTE)
	metricContainerName               = metricFunc("name", metric.ATTRIBUTE)
	metricState                       = metricFunc("state", metric.ATTRIBUTE)
	metricStatus                      = metricFunc("status", metric.ATTRIBUTE)
	metricRestartCount                = metricFunc("restartCount", metric.GAUGE)
	metricCPUUsedCores                = metricFunc("cpuUsedCores", metric.GAUGE)
	metricCPUUsedCoresPercent         = metricFunc("cpuUsedCoresPercent", metric.GAUGE)
	metricCPULimitCores               = metricFunc("cpuLimitCores", metric.GAUGE)
	metricCPUPercent                  = metricFunc("cpuPercent", metric.GAUGE)
	metricCPUKernelPercent            = metricFunc("cpuKernelPercent", metric.GAUGE)
	metricCPUUserPercent              = metricFunc("cpuUserPercent", metric.GAUGE)
	metricCPUThrottleTimeMS           = metricFunc("cpuThrottleTimeMs", metric.GAUGE)
	metricCPUThrottlePeriods          = metricFunc("cpuThrottlePeriods", metric.GAUGE)
	metricCPUShares                   = metricFunc("cpuShares", metric.GAUGE)
	metricCPUProcs                    = metricFunc("cpuProcs", metric.GAUGE)
	metricMemoryUsageBytes            = metricFunc("memoryUsageBytes", metric.GAUGE)
	metricMemoryCacheBytes            = metricFunc("memoryCacheBytes", metric.GAUGE)
	metricMemoryResidentSizeBytes     = metricFunc("memoryResidentSizeBytes", metric.GAUGE)
	metricMemorySizeLimitBytes        = metricFunc("memorySizeLimitBytes", metric.GAUGE)
	metricMemoryUsageLimitPercent     = metricFunc("memoryUsageLimitPercent", metric.GAUGE)
	metricMemoryKernelUsageBytes      = metricFunc("memoryKernelUsageBytes", metric.GAUGE)
	metricMemorySwapUsageBytes        = metricFunc("memorySwapUsageBytes", metric.GAUGE)
	metricMemorySwapOnlyUsageBytes    = metricFunc("memorySwapOnlyUsageBytes", metric.GAUGE)
	metricMemorySwapLimitBytes        = metricFunc("memorySwapLimitBytes", metric.GAUGE)
	metricMemorySwapLimitUsagePercent = metricFunc("memorySwapLimitUsagePercent", metric.GAUGE)
	metricMemoryCommitBytes           = metricFunc("memoryCommitBytes", metric.GAUGE)
	metricMemoryCommitPeakBytes       = metricFunc("memoryCommitPeakBytes", metric.GAUGE)
	metricMemoryPrivateWorkingSet     = metricFunc("memoryPrivateWorkingSet", metric.GAUGE)
	metricMemorySoftLimitBytes        = metricFunc("memorySoftLimitBytes", metric.GAUGE)
	metricIOReadCountPerSecond        = metricFunc("ioReadCountPerSecond", metric.PRATE)
	metricIOWriteCountPerSecond       = metricFunc("ioWriteCountPerSecond", metric.PRATE)
	metricIOReadBytesPerSecond        = metricFunc("ioReadBytesPerSecond", metric.PRATE)
	metricIOWriteBytesPerSecond       = metricFunc("ioWriteBytesPerSecond", metric.PRATE)
	metricIOTotalReadCount            = metricFunc("ioTotalReadCount", metric.GAUGE)
	metricIOTotalWriteCount           = metricFunc("ioTotalWriteCount", metric.GAUGE)
	metricIOTotalReadBytes            = metricFunc("ioTotalReadBytes", metric.GAUGE)
	metricIOTotalWriteBytes           = metricFunc("ioTotalWriteBytes", metric.GAUGE)
	metricIOReadCountNormalized       = metricFunc("ioReadCountNormalized", metric.GAUGE)
	metricIOWriteCountNormalized      = metricFunc("ioWriteCountNormalized", metric.GAUGE)
	metricIOTotalBytes                = metricFunc("ioTotalBytes", metric.GAUGE)
	metricThreadCount                 = metricFunc("threadCount", metric.GAUGE)
	metricThreadCountLimit            = metricFunc("threadCountLimit", metric.GAUGE)
	metricRxBytes                     = metricFunc("networkRxBytes", metric.GAUGE)
	metricRxDropped                   = metricFunc("networkRxDropped", metric.GAUGE)
	metricRxErrors                    = metricFunc("networkRxErrors", metric.GAUGE)
	metricRxPackets                   = metricFunc("networkRxPackets", metric.GAUGE)
	metricTxBytes                     = metricFunc("networkTxBytes", metric.GAUGE)
	metricTxDropped                   = metricFunc("networkTxDropped", metric.GAUGE)
	metricTxErrors                    = metricFunc("networkTxErrors", metric.GAUGE)
	metricTxPackets                   = metricFunc("networkTxPackets", metric.GAUGE)
	metricRxBytesPerSecond            = metricFunc("networkRxBytesPerSecond", metric.PRATE)
	metricRxDroppedPerSecond          = metricFunc("networkRxDroppedPerSecond", metric.PRATE)
	metricRxErrorsPerSecond           = metricFunc("networkRxErrorsPerSecond", metric.PRATE)
	metricRxPacketsPerSecond          = metricFunc("networkRxPacketsPerSecond", metric.PRATE)
	metricTxBytesPerSecond            = metricFunc("networkTxBytesPerSecond", metric.PRATE)
	metricTxDroppedPerSecond          = metricFunc("networkTxDroppedPerSecond", metric.PRATE)
	metricTxErrorsPerSecond           = metricFunc("networkTxErrorsPerSecond", metric.PRATE)
	metricTxPacketsPerSecond          = metricFunc("networkTxPacketsPerSecond", metric.PRATE)
	metricStorageDataUsed             = metricFunc("storageDataUsedBytes", metric.GAUGE)
	metricStorageDataAvailable        = metricFunc("storageDataAvailableBytes", metric.GAUGE)
	metricStorageDataTotal            = metricFunc("storageDataTotalBytes", metric.GAUGE)
	metricStorageDataUsagePercent     = metricFunc("storageDataUsagePercent", metric.GAUGE)
	metricStorageMetadataUsed         = metricFunc("storageMetadataUsedBytes", metric.GAUGE)
	metricStorageMetadataAvailable    = metricFunc("storageMetadataAvailableBytes", metric.GAUGE)
	metricStorageMetadataTotal        = metricFunc("storageMetadataTotalBytes", metric.GAUGE)
	metricStorageMetadataUsagePercent = metricFunc("storageMetadataUsagePercent", metric.GAUGE)
)

type entry struct {
	Name  string
	Type  metric.SourceType
	Value interface{}
}

func metricFunc(name string, sType metric.SourceType) func(interface{}) entry {
	return func(value interface{}) entry {
		return entry{Name: name, Type: sType, Value: value}
	}
}
