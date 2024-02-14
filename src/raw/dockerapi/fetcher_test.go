package dockerapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/docker/docker/api/types"
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
