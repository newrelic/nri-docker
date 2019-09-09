package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
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

func (cs *ContainerSampler) metrics(containerID string) []Metric {
	// TODO: consider streaming in a long-lived integration
	apiStats, err := cs.docker.ContainerStats(context.Background(), containerID, false)
	if err != nil {
		log.Error("error retrieving container %s stats: %s", containerID, err.Error())
		return []Metric{}
	}

	jsonStats, err := ioutil.ReadAll(apiStats.Body)
	if err != nil {
		log.Error("wrong JSON for container %s stats: %s", containerID, err.Error())
		return []Metric{}
	}
	_ = apiStats.Body.Close()

	stats := types.Stats{}

	err = json.Unmarshal(jsonStats, &stats)
	if err != nil {
		log.Error("wrong JSON for container %s stats: %s", containerID, err.Error())
		return []Metric{}
	}

	fmt.Println(string(jsonStats))
	return []Metric{}

}

/*

func populateCPUStat(container docker.CgroupDockerStat, ms *metric.Set) error {


	rndCpu := 10 + rand.Float64()*10
	for _, metric := range []Metric{
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

		cs.metrics(container.ID)
	}
	return nil
}
