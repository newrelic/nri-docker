package raw

import (
	"context"
	"io"

	"github.com/moby/moby/api/types/container"
)

// DockerInspector includes `Informer` and a method to inspect a specific container.
type DockerInspector interface {
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
}

// DockerClient defines the required methods to query docker.
type DockerClient interface {
	DockerInspector
	ContainerList(ctx context.Context, all bool) ([]container.Summary, error)
}

// ContainerStatsResponse wraps the response from ContainerStats.
type ContainerStatsResponse struct {
	Body io.ReadCloser
}

// DockerStatsClient defines how to access docker container stats through the docker API.
type DockerStatsClient interface {
	ContainerStats(ctx context.Context, containerID string, stream bool) (ContainerStatsResponse, error)
}
