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
		BlkioStats: types.BlkioStats{
			IoServiceBytesRecursive: []types.BlkioStatEntry{
				{Op: "Read", Value: 5885952, Major: 202, Minor: 26468},
				{Op: "Write", Value: 45056, Major: 202, Minor: 26468},
				{Op: "Sync", Value: 5931008, Major: 202, Minor: 26468},
				{Op: "Async", Value: 0, Major: 202, Minor: 26468},
				{Op: "Total", Value: 5931008, Major: 202, Minor: 26468},
			},
			IoServicedRecursive: []types.BlkioStatEntry{
				{Op: "Read", Value: 341, Major: 202, Minor: 26468},
				{Op: "Write", Value: 11, Major: 202, Minor: 26468},
				{Op: "Sync", Value: 352, Major: 202, Minor: 26468},
				{Op: "Async", Value: 0, Major: 202, Minor: 26468},
				{Op: "Total", Value: 352, Major: 202, Minor: 26468},
			},
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
		PidsStats: types.PidsStats{
			Current: mockMetricValue,
			Limit:   mockMetricValue,
		},
	},
	Networks: map[string]types.NetworkStats{
		"eth0": mockNetworkStats,
		"eth1": mockNetworkStats,
		"eth2": mockNetworkStats,
	},
}

const mockMetricValue = 1

var mockNetworkStats = types.NetworkStats{
	RxBytes:   mockMetricValue,
	RxErrors:  mockMetricValue,
	RxPackets: mockMetricValue,
	RxDropped: mockMetricValue,
	TxBytes:   mockMetricValue,
	TxErrors:  mockMetricValue,
	TxPackets: mockMetricValue,
	TxDropped: mockMetricValue,
}

func Test_Fetch(t *testing.T) {
	client := mockDockerStatsClient{}
	client.On("ContainerStats", mock.Anything).Return(mockStats)

	fetcher := dockerapi.NewFetcher(&client)

	metrics, err := fetcher.Fetch(types.ContainerJSON{ContainerJSONBase: &types.ContainerJSONBase{
		ID: "test",
		HostConfig: &container.HostConfig{
			Resources: container.Resources{
				// Required by memory metrics test
				Memory:            1024 * 1024 * 500, // 500 MB
				MemoryReservation: 1024 * 1024 * 250, // 250 MB, for SoftLimit calculation
				MemorySwap:        1024 * 1024 * 600, // Total memory + swap
				// Required by cpu metric test
				CPUShares: 2048,
			},
		},
	}})
	require.NoError(t, err)

	t.Run("Network metrics", func(t *testing.T) {
		t.Parallel()
		// Multiply by the number of interfaces since network metrics are aggregated across all net interfaces.
		expectedValue := int64(mockMetricValue * 3)
		assert.Equal(t, expectedValue, metrics.Network.RxBytes)
		assert.Equal(t, expectedValue, metrics.Network.RxDropped)
		assert.Equal(t, expectedValue, metrics.Network.RxErrors)
		assert.Equal(t, expectedValue, metrics.Network.RxPackets)
		assert.Equal(t, expectedValue, metrics.Network.TxBytes)
		assert.Equal(t, expectedValue, metrics.Network.TxDropped)
		assert.Equal(t, expectedValue, metrics.Network.TxErrors)
		assert.Equal(t, expectedValue, metrics.Network.TxPackets)
	})

	t.Run("Pid metrics", func(t *testing.T) {
		t.Parallel()

		expectedValue := uint64(mockMetricValue)
		assert.Equal(t, expectedValue, metrics.Pids.Current)
		assert.Equal(t, expectedValue, metrics.Pids.Limit)
	})

	t.Run("Memory metrics", func(t *testing.T) {
		expectedMemoryMetrics := raw.Memory{
			UsageLimit:        1024 * 1024 * 500, // 500 MB total memory limit
			FuzzUsage:         1024 * 1024 * 250, // 250 MB current memory usage
			SwapUsage:         0,
			SwapLimit:         1024 * 1024 * 100, // 100 MB
			SoftLimit:         1024 * 1024 * 250, // 250 MB memory reservation (soft limit)
			Cache:             1024 * 1024 * 25,  // 25 MB cache usage
			RSS:               1024 * 1024 * 75,  // 75 MB RSS usage
			KernelMemoryUsage: 1024 * 1024 * 10,  // 10 MB kernel memory usage (kernel_stack + slab)
		}
		assert.Equal(t, expectedMemoryMetrics, metrics.Memory)
	})

	t.Run("CPU metrics", func(t *testing.T) {
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

		assert.Equal(t, expectedCPUMetrics, metrics.CPU)
	})

	t.Run("Blkio metrics", func(t *testing.T) {
		expectedBlkioMetrics := raw.Blkio{
			IoServiceBytesRecursive: []raw.BlkioEntry{
				{Op: "Read", Value: 5885952},
				{Op: "Write", Value: 45056},
				{Op: "Sync", Value: 5931008},
				{Op: "Async", Value: 0},
				{Op: "Total", Value: 5931008},
			},
			IoServicedRecursive: []raw.BlkioEntry{
				{Op: "Read", Value: 341},
				{Op: "Write", Value: 11},
				{Op: "Sync", Value: 352},
				{Op: "Async", Value: 0},
				{Op: "Total", Value: 352},
			},
		}

		assert.Equal(t, expectedBlkioMetrics, metrics.Blkio)
	})

}

func Test_NilHostConfig(t *testing.T) {
	client := mockDockerStatsClient{}
	client.On("ContainerStats", mock.Anything).Return(mockStats)

	fetcher := dockerapi.NewFetcher(&client)

	metricsNoHostConfig, err := fetcher.Fetch(types.ContainerJSON{ContainerJSONBase: &types.ContainerJSONBase{ID: "test"}})
	require.NoError(t, err)

	assert.EqualValues(t, 0, metricsNoHostConfig.CPU.Shares, "When hostConfig is not available, cpu shares cannot be set")
	assert.EqualValues(t, 0, metricsNoHostConfig.Memory.SwapLimit, "When hostConfig is not available, SwapLimit cannot be set")
	assert.EqualValues(t, 0, metricsNoHostConfig.Memory.SoftLimit, "When hostConfig is not available, SoftLimit cannot be set")

}
