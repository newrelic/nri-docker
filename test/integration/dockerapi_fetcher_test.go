package integration_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/docker/docker/client"
	"github.com/newrelic/nri-docker/src/raw/dockerapi"
	"github.com/stretchr/testify/require"
)

// TODO: this should extended/replaced when the fetcher actually fetches something.
// It currently it shows how to get container stats data and can be useful for development purposes.
func TestDockerAPIHelpers(t *testing.T) {
	// Build the client and the fetcher
	// The API Version can be set up using `args.DockerClientVersion` (defaults to 1.24 for now)
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.24"))
	require.NoError(t, err)
	f := dockerapi.NewFetcher(dockerClient)

	// run a container for testing purposes
	containerID, dockerRM := stress(t, "stress-ng", "-c", "0", "-l", "0", "-t", "5m")
	defer dockerRM()

	// show container inspect data (it's done in biz/metrics)
	inspectData, err := dockerClient.ContainerInspect(context.Background(), containerID)
	require.NoError(t, err)
	logAsJSON(t, "Inspect data", &inspectData)

	// fetch stats data
	statsData, err := f.ContainerStats(context.Background(), containerID)
	require.NoError(t, err)
	logAsJSON(t, "Container Stats", &statsData)
}

func logAsJSON(t *testing.T, title string, data any) {
	b, err := json.Marshal(data)
	require.NoError(t, err)
	t.Logf("%s: %s", title, string(b))
}
