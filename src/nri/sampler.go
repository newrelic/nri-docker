package nri

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/newrelic/infra-integrations-sdk/data/attribute"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/persist"

	"github.com/newrelic/nri-docker/src/biz"
	"github.com/newrelic/nri-docker/src/config"
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
	docker  raw.DockerClient
	config  config.ArgumentList
}

// NewSampler returns a ContainerSampler instance.
func NewSampler(fetcher raw.Fetcher, docker raw.DockerClient, config config.ArgumentList) (*ContainerSampler, error) {
	cacheTTL, err := time.ParseDuration(config.CacheTTL)
	if err != nil {
		return nil, err
	}

	exitedContainerTTL, err := time.ParseDuration(config.ExitedContainersTTL)
	if err != nil {
		return nil, err
	}

	// SDK Storer to keep metric values between executions (e.g. for rates and deltas)
	store, err := persist.NewFileStore(
		persist.DefaultPath("container_cpus"),
		log.NewStdErr(true),
		cacheTTL)
	if err != nil {
		return nil, err
	}

	return &ContainerSampler{
		metrics: biz.NewProcessor(store, fetcher, docker, exitedContainerTTL),
		docker:  docker,
		store:   store,
		config:  config,
	}, nil
}

// SampleAll populates the integration of the argument with metrics and labels from all the containers in the system
// running and non-running
//
//nolint:gocyclo
func (cs *ContainerSampler) SampleAll(ctx context.Context, i *integration.Integration, cgroupInfo types.Info) error {
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

	var storageEntry []entry
	if !cs.config.DisableStorageMetrics {
		storageStats, err := biz.ParseDeviceMapperStats(cgroupInfo)
		if err != nil {
			log.Warn("computing Storage Driver stats: %s", err.Error())
		}
		storageEntry = getStorageEntry(storageStats)
	}

	for _, container := range containers {
		metrics, err := cs.metrics.Process(container.ID)
		if err != nil {
			switch {
			case errors.Is(err, biz.ErrExitedContainerExpired):
				log.Debug("skipping samples for container (%s): %s", container.ID, err.Error())
				continue
			case errors.Is(err, biz.ErrExitedContainerUnexpired):
				log.Debug("skipping fetching metrics from container (%s): %s", container.ID, err.Error())
			default:
				log.Error("fetching metrics for container %v: %v", container.ID, err)
			}
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
			attribute.Attr(attrContainerID, container.ID),
			attribute.Attr(attrShortContainerID, shortContainerID),
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
	for _, m := range metrics {
		if err := ms.SetMetric(m.Name, m.Value, m.Type); err != nil {
			log.Warn("Unexpected error setting metric %v: %v", m, err)
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
	entries := []entry{
		metricCommandLine(container.Command),
		metricContainerName(cname),
		metricContainerImage(container.ImageID),
		metricContainerImageName(container.Image),
		metricState(container.State),
		metricStatus(container.Status),
	}

	// Removes attributes with empty values to avoid be reported.
	sanitizedEntries := []entry{}
	for _, entry := range entries {
		if !isAttributeValueEmpty(entry) {
			sanitizedEntries = append(sanitizedEntries, entry)
		}
	}

	return sanitizedEntries
}

func isAttributeValueEmpty(e entry) bool {
	if e.Type == metric.ATTRIBUTE {
		strVal, ok := e.Value.(string)
		return ok && strVal == ""
	}
	return false
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
	}
	if mem.MemLimitBytes > 0 {
		metrics = append(metrics,
			metricMemorySizeLimitBytes(mem.MemLimitBytes),
			metricMemoryUsageLimitPercent(mem.UsagePercent),
		)
	}
	if mem.SoftLimitBytes > 0 {
		metrics = append(metrics, metricMemorySoftLimitBytes(mem.SoftLimitBytes))
	}
	if mem.SwapLimitBytes > 0 {
		metrics = append(metrics, metricMemorySwapLimitBytes(mem.SwapLimitBytes))
	}
	if mem.SwapLimitBytes > 0 && mem.SwapLimitUsagePercent != nil {
		metrics = append(metrics, metricMemorySwapLimitUsagePercent(*mem.SwapLimitUsagePercent))
	}
	if mem.SwapUsageBytes != nil {
		metrics = append(metrics, metricMemorySwapUsageBytes(*mem.SwapUsageBytes))
	}
	if mem.SwapOnlyUsageBytes != nil {
		metrics = append(metrics, metricMemorySwapOnlyUsageBytes(*mem.SwapOnlyUsageBytes))
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
	var entries []entry

	if bio.TotalReadCount != nil {
		entries = append(entries, metricIOTotalReadCount(*bio.TotalReadCount), metricIOReadCountPerSecond(*bio.TotalReadCount))
	}
	if bio.TotalWriteCount != nil {
		entries = append(entries, metricIOTotalWriteCount(*bio.TotalWriteCount), metricIOWriteCountPerSecond(*bio.TotalWriteCount))
	}
	if bio.TotalReadBytes != nil {
		entries = append(entries, metricIOTotalReadBytes(*bio.TotalReadBytes), metricIOReadBytesPerSecond(*bio.TotalReadBytes))
	}
	if bio.TotalWriteBytes != nil {
		entries = append(entries, metricIOTotalWriteBytes(*bio.TotalWriteBytes), metricIOWriteBytesPerSecond(*bio.TotalWriteBytes))
	}

	if bio.TotalReadBytes != nil || bio.TotalWriteBytes != nil {
		totalBytes := 0.0
		if bio.TotalReadBytes != nil {
			totalBytes += *bio.TotalReadBytes
		}
		if bio.TotalWriteBytes != nil {
			totalBytes += *bio.TotalWriteBytes
		}
		entries = append(entries, metricIOTotalBytes(totalBytes))
	}

	return entries
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
		metricStorageDataUsagePercent(m.DataUsagePercent),
		metricStorageMetadataUsed(m.MetadataUsed),
		metricStorageMetadataAvailable(m.MetadataAvailable),
		metricStorageMetadataTotal(m.MetadataTotal),
		metricStorageMetadataUsagePercent(m.MetadataUsagePercent),
	}
}
