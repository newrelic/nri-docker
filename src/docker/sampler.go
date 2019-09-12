package docker

import (
	"context"
	"math"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-docker/src/stats"
)

const labelPrefix = "label."
const dockerClientVersion = "1.24" // todo: make configurable

type ContainerSampler struct {
	docker *client.Client
	stats  stats.Provider
}

func populate(ms *metric.Set, metrics []Metric) error {
	for _, metric := range metrics {
		if err := ms.SetMetric(metric.Name, metric.Value, metric.Type); err != nil {
			return err
		}
	}
	return nil
}

func attributes(container types.Container) []Metric {
	var cname string
	if len(container.Names) > 0 {
		cname = container.Names[0]
	}
	return []Metric{
		MetricCommandLine(container.Command),
		MetricContainerName(cname),
		MetricContainerImage(container.ImageID),
		MetricContainerImageName(container.Image),
		MetricState(container.State),
		MetricStatus(container.Status),
	}
}

func labels(container types.Container) []Metric {
	metrics := make([]Metric, 0, len(container.Labels))
	for key, val := range container.Labels {
		metrics = append(metrics, Metric{
			Name:  labelPrefix + key,
			Value: val,
			Type:  metric.ATTRIBUTE,
		})
	}
	return metrics
}

func (cs *ContainerSampler) statsMetrics(containerID string) []Metric {
	stats, err := cs.stats.Fetch(containerID)
	if err != nil {
		log.Error("error retrieving stats for container %s: %s", containerID, err.Error())
		return []Metric{}
	}

	cpu, mem, bio := stats.CPU(), stats.Memory(), stats.BlockingIO()
	memLimits := mem.MemLimitBytes
	// negative or ridiculously large memory limits are set to 0 (no limit)
	if memLimits < 0 || memLimits > float64(math.MaxInt64/2) {
		memLimits = 0
	}
	return []Metric{
		MetricPIDs(float64(stats.PidsStats.Current)),
		MetricCPUPercent(cpu.CPU),
		MetricCPUKernelPercent(cpu.Kernel),
		MetricCPUUserPercent(cpu.User),
		MetricMemoryCacheBytes(mem.CacheUsageBytes),
		MetricMemoryUsageBytes(mem.UsageBytes),
		MetricMemoryResidentSizeBytes(mem.RSSUsageBytes),
		MetricMemorySizeLimitBytes(memLimits),
		MetricIOTotalReadCount(bio.TotalReadCount),
		MetricIOTotalWriteCount(bio.TotalWriteCount),
		MetricIOTotalReadBytes(bio.TotalReadBytes),
		MetricIOTotalWriteBytes(bio.TotalWriteBytes),
		MetricIOReadCountPerSecond(bio.TotalReadCount),
		MetricIOWriteCountPerSecond(bio.TotalWriteCount),
		MetricIOReadBytesPerSecond(bio.TotalReadBytes),
		MetricIOWriteBytesPerSecond(bio.TotalWriteBytes),
	}
}

func NewContainerSampler(statsProvider stats.Provider) (ContainerSampler, error) {
	cli, err := client.NewEnvClient()
	cli.UpdateClientVersion(dockerClientVersion) // TODO: make it configurable
	return ContainerSampler{
		docker: cli,
		stats:  statsProvider,
	}, err
}

func (cs *ContainerSampler) SampleAll(entity *integration.Entity) error {
	// TODO: filter by state == running?
	containers, err := cs.docker.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return err
	}
	for _, container := range containers {
		ms := entity.NewMetricSet(ContainerSampleName,
			metric.Attr(AttrContainerID, container.ID)) // TODO: provide other unique label

		if err := populate(ms, attributes(container)); err != nil {
			return err
		}

		if err := populate(ms, cs.statsMetrics(container.ID)); err != nil {
			return err
		}

		if err := populate(ms, labels(container)) ; err != nil {
			return err
		}
	}
	return nil
}
