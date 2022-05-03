package raw

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
)

type dockerClientMock struct {
	infoCallCount int
}

func (c *dockerClientMock) Info(ctx context.Context) (types.Info, error) {
	c.infoCallCount++
	return types.Info{}, nil
}

func (c *dockerClientMock) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	return types.ContainerJSON{}, nil
}

func (c *dockerClientMock) ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error) {
	return nil, nil
}

func TestCachedInfoDockerClient(t *testing.T) {
	client := &dockerClientMock{}
	cached := NewCachedInfoDockerClient(client)
	cached.Info(context.Background())
	assert.Equal(t, 1, client.infoCallCount)
	cached.Info(context.Background())
	assert.Equal(t, 1, client.infoCallCount)
}
