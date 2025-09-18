package raw

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

// DockerInspector includes `Informer` and a method to inspect a specific container.
type DockerInspector interface {
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
}

// DockerClient defines the required methods to query docker.
type DockerClient interface {
	DockerInspector
	ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error)
}

// DockerStatsClient defines how to access docker container stats through the docker API.
type DockerStatsClient interface {
	ContainerStats(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error)
}
