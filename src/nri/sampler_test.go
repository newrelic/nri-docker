package nri

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"

	"github.com/stretchr/testify/mock"

	"github.com/newrelic/nri-docker/src/biz"
	"github.com/newrelic/nri-docker/src/constants"
	"github.com/newrelic/nri-docker/src/raw"
)

// mocker is a Docker mock
type mocker struct {
	mock.Mock
}

func (m *mocker) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	args := m.Called(ctx, options)
	return args.Get(0).([]types.Container), args.Error(1)
}

func (m *mocker) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	args := m.Called(ctx, containerID)
	return args.Get(0).(types.ContainerJSON), args.Error(1)
}

type mockStorer struct {
	mock.Mock
}

func (m *mockStorer) Set(key string, value interface{}) int64 {
	args := m.Called(key, value)
	return args.Get(0).(int64)
}
func (m *mockStorer) Get(key string, valuePtr interface{}) (int64, error) {
	args := m.Called(key, valuePtr)
	return args.Get(0).(int64), args.Error(1)
}
func (m *mockStorer) Delete(key string) error {
	return m.Called(key).Error(0)
}
func (m *mockStorer) Save() error {
	return m.Called().Error(0)
}

func storerMock() *mockStorer {
	mStore := mockStorer{}
	mStore.On("Save").Return(nil)
	mStore.On("Get", mock.Anything, mock.Anything).Return(int64(0), nil)
	mStore.On("Set", mock.Anything, mock.Anything).Return(int64(0))

	return &mStore
}

type mockFetcher struct {
	mock.Mock
}

func (m *mockFetcher) Fetch(json types.ContainerJSON) (raw.Metrics, error) {
	args := m.Called(json)
	return args.Get(0).(raw.Metrics), nil
}

func TestECSLabelRename(t *testing.T) {
	var (
		givenLabels = map[string]string{
			"com.amazonaws.ecs.container-name":          "the-container-name",
			"com.amazonaws.ecs.cluster":                 "the-cluster-name",
			"com.amazonaws.ecs.task-arn":                "the-task-arn",
			"com.amazonaws.ecs.task-definition-family":  "the-task-definition-family",
			"com.amazonaws.ecs.task-definition-version": "the-task-definition-version",
			"my-label-name":                             "my-label-value",
		}
		expectedLabels = map[string]string{
			// the original labels
			"label.com.amazonaws.ecs.container-name":          "the-container-name",
			"label.com.amazonaws.ecs.cluster":                 "the-cluster-name",
			"label.com.amazonaws.ecs.task-arn":                "the-task-arn",
			"label.com.amazonaws.ecs.task-definition-family":  "the-task-definition-family",
			"label.com.amazonaws.ecs.task-definition-version": "the-task-definition-version",
			// the normalized ECS labels, not prefixed with "label."
			"ecsContainerName":         "the-container-name",
			"ecsClusterName":           "the-cluster-name",
			"ecsTaskArn":               "the-task-arn",
			"ecsTaskDefinitionFamily":  "the-task-definition-family",
			"ecsTaskDefinitionVersion": "the-task-definition-version",
			// the random label
			"label.my-label-name": "my-label-value",
		}
	)
	mocker := &mocker{}
	mocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{
		{
			ID:      "containerid",
			Names:   []string{"Container 1"},
			Image:   "my_image",
			ImageID: "my_image_id",
			Labels:  givenLabels,
		},
	}, nil)
	mocker.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{}, nil)

	mStore := storerMock()

	sampler := ContainerSampler{
		metrics: biz.NewProcessor(mStore, nil, mocker, 0),
		docker:  mocker,
		store:   mStore,
	}

	i, err := integration.New("test", "test-version")
	assert.NoError(t, err)
	assert.NoError(t, sampler.SampleAll(context.Background(), i, system.Info{}))

	for expectedName, expectedValue := range expectedLabels {
		value, ok := i.Entities[0].Metrics[0].Metrics[expectedName]
		if !ok {
			t.Errorf("Expected label '%s=%s' not found.", expectedName, expectedValue)
		}

		if value != expectedValue {
			t.Errorf("Label %s has value of %v, expected %s", expectedName, value, expectedValue)
		}
	}
}

func TestExitedContainerTTLExpired(t *testing.T) {
	mocker := &mocker{}
	mocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{testingContainer}, nil)
	mocker.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Status:     "exited",
				FinishedAt: time.Now().Add(-2 * time.Hour).Format(time.RFC3339Nano),
			},
		},
	}, nil)

	mStore := storerMock()

	sampler := ContainerSampler{
		metrics: biz.NewProcessor(mStore, nil, mocker, 30*time.Minute),
		docker:  mocker,
		store:   mStore,
	}

	i, err := integration.New("test", "test-version")
	assert.NoError(t, err)

	err = sampler.SampleAll(context.Background(), i, system.Info{})
	assert.NoError(t, err)
	assert.Empty(t, i.Entities)
}

//nolint:funlen // this is a test
func TestSampleAll(t *testing.T) {
	mocker := &mocker{}
	mocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{testingContainer}, nil)
	mocker.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Status: "running",
			},
			RestartCount: 1,
		},
	}, nil)

	mStore := storerMock()

	fetcher := &mockFetcher{}
	fetcher.On("Fetch", mock.Anything).Return(allMetrics(), nil)

	sampler := ContainerSampler{
		metrics: biz.NewProcessor(mStore, fetcher, mocker, 30*time.Minute),
		docker:  mocker,
		store:   mStore,
	}

	i, err := integration.New("test", "test-version")
	assert.NoError(t, err)

	err = sampler.SampleAll(context.Background(), i, cgroupInfo)
	assert.NoError(t, err)
	require.Len(t, i.Entities, 1)
	require.Len(t, i.Entities[0].Metrics, 1)

	metrics := i.Entities[0].Metrics[0].Metrics

	assert.Equal(t, i.Entities[0].Metadata.Name, containerID)

	// Container attributes
	assert.Equal(t, "image_id", metrics["image"])
	assert.Equal(t, "command", metrics["commandLine"])
	assert.Equal(t, "image", metrics["imageName"])
	assert.Equal(t, "name", metrics["name"])
	assert.Equal(t, "running", metrics["status"])
	assert.Equal(t, float64(1), metrics["restartCount"])
	assert.NotContains(t, metrics, "state", "container attributes are not populated with empty values")

	// Labels
	assert.Equal(t, metrics["label.noValue"], "", "empty label value should be preserved")
	assert.Equal(t, metrics["label.value"], "foo")

	// DataStorage
	assert.NotZero(t, metrics["storageDataUsedBytes"])
	assert.NotZero(t, metrics["storageDataAvailableBytes"])
	assert.NotZero(t, metrics["storageDataTotalBytes"])
	assert.NotZero(t, metrics["storageDataUsagePercent"])
	assert.NotZero(t, metrics["storageMetadataUsedBytes"])
	assert.NotZero(t, metrics["storageMetadataAvailableBytes"])
	assert.NotZero(t, metrics["storageMetadataTotalBytes"])
	assert.NotZero(t, metrics["storageMetadataUsagePercent"])

	// Memory
	assert.NotZero(t, metrics["memoryUsageLimitPercent"])
	if runtime.GOOS == constants.WindowsPlatformName {
		assert.NotZero(t, metrics["memoryCommitBytes"])
		assert.NotZero(t, metrics["memoryCommitPeakBytes"])
		assert.NotZero(t, metrics["memoryPrivateWorkingSet"])
	} else {
		assert.NotZero(t, metrics["memoryUsageBytes"])
		assert.NotZero(t, metrics["memoryCacheBytes"])
		assert.NotZero(t, metrics["memoryResidentSizeBytes"])
		assert.NotZero(t, metrics["memorySizeLimitBytes"])
		assert.NotZero(t, metrics["memoryKernelUsageBytes"])
		assert.NotZero(t, metrics["memorySwapUsageBytes"])
		assert.NotZero(t, metrics["memorySwapOnlyUsageBytes"])
		assert.NotZero(t, metrics["memorySwapLimitBytes"])
		assert.NotZero(t, metrics["memorySwapLimitUsagePercent"])
		assert.NotZero(t, metrics["memorySoftLimitBytes"])
	}

	// Pids
	assert.NotZero(t, metrics["threadCount"])
	assert.NotZero(t, metrics["threadCountLimit"])

	// Network
	// Missing persecond metrics that needs store to be calculated
	assert.NotZero(t, metrics["networkRxBytes"])
	assert.NotZero(t, metrics["networkRxDropped"])
	assert.NotZero(t, metrics["networkRxErrors"])
	assert.NotZero(t, metrics["networkRxPackets"])
	assert.NotZero(t, metrics["networkTxBytes"])
	assert.NotZero(t, metrics["networkTxDropped"])
	assert.NotZero(t, metrics["networkTxErrors"])
	assert.NotZero(t, metrics["networkTxPackets"])

	// Missing CPU metrics that needs store to be calculated

	// io
	// Missing persecond metrics that needs store to be calculated
	assert.NotZero(t, metrics["ioTotalReadCount"])
	assert.NotZero(t, metrics["ioTotalWriteCount"])
	assert.NotZero(t, metrics["ioTotalReadBytes"])
	assert.NotZero(t, metrics["ioTotalWriteBytes"])
	assert.NotZero(t, metrics["ioTotalBytes"])
}

func TestSampleAllMissingMetrics(t *testing.T) {
	mocker := &mocker{}
	mocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{testingContainer}, nil)
	mocker.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{State: &types.ContainerState{Status: "running"}}},
		nil,
	)

	mStore := storerMock()

	fetcher := &mockFetcher{}
	fetcher.On("Fetch", mock.Anything).Return(requiredMetrics(), nil)

	sampler := ContainerSampler{
		metrics: biz.NewProcessor(mStore, fetcher, mocker, 30*time.Minute),
		docker:  mocker,
		store:   mStore,
	}

	i, err := integration.New("test", "test-version")
	assert.NoError(t, err)

	err = sampler.SampleAll(context.Background(), i, cgroupInfo)
	assert.NoError(t, err)
	require.Len(t, i.Entities, 1)
	require.Len(t, i.Entities[0].Metrics, 1)

	metrics := i.Entities[0].Metrics[0].Metrics

	// not required metrics should not be present
	// IO
	assert.NotContains(t, metrics, "ioTotalReadCount")
	assert.NotContains(t, metrics, "ioTotalWriteCount")
	assert.NotContains(t, metrics, "ioTotalReadBytes")
	assert.NotContains(t, metrics, "ioTotalWriteBytes")
	assert.NotContains(t, metrics, "ioTotalBytes")
	// Memory
	assert.NotContains(t, metrics, "memorySwapUsageBytes")
	assert.NotContains(t, metrics, "memorySwapOnlyUsageBytes")
	assert.NotContains(t, metrics, "memorySwapLimitUsagePercent")

	// check required metrics are collected
	if runtime.GOOS == constants.WindowsPlatformName {
		assert.NotZero(t, metrics["memoryCommitBytes"])
		assert.NotZero(t, metrics["memoryCommitPeakBytes"])
		assert.NotZero(t, metrics["memoryPrivateWorkingSet"])
	} else {
		assert.NotZero(t, metrics["memoryUsageBytes"])
	}
}

const (
	nonZeroUint uint64 = 100
	nonZero     int64  = 100
)

const containerID = "containerid"

var testingContainer = types.Container{
	ID:      containerID,
	Names:   []string{"name"},
	Image:   "image",
	ImageID: "image_id",
	State:   "",
	Status:  "running",
	Command: "command",
	Labels: map[string]string{
		"value":   "foo",
		"noValue": "",
	},
}
var cgroupInfo = system.Info{
	Driver: "devicemapper",
	DriverStatus: [][2]string{
		{"Data Space Used", "1920.92 MB"},
		{"Data Space Total", "102 GB"},
		{"Data Space Available", "100.13 GB"},
		{"Metadata Space Used", "147.5 kB"},
		{"Metadata Space Total", "1.07 GB"},
		{"Metadata Space Available", "1.069 GB"},
	},
}

func requiredMetrics() raw.Metrics {
	m := allMetrics()
	// Remove metrics that are not required
	m.Blkio.IoServiceBytesRecursive = nil
	m.Blkio.IoServicedRecursive = nil

	m.Memory.SwapUsage = nil

	return m
}

func allMetrics() raw.Metrics {
	// must be grater than FuzzUsage to avoid having onlySwap metric equal zero
	swapValue := nonZeroUint * 2
	return raw.Metrics{
		ContainerID: containerID,
		Memory: raw.Memory{
			UsageLimit:        nonZeroUint,
			Cache:             nonZeroUint,
			RSS:               nonZeroUint,
			SwapUsage:         &swapValue,
			FuzzUsage:         nonZeroUint,
			KernelMemoryUsage: nonZeroUint,
			SwapLimit:         nonZeroUint,
			SoftLimit:         nonZeroUint,
			Commit:            nonZeroUint,
			CommitPeak:        nonZeroUint,
			PrivateWorkingSet: nonZeroUint,
		},
		Network: raw.Network{
			RxBytes:   nonZero,
			RxDropped: nonZero,
			RxErrors:  nonZero,
			RxPackets: nonZero,
			TxBytes:   nonZero,
			TxDropped: nonZero,
			TxErrors:  nonZero,
			TxPackets: nonZero,
		},
		CPU: raw.CPU{},
		Pids: raw.Pids{
			Current: nonZeroUint,
			Limit:   nonZeroUint,
		},
		Blkio: raw.Blkio{
			IoServiceBytesRecursive: []raw.BlkioEntry{
				{
					Op:    "Read",
					Value: nonZeroUint,
				},
				{
					Op:    "Write",
					Value: nonZeroUint,
				},
			},
			IoServicedRecursive: []raw.BlkioEntry{
				{
					Op:    "Read",
					Value: nonZeroUint,
				},
				{
					Op:    "Write",
					Value: nonZeroUint,
				},
			},
		},
	}
}
