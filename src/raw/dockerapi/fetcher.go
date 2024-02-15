package dockerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-docker/src/raw"
)

type Fetcher struct {
	statsClient raw.DockerStatsClient
}

func NewFetcher(statsClient raw.DockerStatsClient) *Fetcher {
	return &Fetcher{statsClient: statsClient}
}

func (f *Fetcher) Fetch(container types.ContainerJSON) (raw.Metrics, error) {
	containerStats, err := f.containerStats(context.Background(), container.ID)
	if err != nil {
		return raw.Metrics{}, fmt.Errorf("could not fetch stats for container %s: %w", container.ID, err)
	}
	metrics := raw.Metrics{
		Time:        time.Now(), // nolint: staticcheck
		ContainerID: container.ID,
		Memory:      f.memoryMetrics(containerStats),
		Network:     f.networkMetrics(containerStats),
		CPU:         f.cpuMetrics(container, containerStats.CPUStats),
		Pids:        f.pidsMetrics(containerStats),
		Blkio:       f.blkioMetrics(containerStats),
	}
	return metrics, nil
}

func (f *Fetcher) memoryMetrics(containerStats types.StatsJSON) raw.Memory {
	return raw.Memory{}
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

func (f *Fetcher) pidsMetrics(containerStats types.StatsJSON) raw.Pids {
	return raw.Pids{}
}

func (f *Fetcher) blkioMetrics(containerStats types.StatsJSON) raw.Blkio {
	return raw.Blkio{}
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
