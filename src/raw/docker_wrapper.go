package raw

import (
	"context"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/client"
)

// DockerClientWrapper wraps a *client.Client and adapts it to the
// DockerClient, DockerStatsClient, and DockerInspector interfaces.
type DockerClientWrapper struct {
	client *client.Client
}

// NewDockerClientWrapper creates a new DockerClientWrapper.
func NewDockerClientWrapper(c *client.Client) *DockerClientWrapper {
	return &DockerClientWrapper{client: c}
}

func (w *DockerClientWrapper) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	result, err := w.client.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return container.InspectResponse{}, err
	}
	return result.Container, nil
}

func (w *DockerClientWrapper) ContainerList(ctx context.Context, all bool) ([]container.Summary, error) {
	result, err := w.client.ContainerList(ctx, client.ContainerListOptions{All: all})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (w *DockerClientWrapper) ContainerStats(ctx context.Context, containerID string, stream bool) (ContainerStatsResponse, error) {
	result, err := w.client.ContainerStats(ctx, containerID, client.ContainerStatsOptions{Stream: stream})
	if err != nil {
		return ContainerStatsResponse{}, err
	}
	return ContainerStatsResponse{Body: result.Body}, nil
}

// Info returns docker daemon info.
func (w *DockerClientWrapper) Info(ctx context.Context) (system.Info, error) {
	result, err := w.client.Info(ctx, client.InfoOptions{})
	if err != nil {
		return system.Info{}, err
	}
	return result.Info, nil
}

// Close closes the underlying client.
func (w *DockerClientWrapper) Close() error {
	return w.client.Close()
}
