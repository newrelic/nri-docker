package docker

import "github.com/newrelic/infra-integrations-sdk/data/metric"

const ContainerSampleName = "ContainerSample"

func metricFunc(name string, sType metric.SourceType) func(interface{}) Metric {
	return func(value interface{}) Metric {
		return Metric{Name: name, Type: sType, Value: value}
	}
}

const AttrContainerID = "containerId"

var (
	MetricCommandLine             = metricFunc("commandLine", metric.ATTRIBUTE)
	MetricContainerImage          = metricFunc("image", metric.ATTRIBUTE)
	MetricContainerImageName      = metricFunc("imageName", metric.ATTRIBUTE)
	MetricContainerName           = metricFunc("name", metric.ATTRIBUTE)
	MetricState                   = metricFunc("state", metric.ATTRIBUTE)
	MetricStatus                  = metricFunc("status", metric.ATTRIBUTE)
	MetricNetworkIface            = metricFunc("networkIface", metric.ATTRIBUTE)
	MetricRestartCount			  = metricFunc("restartCount", metric.GAUGE)
	MetricCPUPercent              = metricFunc("cpuPercent", metric.GAUGE)
	MetricCPUKernelPercent        = metricFunc("cpuKernelPercent", metric.GAUGE)
	MetricCPUUserPercent          = metricFunc("cpuUserPercent", metric.GAUGE)
	MetricCPUThrottleTimeMS       = metricFunc("cpuThrottleTimeMs", metric.GAUGE)
	MetricCPUThrottlePeriods      = metricFunc("cpuThrottlePeriods", metric.GAUGE)
	MetricMemoryUsageBytes        = metricFunc("memoryUsageBytes", metric.GAUGE)
	MetricMemoryCacheBytes        = metricFunc("memoryCacheBytes", metric.GAUGE)
	MetricMemoryResidentSizeBytes = metricFunc("memoryResidentSizeBytes", metric.GAUGE)
	MetricMemorySizeLimitBytes    = metricFunc("memorySizeLimitBytes", metric.GAUGE)
	MetricIOReadCountPerSecond    = metricFunc("ioReadCountPerSecond", metric.RATE)
	MetricIOWriteCountPerSecond   = metricFunc("ioWriteCountPerSecond", metric.RATE)
	MetricIOReadBytesPerSecond    = metricFunc("ioReadBytesPerSecond", metric.RATE)
	MetricIOWriteBytesPerSecond   = metricFunc("ioWriteBytesPerSecond", metric.RATE)
	MetricIOTotalReadCount        = metricFunc("ioTotalReadCount", metric.GAUGE)
	MetricIOTotalWriteCount       = metricFunc("ioTotalWriteCount", metric.GAUGE)
	MetricIOTotalReadBytes        = metricFunc("ioTotalReadBytes", metric.GAUGE)
	MetricIOTotalWriteBytes       = metricFunc("ioTotalWriteBytes", metric.GAUGE)
	MetricIOTotalBytes            = metricFunc("ioTotalBytes", metric.GAUGE)
	MetricProcessCount            = metricFunc("processCount", metric.GAUGE)
	MetricProcessCountLimit       = metricFunc("processCountLimit", metric.GAUGE)
	MetricRxBytes                 = metricFunc("networkRxBytes", metric.GAUGE)
	MetricRxDropped               = metricFunc("networkRxDropped", metric.GAUGE)
	MetricRxErrors                = metricFunc("networkRxErrors", metric.GAUGE)
	MetricRxPackets               = metricFunc("networkRxPackets", metric.GAUGE)
	MetricTxBytes                 = metricFunc("networkTxBytes", metric.GAUGE)
	MetricTxDropped               = metricFunc("networkTxDropped", metric.GAUGE)
	MetricTxErrors                = metricFunc("networkTxErrors", metric.GAUGE)
	MetricTxPackets               = metricFunc("networkTxPackets", metric.GAUGE)
	MetricRxBytesPerSecond        = metricFunc("networkRxBytesPerSecond", metric.RATE)
	MetricRxDroppedPerSecond      = metricFunc("networkRxDroppedPerSecond", metric.RATE)
	MetricRxErrorsPerSecond       = metricFunc("networkRxErrorsPerSecond", metric.RATE)
	MetricRxPacketsPerSecond      = metricFunc("networkRxPacketsPerSecond", metric.RATE)
	MetricTxBytesPerSecond        = metricFunc("networkTxBytesPerSecond", metric.RATE)
	MetricTxDroppedPerSecond      = metricFunc("networkTxDroppedPerSecond", metric.RATE)
	MetricTxErrorsPerSecond       = metricFunc("networkTxErrorsPerSecond", metric.RATE)
	MetricTxPacketsPerSecond      = metricFunc("networkTxPacketsPerSecond", metric.RATE)
)

type Metric struct {
	Name  string
	Type  metric.SourceType
	Value interface{}
}
