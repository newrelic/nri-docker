package docker

import (
	"math/rand"

	"github.com/newrelic/infra-integrations-sdk/data/metric"
)

type Sampler interface {
	Populate(*metric.Set) error
}

type ContainerSampler struct {
}

func (c ContainerSampler) Populate(ms *metric.Set) error {
	rndCpu := 10 + rand.Float64()*10
	for _, metric := range []Metric{
		{"commandLine", metric.ATTRIBUTE, "/command-exec"},
		{"user", metric.ATTRIBUTE, "root"},
		{"containerImage", metric.ATTRIBUTE, "12345"},
		{"containerImageName", metric.ATTRIBUTE, "alpine:latest"},
		{"containerName", metric.ATTRIBUTE, "containername"},
		{"containerId", metric.ATTRIBUTE, "123456"},
		{"state", metric.ATTRIBUTE, "running"},
		{"label.docker.meta", metric.ATTRIBUTE, "label-value"},
		{"cpuPercent", metric.GAUGE, rndCpu},
		{"cpuSystemPercent", metric.GAUGE, rndCpu * 0.2},
		{"cpuUserPercent", metric.GAUGE, rndCpu * 0.8},
		{"memoryVirtualSizeBytes", metric.GAUGE, 10_000_000},
		{"memoryResidentSizeBytes", metric.GAUGE, 8_000_000},
		{"ioReadCountPerSecond", metric.GAUGE, 0}, // take from blkio_stats
		{"ioWriteCountPerSecond", metric.GAUGE, 0},
		{"ioReadBytesPerSecond", metric.GAUGE, 0},
		{"ioWriteBytesPerSecond", metric.GAUGE, 0},
		{"ioTotalReadCount", metric.GAUGE, 0},
		{"ioTotalWriteCount", metric.GAUGE, 0},
		{"ioTotalReadBytes", metric.GAUGE, 0},
		{"ioTotalWriteBytes", metric.GAUGE, 0},
		{"pidsNumber", metric.GAUGE, 1},
	} {
		if err := ms.SetMetric(metric.Name, metric.Value, metric.Type); err != nil {
			return err
		}
	}
	return nil
}

func NewContainerSampler() Sampler {
	return ContainerSampler{}
}
