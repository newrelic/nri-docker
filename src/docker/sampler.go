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
	docker  *client.Client
	stats   stats.Provider
	network stats.NetworkFetcher
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

func (cs *ContainerSampler) networkMetrics(containerPid int) []Metric {
	net, err := cs.network.Fetch(containerPid)
	if err != nil {
		log.Debug("error retrieving network metrics: %s", err.Error())
		return []Metric{}
	}
	return []Metric{
		MetricRxBytes(net.RxBytes),
		MetricRxErrors(net.RxErrors),
		MetricRxDropped(net.RxDropped),
		MetricRxPackets(net.RxPackets),
		MetricTxBytes(net.TxBytes),
		MetricTxErrors(net.TxErrors),
		MetricTxDropped(net.TxDropped),
		MetricTxPackets(net.TxPackets),
		MetricRxBytesPerSecond(net.RxBytes),
		MetricRxErrorsPerSecond(net.RxErrors),
		MetricRxDroppedPerSecond(net.RxDropped),
		MetricRxPacketsPerSecond(net.RxPackets),
		MetricTxBytesPerSecond(net.TxBytes),
		MetricTxErrorsPerSecond(net.TxErrors),
		MetricTxDroppedPerSecond(net.TxDropped),
		MetricTxPacketsPerSecond(net.TxPackets),
	}
}

func NewContainerSampler(statsProvider stats.Provider) (ContainerSampler, error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		return ContainerSampler{}, err
	}
	cli.UpdateClientVersion(dockerClientVersion) // TODO: make it configurable
	net, err := stats.NewNetworkFetcher()
	if err != nil {
		return ContainerSampler{}, err
	}
	return ContainerSampler{
		docker:  cli,
		stats:   statsProvider,
		network: net,
	}, nil
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
			log.Debug("error populating container %v attributes: %s", container.ID, err)
			continue
		}

		if err := populate(ms, cs.statsMetrics(container.ID)); err != nil {
			log.Debug("error populating container %v stats metrics: %s", container.ID, err)
			continue
		}

		cjson, err := cs.docker.ContainerInspect(context.Background(), container.ID)
		if err != nil {
			log.Debug("error inspecting container %v: %s", container.ID, err)
			continue
		}
		if cjson.State == nil {
			log.Debug("error: container %v has no state: %s", container.ID, err)
			continue
		}
		if err := populate(ms, cs.networkMetrics(cjson.State.Pid)); err != nil {
			log.Debug("error populating container %v network metrics: %s", container.ID, err)
			continue
		}

		if err := populate(ms, labels(container)); err != nil {
			log.Debug("error populating container %v labels: %s", container.ID, err)
			continue
		}

	}
	return nil
}
