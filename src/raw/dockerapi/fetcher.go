package dockerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	"github.com/newrelic/nri-docker/src/raw"
)

type Fetcher struct {
	statsClient raw.DockerStatsClient
	platform    string
}

func NewFetcher(statsClient raw.DockerStatsClient, platform string) *Fetcher {
	return &Fetcher{statsClient: statsClient, platform: platform}
}

func (f *Fetcher) Fetch(container types.ContainerJSON) (raw.Metrics, error) {
	containerStats, err := f.containerStats(context.Background(), container.ID)
	if err != nil {
		return raw.Metrics{}, fmt.Errorf("could not fetch stats for container %s: %w", container.ID, err)
	}
	metrics := raw.Metrics{
		Time:        time.Now(), // nolint: staticcheck
		ContainerID: container.ID,
		Memory:      f.memoryMetrics(containerStats, container.HostConfig),
		Network:     f.networkMetrics(containerStats),
		CPU:         f.cpuMetrics(container, containerStats.CPUStats),
		Blkio:       f.blkioMetrics(containerStats.BlkioStats),
		Pids:        f.pidsMetrics(containerStats.PidsStats),
	}
	return metrics, nil
}

func (f *Fetcher) memoryMetrics(containerStats types.StatsJSON, hostConfig *container.HostConfig) raw.Memory {
	mem := raw.Memory{}

	// mem.UsageLimit and mem.FuzzUsage are fetched in the same way that for the cgroup file fetchers.
	if containerStats.MemoryStats.Usage != 0 {
		mem.UsageLimit = containerStats.MemoryStats.Limit
		mem.FuzzUsage = containerStats.MemoryStats.Usage
	}

	if hostConfig != nil {
		// mem.SwapUsage is not reported in the docker API, we keep its zero value. We're doing the same for Fargate.
		mem.SwapLimit = uint64(hostConfig.MemorySwap) - uint64(hostConfig.Memory)
		mem.SoftLimit = uint64(hostConfig.MemoryReservation)
	} else {
		log.Debug("received a nil hostConfig")
	}

	mem.Cache = getOrDebuglog(containerStats.MemoryStats.Stats, "file", "memory_stats.stats")
	mem.RSS = getOrDebuglog(containerStats.MemoryStats.Stats, "anon", "memory_stats.stats")
	mem.KernelMemoryUsage = getOrDebuglog(containerStats.MemoryStats.Stats, "kernel_stack", "memory_stats.stats") +
		getOrDebuglog(containerStats.MemoryStats.Stats, "slab", "memory_stats.stats")

	return mem
}

// networkMetrics aggregates and returns network metrics across all of a container's interfaces.
// All network metrics are monotonic counters that are represented with PRATE type of metric.
func (f *Fetcher) networkMetrics(containerStats types.StatsJSON) raw.Network {
	aggregatedMetrics := raw.Network{}
	for _, netStats := range containerStats.Networks {
		aggregatedMetrics.RxBytes += int64(netStats.RxBytes)
		aggregatedMetrics.RxDropped += int64(netStats.RxDropped)
		aggregatedMetrics.RxErrors += int64(netStats.RxErrors)
		aggregatedMetrics.RxPackets += int64(netStats.RxPackets)
		aggregatedMetrics.TxBytes += int64(netStats.TxBytes)
		aggregatedMetrics.TxDropped += int64(netStats.TxDropped)
		aggregatedMetrics.TxErrors += int64(netStats.TxErrors)
		aggregatedMetrics.TxPackets += int64(netStats.TxPackets)
	}

	return aggregatedMetrics
}

func (f *Fetcher) cpuMetrics(container types.ContainerJSON, cpuStats types.CPUStats) raw.CPU {
	var cpuShares uint64
	if container.HostConfig == nil {
		log.Debug("Could not fetch cpuShares since the container %q host configuration is not available", container.ID)
	} else {
		cpuShares = uint64(container.HostConfig.CPUShares)
	}
	return raw.CPU{
		TotalUsage:        cpuStats.CPUUsage.TotalUsage,
		UsageInUsermode:   cpuStats.CPUUsage.UsageInUsermode,
		UsageInKernelmode: cpuStats.CPUUsage.UsageInKernelmode,
		ThrottledPeriods:  cpuStats.ThrottlingData.ThrottledPeriods,
		ThrottledTimeNS:   cpuStats.ThrottlingData.ThrottledTime,
		SystemUsage:       cpuStats.SystemUsage,
		OnlineCPUs:        uint(cpuStats.OnlineCPUs),
		Shares:            cpuShares,
		// PercpuUsage is not set in cgroups v2 (it is set to nil) but it is not reported by the integration,
		// it is used to report the 'OnlineCPUs' value when online CPUs cannot be fetched.
		PercpuUsage: cpuStats.CPUUsage.PercpuUsage,
	}
}

func (f *Fetcher) pidsMetrics(pidStats types.PidsStats) raw.Pids {
	return raw.Pids{
		Current: pidStats.Current,
		Limit:   pidStats.Limit,
	}
}

func (f *Fetcher) blkioMetrics(blkioStats types.BlkioStats) raw.Blkio {
	return raw.Blkio{
		IoServiceBytesRecursive: toRawBlkioEntry(blkioStats.IoServiceBytesRecursive),
		IoServicedRecursive:     toRawBlkioEntry(blkioStats.IoServicedRecursive),
	}
}

func (f *Fetcher) containerStats(ctx context.Context, containerID string) (types.StatsJSON, error) {
	m, err := f.statsClient.ContainerStats(ctx, containerID, false)
	if err != nil {
		return types.StatsJSON{}, err
	}
	var statsJSON types.StatsJSON
	err = json.NewDecoder(m.Body).Decode(&statsJSON)
	m.Body.Close()
	if err != nil {
		return types.StatsJSON{}, err
	}
	return statsJSON, nil
}

func toRawBlkioEntry(entries []types.BlkioStatEntry) []raw.BlkioEntry {
	result := []raw.BlkioEntry{}
	for _, entry := range entries {
		result = append(result, raw.BlkioEntry{Op: entry.Op, Value: entry.Value})
	}
	return result
}

func getOrDebuglog(m map[string]uint64, key string, metricsPath string) uint64 { // nolint:unparam
	if val, ok := m[key]; ok {
		return val
	}
	log.Debug("Could not fetch metric value from docker API: the key %q was not found in %s", key, metricsPath)
	return 0
}
