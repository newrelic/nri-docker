package raw

import (
	"context"

	"github.com/docker/docker/api/types"
)

// DockerInspector includes `Informer` and a method to inspect a specific container.
type DockerInspector interface {
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
