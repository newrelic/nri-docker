package nri

import (
	"context"
	"testing"
	"time"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/stretchr/testify/assert"

	"github.com/docker/docker/api/types"

	"github.com/stretchr/testify/mock"

	"github.com/newrelic/nri-docker/src/biz"
	"github.com/newrelic/nri-docker/src/raw"
)

// mocker is a Docker mock
type mocker struct {
	mock.Mock
}

func (m *mocker) ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error) {
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

	mStore := &mockStorer{}
	mStore.On("Save").Return(nil)

	sampler := ContainerSampler{
		metrics: biz.NewProcessor(mStore, nil, mocker, 0),
		docker:  mocker,
		store:   mStore,
	}

	i, err := integration.New("test", "test-version")
	assert.NoError(t, err)
	assert.NoError(t, sampler.SampleAll(context.Background(), i, types.Info{}))

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
	mocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{
		{
			ID:      "containerid",
			Names:   []string{"Container 1"},
			Image:   "my_image",
			ImageID: "my_image_id",
		},
	}, nil)
	mocker.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Status:     "exited",
				FinishedAt: time.Now().Add(-2 * time.Hour).Format(time.RFC3339Nano),
			},
		},
	}, nil)
	mStore := &mockStorer{}
	mStore.On("Save").Return(nil)

	sampler := ContainerSampler{
		metrics: biz.NewProcessor(mStore, nil, mocker, 30*time.Minute),
		docker:  mocker,
		store:   mStore,
	}

	i, err := integration.New("test", "test-version")
	assert.NoError(t, err)

	err = sampler.SampleAll(context.Background(), i, types.Info{})
	assert.NoError(t, err)
	assert.Empty(t, i.Entities)
}

func TestSampleAll(t *testing.T) {
	mocker := &mocker{}
	mocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{
		{
			ID:      "containerid",
			Names:   []string{"Container 1"},
			Image:   "my_image",
			ImageID: "my_image_id",
			State:   "",
			Labels: map[string]string{
				"value":   "foo",
				"noValue": "",
			},
		},
	}, nil)
	mocker.On("ContainerInspect", mock.Anything, mock.Anything).Return(types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Status:     "exited",
				FinishedAt: time.Now().Add(-15 * time.Minute).Format(time.RFC3339Nano),
			},
		},
	}, nil)

	info := types.Info{
		Driver: "devicemapper",
		DriverStatus: [][2]string{
			{"Data Space Total", "102 GB"},
		},
	}

	mStore := &mockStorer{}
	mStore.On("Save").Return(nil)
	mStore.On("Get", mock.Anything, mock.Anything).Return(int64(0), nil)
	mStore.On("Set", mock.Anything, mock.Anything).Return(int64(0))

	fetcher := &mockFetcher{}
	fetcher.On("Fetch", mock.Anything).Return(raw.Metrics{}, nil)

	sampler := ContainerSampler{
		metrics: biz.NewProcessor(mStore, fetcher, mocker, 30*time.Minute),
		docker:  mocker,
		store:   mStore,
	}

	i, err := integration.New("test", "test-version")
	assert.NoError(t, err)

	err = sampler.SampleAll(context.Background(), i, info)
	assert.NoError(t, err)
	assert.Len(t, i.Entities, 1)
	assert.Equal(t, i.Entities[0].Metadata.Name, "containerid")
	assert.Equal(t, i.Entities[0].Metrics[0].Metrics["storageDataTotalBytes"], 102e9)

	assert.Equal(t, i.Entities[0].Metrics[0].Metrics["image"], "my_image_id")
	assert.Equal(t, i.Entities[0].Metrics[0].Metrics["label.value"], "foo")

	// emtpy labels are populated
	assert.Equal(t, i.Entities[0].Metrics[0].Metrics["label.noValue"], "")

	// container attributes are not populated with emtpy values
	assert.NotContains(t, i.Entities[0].Metrics[0].Metrics, "state")
}

func TestMemoryMetrics(t *testing.T) {
	var tests = []struct {
		name  string
		input *biz.Memory
		check []entry
	}{
		{
			name: "all metrics presents",
			input: &biz.Memory{
				UsageBytes:       1,
				CacheUsageBytes:  2,
				RSSUsageBytes:    3,
				MemLimitBytes:    4,
				UsagePercent:     5,
				KernelUsageBytes: 6,
				SoftLimitBytes:   7,
				SwapLimitBytes:   8,
				// Without thrse metrics none is set
				// SwapUsageBytes:        nil,
				// SwapOnlyUsageBytes:    nil,
				// SwapLimitUsagePercent: nil,
			},
			check: []entry{
				{Name: "memoryCacheBytes", Value: uint64(2)},
				{Name: "memoryUsageBytes", Value: uint64(1)},
				{Name: "memoryResidentSizeBytes", Value: uint64(3)},
				{Name: "memoryKernelUsageBytes", Value: uint64(6)},
				{Name: "memorySizeLimitBytes", Value: uint64(4)},
				{Name: "memoryUsageLimitPercent", Value: float64(5)},
				{Name: "memorySoftLimitBytes", Value: uint64(7)},
				{Name: "memorySwapLimitBytes", Value: uint64(8)},
			},
		},
		{
			name: "all metrics presents",
			input: &biz.Memory{
				UsageBytes:            1,
				CacheUsageBytes:       2,
				RSSUsageBytes:         3,
				MemLimitBytes:         4,
				UsagePercent:          5,
				KernelUsageBytes:      0,
				SoftLimitBytes:        7,
				SwapLimitBytes:        8,
				SwapUsageBytes:        uint64ToPointer(9),
				SwapOnlyUsageBytes:    uint64ToPointer(10),
				SwapLimitUsagePercent: float64ToPointer(11),
			},
			check: []entry{
				{Name: "memoryCacheBytes", Value: uint64(2)},
				{Name: "memoryUsageBytes", Value: uint64(1)},
				{Name: "memoryResidentSizeBytes", Value: uint64(3)},
				{Name: "memoryKernelUsageBytes", Value: uint64(0)},
				{Name: "memorySizeLimitBytes", Value: uint64(4)},
				{Name: "memoryUsageLimitPercent", Value: float64(5)},
				{Name: "memorySoftLimitBytes", Value: uint64(7)},
				{Name: "memorySwapLimitBytes", Value: uint64(8)},
				{Name: "memorySwapLimitUsagePercent", Value: float64(11)},
				{Name: "memorySwapUsageBytes", Value: uint64(9)},
				{Name: "memorySwapOnlyUsageBytes", Value: uint64(10)},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := memory(tt.input)
			assert.Equal(t, tt.check, res)
		})
	}
}

func uint64ToPointer(u uint64) *uint64 {
	return &u
}

func float64ToPointer(f float64) *float64 {
	return &f
}
