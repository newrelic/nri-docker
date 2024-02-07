package dockerapi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/newrelic/nri-docker/src/raw"
)

type Fetcher struct {
	statsClient raw.DockerStatsClient
}

func NewFetcher(statsClient raw.DockerStatsClient) *Fetcher {
	return &Fetcher{statsClient: statsClient}
}

func (f *Fetcher) Fetch(container types.ContainerJSON) (raw.Metrics, error) {
	containerStats, err := f.ContainerStats(context.Background(), container.ID)
	if err != nil {
		return raw.Metrics{}, fmt.Errorf("could not fetch stats for container %s: %w", container.ID, err)
	}
	now := time.Now() // nolint: staticcheck
	metrics := raw.Metrics{
		Time:        now,
		ContainerID: container.ID,
		Memory:      f.memoryMetrics(containerStats),
		Network:     f.networkMetrics(containerStats),
		CPU:         f.cpuMetrics(containerStats),
		Pids:        f.pidsMetrics(containerStats),
		Blkio:       f.blkioMetrics(containerStats),
	}
	return metrics, nil
}

func (f *Fetcher) memoryMetrics(containerStats types.StatsJSON) raw.Memory {
	panic("unimplemented")
}

func (f *Fetcher) networkMetrics(containerStats types.StatsJSON) raw.Network {
	panic("unimplemented")
}

func (f *Fetcher) cpuMetrics(containerStats types.StatsJSON) raw.CPU {
	panic("unimplemented")
}

func (f *Fetcher) pidsMetrics(containerStats types.StatsJSON) raw.Pids {
	panic("unimplemented")
}

func (f *Fetcher) blkioMetrics(containerStats types.StatsJSON) raw.Blkio {
	panic("unimplemented")
}

// TODO: the helper below should be private when the corresponding fetcher is ready to be used.
func (f *Fetcher) ContainerStats(ctx context.Context, containerID string) (types.StatsJSON, error) {
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
