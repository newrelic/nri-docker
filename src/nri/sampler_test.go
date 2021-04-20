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
func (m *mocker) Info(ctx context.Context) (types.Info, error) {
	args := m.Called(ctx)
	return args.Get(0).(types.Info), args.Error(1)
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

	mocker.On("Info", mock.Anything, mock.Anything).Return(types.Info{}, nil)

	mStore := &mockStorer{}
	mStore.On("Save").Return(nil)

	sampler := ContainerSampler{
		metrics: biz.NewProcessor(mStore, nil, mocker, 0),
		docker:  mocker,
		store:   mStore,
	}

	i, err := integration.New("test", "test-version")
	assert.NoError(t, err)
	assert.NoError(t, sampler.SampleAll(context.Background(), i))

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
	mocker.On("Info", mock.Anything, mock.Anything).Return(types.Info{}, nil)
	mStore := &mockStorer{}
	mStore.On("Save").Return(nil)

	sampler := ContainerSampler{
		metrics: biz.NewProcessor(mStore, nil, mocker, 30*time.Minute),
		docker:  mocker,
		store:   mStore,
	}

	i, err := integration.New("test", "test-version")
	assert.NoError(t, err)

	err = sampler.SampleAll(context.Background(), i)
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

	mocker.On("Info", mock.Anything, mock.Anything).Return(types.Info{
		Driver: "devicemapper",
		DriverStatus: [][2]string{
			{"Data Space Total", "102 GB"},
		},
	}, nil)

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

	err = sampler.SampleAll(context.Background(), i)
	assert.NoError(t, err)
	assert.Len(t, i.Entities, 1)
	assert.Equal(t, i.Entities[0].Metadata.Name, "containerid")
	assert.Equal(t, i.Entities[0].Metrics[0].Metrics["storageDataTotalBytes"], 102e9)
}
