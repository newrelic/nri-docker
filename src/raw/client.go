package raw

import (
	"context"

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
