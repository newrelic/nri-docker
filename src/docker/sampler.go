package docker

import (
	"context"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
)

type ContainerSampler struct {
	docker *client.Client
}

func populate(ms *metric.Set, metrics []Metric) error {
	for _, metric := range metrics {
		if err := ms.SetMetric(metric.Name, metric.Value, metric.Type); err != nil {
			return err
		}
	}
	return nil
}

func statMetrics(container types.Container) []Metric {
	var cname string
	if len(container.Names) > 0 {
		cname = container.Names[0]
	}
	return []Metric{
		MetricCommandLine(container.Command),
		MetricContainerID(container.ID),
		MetricContainerName(cname),
		MetricContainerImage(container.ImageID),
		MetricContainerImageName(container.Image),
		MetricState(container.State),
		MetricStatus(container.Status),
	}
}

/*

func populateCPUStat(container docker.CgroupDockerStat, ms *metric.Set) error {


	rndCpu := 10 + rand.Float64()*10
	for _, metric := range []Metric{
		MetricCommandLine( "/command-exec"),
		MetricUser("root"),
		MetricContainerImage("12345"),
		MetricContainerImageName("alpine:latest"),
		MetricContainerName("containername"),
		MetricContainerID("123456"),
		MetricState("running"),
		{"label.docker.meta", metric.ATTRIBUTE, "label-value"},
		MetricCPUPercent(rndCpu),
		MetricCPUSystemPercent(rndCpu * 0.2),
		MetricCPUUserPercent(rndCpu * 0.8),
		MetricMemoryVirtualSizeBytes(10000000),
		MetricMemoryResidentSizeBytes(8000000),
		MetricIOReadCountPerSecond(0), // take from blkio_stats
		MetricIOWriteCountPerSecond(0),
		MetricIOReadBytesPerSecond(0),
		MetricIOWriteBytesPerSecond(0),
		MetricIOTotalReadCount(0),
		MetricIOTotalWriteCount(0),
		MetricIOTotalReadBytes(0),
		MetricIOTotalWriteBytes(0),
		MetricPIDs(1),
	} {
		if err := ms.SetMetric(metric.Name, metric.Value, metric.Type); err != nil {
			return err
		}
	}
	return nil
}
*/
func NewContainerSampler() (ContainerSampler, error) {
	cli, err := client.NewEnvClient()
	cli.UpdateClientVersion("1.24") // TODO: make it configurable
	return ContainerSampler{docker: cli}, err
}

func (cs *ContainerSampler) SampleAll(entity *integration.Entity) error {
	// TODO: filter by state == running?
	containers, err := cs.docker.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return err
	}
	for _, container := range containers {
		ms := entity.NewMetricSet(ContainerSampleName)

		if err := populate(ms, statMetrics(container)); err != nil {
			return err
		}
	}
	return nil
}
