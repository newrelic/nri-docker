package raw

import (
	"context"
	"sync"

	"github.com/docker/docker/api/types"
)

// Infomer implements a way to get system-wide information regarding to docker.
type DockerInformer interface {
	Info(ctx context.Context) (types.Info, error)
}

// DockerInspector includes `Informer` and a method to inspect a specific container.
type DockerInspector interface {
	DockerInformer
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
}

// DockerClient defines the required methods to query docker.
type DockerClient interface {
	DockerInspector
	ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error)
}

// DockerStatsClient defines how to access docker container stats through the docker API.
type DockerStatsClient interface {
	ContainerStats(ctx context.Context, containerID string, stream bool) (types.ContainerStats, error)
}

// CachedInfoDockerClient Wraps a DockerClient indefinitely caching Info method first call.
type CachedInfoDockerClient struct {
	DockerClient

	once         sync.Once
	infoResponse types.Info
	infoError    error
}

func (c *CachedInfoDockerClient) Info(ctx context.Context) (types.Info, error) {
	c.once.Do(func() {
		c.infoResponse, c.infoError = c.DockerClient.Info(ctx)
	})
	return c.infoResponse, c.infoError
}

// NewCachedInfoDockerClient returns a client wrapper using the provided one.
func NewCachedInfoDockerClient(c DockerClient) *CachedInfoDockerClient {
	return &CachedInfoDockerClient{DockerClient: c}
}
