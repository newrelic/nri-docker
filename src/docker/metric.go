package docker

import "github.com/newrelic/infra-integrations-sdk/data/metric"

const ContainerSampleName = "DockerContainerSample"

func metricFunc(name string, sType metric.SourceType) func(interface{}) Metric {
	return func(value interface{}) Metric {
		return Metric{Name: name, Type: sType, Value: value}
	}
}

/*
Metric renames over the original dummy sample:
cpuSystemPercent -> cpuKernelPercent
memoryVirtualSizeBytes -> memoryUsageBytes

Nuevas:
-> memorySizeLimitBytes
*/
var (
	MetricCommandLine             = metricFunc("commandLine", metric.ATTRIBUTE)
	MetricContainerImage          = metricFunc("image", metric.ATTRIBUTE)
	MetricContainerImageName      = metricFunc("imageName", metric.ATTRIBUTE)
	MetricContainerName           = metricFunc("name", metric.ATTRIBUTE)
	MetricContainerID             = metricFunc("containerId", metric.ATTRIBUTE)
	MetricState                   = metricFunc("state", metric.ATTRIBUTE)
	MetricStatus                  = metricFunc("status", metric.ATTRIBUTE)
	MetricCPUPercent              = metricFunc("cpuPercent", metric.GAUGE)
	MetricCPUKernelPercent        = metricFunc("cpuKernelPercent", metric.GAUGE)
	MetricCPUUserPercent          = metricFunc("cpuUserPercent", metric.GAUGE)
	MetricMemoryUsageBytes        = metricFunc("memoryUsageBytes", metric.GAUGE)
	MetricMemoryCacheBytes        = metricFunc("memoryCacheBytes", metric.GAUGE)
	MetricMemoryResidentSizeBytes = metricFunc("memoryResidentSizeBytes", metric.GAUGE)
	MetricMemorySizeLimitBytes    = metricFunc("memorySizeLimitBytes", metric.GAUGE)
	MetricIOReadCountPerSecond    = metricFunc("ioReadCountPerSecond", metric.GAUGE)
	MetricIOWriteCountPerSecond   = metricFunc("ioWriteCountPerSecond", metric.GAUGE)
	MetricIOReadBytesPerSecond    = metricFunc("ioReadBytesPerSecond", metric.GAUGE)
	MetricIOWriteBytesPerSecond   = metricFunc("ioWriteBytesPerSecond", metric.GAUGE)
	MetricIOTotalReadCount        = metricFunc("ioTotalReadCount", metric.GAUGE)
	MetricIOTotalWriteCount       = metricFunc("ioTotalWriteCount", metric.GAUGE)
	MetricIOTotalReadBytes        = metricFunc("ioTotalReadBytes", metric.GAUGE)
	MetricIOTotalWriteBytes       = metricFunc("ioTotalWriteBytes", metric.GAUGE)
	MetricPIDs                    = metricFunc("pidsNumber", metric.GAUGE)
)

type Metric struct {
	Name  string
	Type  metric.SourceType
	Value interface{}
}
