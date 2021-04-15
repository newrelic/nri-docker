package nri

import (
	"context"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/persist"
	"github.com/newrelic/nri-docker/src/biz"
	"github.com/newrelic/nri-docker/src/raw"
)

const (
	labelPrefix            = "label."
	containerSampleName    = "ContainerSample"
	attrContainerID        = "containerId"
	attrShortContainerID   = "shortContainerId"
	shortContainerIDLength = 12
)

// ContainerSampler invokes the metrics sampling and processing for all the existing containers, and populates the
// integration execution information with the exported metrics
type ContainerSampler struct {
	metrics biz.Processer
	store   persist.Storer
	docker  DockerClient
}

// DockerClient is an abstraction of the Docker client.
type DockerClient interface {
	biz.Inspector
	ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error)
}

// NewSampler returns a ContainerSampler instance.
func NewSampler(fetcher raw.Fetcher, docker DockerClient, exitedContainerTTL time.Duration) (*ContainerSampler, error) {
	// SDK Storer to keep metric values between executions (e.g. for rates and deltas)
	store, err := persist.NewFileStore( // TODO: make the following options configurable
		persist.DefaultPath("container_cpus"),
		log.NewStdErr(true),
		60*time.Second)
	if err != nil {
		return nil, err
	}

	return &ContainerSampler{
		metrics: biz.NewProcessor(store, fetcher, docker, exitedContainerTTL),
		docker:  docker,
		store:   store,
	}, nil
}

// SampleAll populates the integration of the argument with metrics and labels from all the containers in the system
// running and non-running
func (cs *ContainerSampler) SampleAll(ctx context.Context, i *integration.Integration) error {
	defer func() {
		if err := cs.store.Save(); err != nil {
			log.Warn("persisting previous metrics: %s", err.Error())
		}
	}()

	// todo: configure to retrieve only the running containers
	containers, err := cs.docker.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return err
	}

	//TODO Add ff to get this info
	info, err := cs.docker.Info(context.Background())
	if err != nil {
		log.Error("fetching info from docker api: %s", err.Error())
	}
	storageStats, err := biz.ParseDeviceMapperStats(info)
	if err != nil {
		log.Warn("computing Storage Driver stats: %s", err.Error())
	}
	storageEntry := getStorageEntry(storageStats)

	for _, container := range containers {
		metrics, err := cs.metrics.Process(container.ID)
		if err != nil {
			if _, ok := err.(*biz.ErrExitedContainerExpired); ok {
				log.Debug(err.Error())
				continue
			}
			log.Error("error fetching metrics for container %v: %v", container.ID, err)
		}

		// Creating entity and populating metrics
		entity, err := i.Entity(container.ID, "docker")
		if err != nil {
			return err
		}

		var shortContainerID string
		if len(container.ID) > shortContainerIDLength {
			shortContainerID = container.ID[:shortContainerIDLength]
		} else {
			shortContainerID = container.ID
		}

		ms := entity.NewMetricSet(containerSampleName,
			metric.Attr(attrContainerID, container.ID),
			metric.Attr(attrShortContainerID, shortContainerID),
		)

		// populating metrics that are common to running and stopped containers
		populate(ms, attributes(container))
		populate(ms, labels(container))
		populate(ms, storageEntry)

		// TODO: this *needs* to be refactored into the call to ContainerList, because different
		// systems might represent running containers in a slightly different way. This can be tricky
		// because for Docker containers we're relying on the capabilities of the official Docker client.
		// Possibly wrapping the Docker client with another type is a good solution.
		// i.e. Docker uses `state = running` and ECS uses `status = RUNNING`.
		// If State is empty, we check by status up.
		if strings.ToLower(container.State) != "running" &&
			strings.ToLower(container.Status) != "running" &&
			!strings.HasPrefix(strings.ToLower(container.Status), "up") {
			log.Debug("Skipped not running container: %s.", container.ID)
			continue
		}

		// populating metrics that only apply to running containers
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
			log.Warn("Unexpected error setting metric %v: %v", metric, err)
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

// labelRename contains a list of labels that should be
// renamed. It does not rename the label in place, but creates
// a copy to ensure we are not deleting any data.
var labelRename = map[string]string{
	"com.amazonaws.ecs.container-name":          "ecsContainerName",
	"com.amazonaws.ecs.cluster":                 "ecsClusterName",
	"com.amazonaws.ecs.task-arn":                "ecsTaskArn",
	"com.amazonaws.ecs.task-definition-family":  "ecsTaskDefinitionFamily",
	"com.amazonaws.ecs.task-definition-version": "ecsTaskDefinitionVersion",

	// com.newrelic.nri-docker.* labels are created by aws.processFargateLabels containing fargate info
	"com.newrelic.nri-docker.launch-type": "ecsLaunchType",
	"com.newrelic.nri-docker.cluster-arn": "ecsClusterArn",
	"com.newrelic.nri-docker.aws-region":  "awsRegion",
}

func labels(container types.Container) []entry {
	metrics := make([]entry, 0, len(container.Labels))
	for key, val := range container.Labels {
		if newName, ok := labelRename[key]; ok {
			metrics = append(metrics, entry{
				Name:  newName,
				Value: val,
				Type:  metric.ATTRIBUTE,
			})
		}

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
		metricMemoryKernelUsageBytes(mem.KernelUsageBytes),
		metricMemorySwapUsageBytes(mem.SwapUsageBytes),
		metricMemorySwapOnlyUsageBytes(mem.SwapOnlyUsageBytes),
	}
	if mem.MemLimitBytes > 0 {
		metrics = append(metrics,
			metricMemorySizeLimitBytes(mem.MemLimitBytes),
			metricMemoryUsageLimitPercent(mem.UsagePercent),
		)
	}
	if mem.SwapLimitBytes > 0 {
		metrics = append(metrics,
			metricMemorySwapLimitBytes(mem.SwapLimitBytes),
			metricMemorySwapLimitUsagePercent(mem.SwapLimitUsagePercent),
		)
	}
	if mem.SoftLimitBytes > 0 {
		metrics = append(metrics,
			metricMemorySoftLimitBytes(mem.SoftLimitBytes),
		)
	}
	return metrics
}

func pids(pids *biz.Pids) []entry {
	return []entry{
		metricThreadCount(pids.Current),
		metricThreadCountLimit(pids.Limit),
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
		metricCPUShares(cpu.Shares),
	}
}

func misc(m *biz.Sample) []entry {
	return []entry{
		metricRestartCount(m.RestartCount),
	}
}

func getStorageEntry(m *biz.DeviceMapperStats) []entry {
	if m == nil {
		return []entry{}
	}
	return []entry{
		metricStorageDataUsed(m.DataUsed),
		metricStorageDataAvailable(m.DataAvailable),
		metricStorageDataTotal(m.DataTotal),
		metricStorageMetadataUsed(m.MetadataUsed),
		metricStorageMetadataAvailable(m.MetadataAvailable),
		metricStorageMetadataTotal(m.MetadataTotal),
	}
}
