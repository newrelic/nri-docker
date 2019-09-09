package docker

import "github.com/newrelic/infra-integrations-sdk/data/metric"

const ContainerSampleName = "DockerContainerSample"

type Metric struct {
	Name  string
	Type  metric.SourceType
	Value interface{}
}

