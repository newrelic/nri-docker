package docker

import "github.com/newrelic/infra-integrations-sdk/data/metric"

const ContainerSampleName = "DockerContainerSample"

func metricFunc(name string, sType metric.SourceType) func(interface{}) Metric {
	return func(value interface{}) Metric {
		return Metric{Name: name, Type: sType, Value: value}
	}
}

var (
	MetricCommandLine             = metricFunc("commandLine", metric.ATTRIBUTE)
	MetricContainerImage          = metricFunc("image", metric.ATTRIBUTE)
	MetricContainerImageName      = metricFunc("imageName", metric.ATTRIBUTE)
	MetricContainerName           = metricFunc("name", metric.ATTRIBUTE)
	MetricContainerID             = metricFunc("containerId", metric.ATTRIBUTE)
	MetricState                   = metricFunc("state", metric.ATTRIBUTE)
	MetricStatus                 = metricFunc("status", metric.ATTRIBUTE)
	MetricCPUPercent              = metricFunc("cpuPercent", metric.GAUGE)
	MetricCPUSystemPercent        = metricFunc("cpuSystemPercent", metric.GAUGE)
	MetricCPUUserPercent          = metricFunc("cpuUserPercent", metric.GAUGE)
	MetricMemoryVirtualSizeBytes  = metricFunc("memoryVirtualSizeBytes", metric.GAUGE)
	MetricMemoryResidentSizeBytes = metricFunc("memoryResidentSizeBytes", metric.GAUGE)
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
