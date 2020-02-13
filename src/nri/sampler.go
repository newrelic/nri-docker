package nri

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/persist"
	"github.com/newrelic/nri-docker/src/biz"
	"github.com/newrelic/nri-docker/src/raw"
)

const (
	labelPrefix         = "label."
	dockerClientVersion = "1.24" // todo: make configurable
	containerSampleName = "ContainerSample"
	attrContainerID     = "containerId"
)

// ContainerSampler invokes the metrics sampling and processing for all the existing containers, and populates the
// integration execution information with the exported metrics
type ContainerSampler struct {
	metrics biz.Processer
	store   persist.Storer
	docker  dockerClient
}

// abstraction of the docker client to facilitate testing
type dockerClient interface {
	biz.Inspector
	ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error)
}

// NewSampler returns a ContainerSampler instance. The hostRoot argument is used only if the integration is
// executed inside a container, and must point to the shared folder that allows accessing to the host root
// folder (usually /host)
func NewSampler(hostRoot, cgroupPath string) (ContainerSampler, error) {
	// instantiating internal components
	// docker client to list and inspect containers
	docker, err := client.NewEnvClient()
	if err != nil {
		return ContainerSampler{}, err
	}
	defer docker.Close()
	docker.UpdateClientVersion(dockerClientVersion)

	// SDK Storer to keep metric values between executions (e.g. for rates and deltas)
	store, err := persist.NewFileStore( // TODO: make the following options configurable
		persist.DefaultPath("container_cpus"),
		log.NewStdErr(true),
		60*time.Second)
	if err != nil {
		return ContainerSampler{}, err
	}

	// Raw metrics fetcher to get the raw metrics from the system (cgroups and proc fs)
	rawFetcher := raw.NewFetcher(hostRoot, cgroupPath, raw.GetMountsFilePath())

	return ContainerSampler{
		metrics: biz.NewProcessor(store, rawFetcher, docker),
		docker:  docker,
		store:   store,
	}, nil
}

// SampleAll populates the integration of the argument with metrics and labels from all the containers in the system
// running and non-running
func (cs *ContainerSampler) SampleAll(i *integration.Integration) error {
	// todo: configure to retrieve only the running containers
	containers, err := cs.docker.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	if err != nil {
		return err
	}
	defer func() {
		if err := cs.store.Save(); err != nil {
			log.Warn("persisting previous metrics: %s", err.Error())
		}
	}()
	for _, container := range containers {
		// Creating entity and populating metrics
		entity, err := i.Entity(container.ID, "docker")
		if err != nil {
			return err
		}
		ms := entity.NewMetricSet(containerSampleName,
			metric.Attr(attrContainerID, container.ID))

		// populating metrics that are common to running and stopped containers
		populate(ms, attributes(container))
		populate(ms, labels(container))

		if container.State != "running" {
			continue
		}

		// populating metrics that only apply to running containers
		metrics, err := cs.metrics.Process(container.ID)
		if err != nil {
			log.Error("error fetching metrics for container %v: %v", container.ID, err)
			continue
		}
		populate(ms, misc(&metrics))
		populate(ms, cpu(&metrics.CPU))
		populate(ms, memory(&metrics.Memory))
		populate(ms, pids(&metrics.Pids))
		populate(ms, blkio(&metrics.BlkIO))
		populate(ms, cs.networkMetrics(&metrics.Network))
	}
	return nil
}

func populate(ms *metric.Set, metrics []entry) {
	for _, metric := range metrics {
		if err := ms.SetMetric(metric.Name, metric.Value, metric.Type); err != nil {
			log.Warn("Unexpected error setting metric %#v: %v", metric, err)
		}
	}
}

func attributes(container types.Container) []entry {
	var cname string
	if len(container.Names) > 0 {
		cname = container.Names[0]
		if len(cname) > 0 && cname[0] == '/' {
			cname = cname[1:]
		}
	}
	return []entry{
		metricCommandLine(container.Command),
		metricContainerName(cname),
		metricContainerImage(container.ImageID),
		metricContainerImageName(container.Image),
		metricState(container.State),
		metricStatus(container.Status),
	}
}

func labels(container types.Container) []entry {
	metrics := make([]entry, 0, len(container.Labels))
	for key, val := range container.Labels {
		metrics = append(metrics, entry{
			Name:  labelPrefix + key,
			Value: val,
			Type:  metric.ATTRIBUTE,
		})
	}
	return metrics
}

func memory(mem *biz.Memory) []entry {
	metrics := []entry{
		metricMemoryCacheBytes(mem.CacheUsageBytes),
		metricMemoryUsageBytes(mem.UsageBytes),
		metricMemoryResidentSizeBytes(mem.RSSUsageBytes),
	}
	if mem.MemLimitBytes > 0 {
		metrics = append(metrics,
			metricMemorySizeLimitBytes(mem.MemLimitBytes),
			metricMemoryUsageLimitPercent(mem.UsagePercent),
		)
	}
	return metrics
}

func pids(pids *biz.Pids) []entry {
	return []entry{
		metricProcessCount(pids.Current),
		metricProcessCountLimit(pids.Limit),
	}
}

func blkio(bio *biz.BlkIO) []entry {
	return []entry{
		metricIOTotalReadCount(bio.TotalReadCount),
		metricIOTotalWriteCount(bio.TotalWriteCount),
		metricIOTotalReadBytes(bio.TotalReadBytes),
		metricIOTotalWriteBytes(bio.TotalWriteBytes),
		metricIOTotalBytes(bio.TotalReadBytes + bio.TotalWriteBytes),
		metricIOReadCountPerSecond(bio.TotalReadCount),
		metricIOWriteCountPerSecond(bio.TotalWriteCount),
		metricIOReadBytesPerSecond(bio.TotalReadBytes),
		metricIOWriteBytesPerSecond(bio.TotalWriteBytes),
	}
}

func (cs *ContainerSampler) networkMetrics(net *biz.Network) []entry {
	return []entry{
		metricRxBytes(net.RxBytes),
		metricRxErrors(net.RxErrors),
		metricRxDropped(net.RxDropped),
		metricRxPackets(net.RxPackets),
		metricTxBytes(net.TxBytes),
		metricTxErrors(net.TxErrors),
		metricTxDropped(net.TxDropped),
		metricTxPackets(net.TxPackets),
		metricRxBytesPerSecond(net.RxBytes),
		metricRxErrorsPerSecond(net.RxErrors),
		metricRxDroppedPerSecond(net.RxDropped),
		metricRxPacketsPerSecond(net.RxPackets),
		metricTxBytesPerSecond(net.TxBytes),
		metricTxErrorsPerSecond(net.TxErrors),
		metricTxDroppedPerSecond(net.TxDropped),
		metricTxPacketsPerSecond(net.TxPackets),
	}
}

func cpu(cpu *biz.CPU) []entry {
	return []entry{
		metricCPUUsedCores(cpu.UsedCores),
		metricCPUUsedCoresPercent(cpu.UsedCoresPercent),
		metricCPULimitCores(cpu.LimitCores),
		metricCPUPercent(cpu.CPUPercent),
		metricCPUKernelPercent(cpu.KernelPercent),
		metricCPUUserPercent(cpu.UserPercent),
		metricCPUThrottlePeriods(cpu.ThrottlePeriods),
		metricCPUThrottleTimeMS(cpu.ThrottledTimeMS),
	}
}

func misc(m *biz.Sample) []entry {
	return []entry{
		metricRestartCount(m.RestartCount),
	}
}
