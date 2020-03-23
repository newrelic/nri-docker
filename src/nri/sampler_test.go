package nri

import (
	"context"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/stretchr/testify/assert"

	"github.com/docker/docker/api/types"

	"github.com/stretchr/testify/mock"

	"github.com/newrelic/nri-docker/src/biz"
)

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

func TestLabelRename(t *testing.T) {

	mocker := &mocker{}
	mocker.On("ContainerList", mock.Anything, mock.Anything).Return([]types.Container{
		{
			ID:      "containerid",
			Names:   []string{"Container 1"},
			Image:   "my_image",
			ImageID: "my_image_id",
			Labels: map[string]string{
				"com.amazonaws.ecs.container-name": "my-container-name",
			},
		},
	}, nil)

	mStore := &mockStorer{}
	mStore.On("Save").Return(nil)

	sampler := ContainerSampler{
		metrics: biz.NewProcessor(mStore, nil, mocker),
		docker:  mocker,
		store:   mStore,
	}

	i, err := integration.New("test", "test-version")
	assert.NoError(t, err)
	assert.NoError(t, sampler.SampleAll(i))

	_, ok := i.Entities[0].Metrics[0].Metrics["ecsContainerName"]
	if !ok {
		t.Fatalf("Expected ecsContainerName field to be present, but it's not found")
	}
	//b, _ := json.MarshalIndent(i.Entities, "", "  ")
	//println(string(b))
}
