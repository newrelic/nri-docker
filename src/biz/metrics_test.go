package biz

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/require"
)

const (
	testDockerClientVersion = "1.24"
	imageTag                = "stress:latest"
	fileName                = "Dockerfile"
	cpus                    = "0.5"
	mem					    = 1e8 // 100 MB of memory
)

func TestAll(t *testing.T) {
	// setup
	docker := newDocker(t)
	defer docker.Close()
	buildTestImage(t, docker)

	t.Run("cpu tests", testCPU)
}

func testCPU(t *testing.T) {
	docker := newDocker(t)

	dockerRM := stress(t, docker, imageTag, strings.Split("-c 2 -t 15s", " "))
	defer dockerRM()

	fmt.Println("Stress command okey makey")
	time.Sleep(15 * time.Second)
}

func newDocker(t *testing.T) *client.Client {
	t.Helper()
	docker, err := client.NewEnvClient()
	require.NoError(t, err)
	docker.UpdateClientVersion(testDockerClientVersion) // TODO: make it configurable
	return docker
}

func buildTestImage(t *testing.T, cli *client.Client) {
	t.Helper()

	cli.ImageRemove(context.Background(), imageTag, types.ImageRemoveOptions{Force:true})

	dockerFile, err := os.Open(fileName)
	require.NoError(t, err)
	defer dockerFile.Close()

	dockerFileContents, err := ioutil.ReadAll(dockerFile)
	require.NoError(t, err)

	tarHeader := &tar.Header{
		Name: fileName,
		Size: int64(len(dockerFileContents)),
	}

	buf := new(bytes.Buffer)
	tarWriter := tar.NewWriter(buf)
	defer tarWriter.Close()

	require.NoError(t, tarWriter.WriteHeader(tarHeader))

	_, err = tarWriter.Write(dockerFileContents)
	require.NoError(t, err)

	dockerFileTarReader := bytes.NewReader(buf.Bytes())

	imageBuildResponse, err := cli.ImageBuild(
		context.Background(),
		dockerFileTarReader,
		types.ImageBuildOptions{
			ForceRemove:true,
			Tags:       []string{imageTag},
			Context:    dockerFileTarReader,
			Remove:     true,
			Dockerfile: fileName,
			CPUSetCPUs: cpus,
			Memory:     mem,
		})
	require.NoError(t, err)
	defer imageBuildResponse.Body.Close()
	io.Copy(os.Stdout, imageBuildResponse.Body)
}

func stress(t *testing.T, cli *client.Client, imageName string, args []string) (closeFunc func()) {
	t.Helper()

	resp, err := cli.ContainerCreate(context.Background(), &container.Config{
		Image: imageName,
		Cmd:   args,
		Tty:   false,
	}, nil, nil, "nri_docker_tests")
	require.NoError(t, err)
	require.NoError(t, cli.ContainerStart(context.Background(), resp.ID, types.ContainerStartOptions{}))
	return func() {
		cli.ContainerRemove(context.Background(), resp.ID, types.ContainerRemoveOptions{Force: true})
	}
}
