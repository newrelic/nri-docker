package dockerapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/newrelic/nri-docker/src/raw"
	"github.com/newrelic/nri-docker/src/raw/dockerapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockDockerStatsClient struct {
	mock.Mock
}

func (m *mockDockerStatsClient) ContainerStats(ctx context.Context, containerID string, stream bool) (types.ContainerStats, error) {
	args := m.Called()

	statsJSON, _ := json.Marshal(args.Get(0).(types.StatsJSON))

	return types.ContainerStats{
		Body: io.NopCloser(bytes.NewReader(statsJSON)),
	}, nil
}

var mockStats = types.StatsJSON{
	Stats: types.Stats{
		CPUStats: types.CPUStats{
			CPUUsage: types.CPUUsage{
				TotalUsage:        11491000,
				UsageInKernelmode: 5745000,
				UsageInUsermode:   5745000,
			},
			ThrottlingData: types.ThrottlingData{
				Periods:          2,
				ThrottledPeriods: 1,
				ThrottledTime:    20000,
			},
			SystemUsage: 31532890000000,
			OnlineCPUs:  2,
		},
		MemoryStats: types.MemoryStats{
			Usage: 1024 * 1024 * 250, // 250 MB current memory usage
			Stats: map[string]uint64{
				"file":         1024 * 1024 * 25, // 25 MB cache usage
				"anon":         1024 * 1024 * 75, // 75 MB RSS usage
				"kernel_stack": 1024 * 1024 * 5,  // 5 MB kernel stack usage
				"slab":         1024 * 1024 * 5,  // 5 MB slab usage
			},
			Limit: 1024 * 1024 * 500, // 500 MB total memory limit
		},
	},
	Networks: map[string]types.NetworkStats{
		"eth0": mockNetworkStats,
		"eth1": mockNetworkStats,
		"eth2": mockNetworkStats,
	},
}

// All network metrics are monotonic counters,
const mockNetworkValue = 1

var mockNetworkStats = types.NetworkStats{
	RxBytes:   mockNetworkValue,
	RxErrors:  mockNetworkValue,
	RxPackets: mockNetworkValue,
	RxDropped: mockNetworkValue,
	TxBytes:   mockNetworkValue,
	TxErrors:  mockNetworkValue,
	TxPackets: mockNetworkValue,
	TxDropped: mockNetworkValue,
}

var expectedMetrics = raw.Metrics{
	Memory: raw.Memory{
		UsageLimit:        1024 * 1024 * 500, // 500 MB total memory limit
		FuzzUsage:         1024 * 1024 * 250, // 250 MB current memory usage
		SwapUsage:         0,
		SwapLimit:         1024 * 1024 * 100, // 100 MB
		SoftLimit:         1024 * 1024 * 250, // 250 MB memory reservation (soft limit)
		Cache:             1024 * 1024 * 25,  // 25 MB cache usage
		RSS:               1024 * 1024 * 75,  // 75 MB RSS usage
		KernelMemoryUsage: 1024 * 1024 * 10,  // 10 MB kernel memory usage (kernel_stack + slab)
	},
}

func Test_NetworkMetrics(t *testing.T) {
	client := mockDockerStatsClient{}
	client.On("ContainerStats", mock.Anything).Return(mockStats)

	fetcher := dockerapi.NewFetcher(&client)
	metrics, err := fetcher.Fetch(types.ContainerJSON{ContainerJSONBase: &types.ContainerJSONBase{ID: "test"}})
	require.NoError(t, err)

	// Multiply by the number of interfaces since network metrics are aggregated across all net interfaces.
	expectedValue := int64(mockNetworkValue * 3)
	assert.Equal(t, expectedValue, metrics.Network.RxBytes)
	assert.Equal(t, expectedValue, metrics.Network.RxDropped)
	assert.Equal(t, expectedValue, metrics.Network.RxErrors)
	assert.Equal(t, expectedValue, metrics.Network.RxPackets)
	assert.Equal(t, expectedValue, metrics.Network.TxBytes)
	assert.Equal(t, expectedValue, metrics.Network.TxDropped)
	assert.Equal(t, expectedValue, metrics.Network.TxErrors)
	assert.Equal(t, expectedValue, metrics.Network.TxPackets)
}
func Test_MemoryMetrics(t *testing.T) {
	client := mockDockerStatsClient{}
	client.On("ContainerStats", mock.Anything).Return(mockStats)

	fetcher := dockerapi.NewFetcher(&client)
	metrics, err := fetcher.Fetch(types.ContainerJSON{ContainerJSONBase: &types.ContainerJSONBase{
		ID: "test",
		HostConfig: &container.HostConfig{
			Resources: container.Resources{
				Memory:            1024 * 1024 * 500, // 500 MB
				MemoryReservation: 1024 * 1024 * 250, // 250 MB, for SoftLimit calculation
				MemorySwap:        1024 * 1024 * 600, // Total memory + swap
			},
		},
	}})
	require.NoError(t, err)

	assert.Equal(t, expectedMetrics.Memory, metrics.Memory)
}

func Test_CPUMetrics(t *testing.T) {
	client := mockDockerStatsClient{}
	client.On("ContainerStats", mock.Anything).Return(mockStats)

	fetcher := dockerapi.NewFetcher(&client)

	expectedCPUMetrics := raw.CPU{
		TotalUsage:        11491000,
		UsageInKernelmode: 5745000,
		UsageInUsermode:   5745000,
		PercpuUsage:       nil,
		Shares:            2048,
		ThrottledPeriods:  1,
		ThrottledTimeNS:   20000,
		SystemUsage:       31532890000000,
		OnlineCPUs:        2,
	}

	container := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:         "test",
			HostConfig: &container.HostConfig{Resources: container.Resources{CPUShares: 2048}},
		},
	}
	metrics, err := fetcher.Fetch(container)
	require.NoError(t, err)

	assert.Equal(t, expectedCPUMetrics, metrics.CPU)

	containerWithNoHostConfig := types.ContainerJSON{ContainerJSONBase: &types.ContainerJSONBase{ID: "test"}}
	metricsNoHostConfig, err := fetcher.Fetch(containerWithNoHostConfig)
	require.NoError(t, err)
	assert.EqualValues(t, 0, metricsNoHostConfig.CPU.Shares, "When hostConfig is not available, cpu shares cannot be set")
}
