package integration

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	eventuallyTimeout       = time.Minute
	eventuallyTick          = time.Second
	testDockerClientVersion = "1.24"
	imageTag                = "stress:latest"
	containerName           = "nri_docker_test"
	cpus                    = 0.5
	memLimitStr             = "100M"
	memLimit                = 100 * 1024 * 1024 // 100 MB of memory
)

func newDocker(t *testing.T) *client.Client {
	t.Helper()
	docker, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion(testDockerClientVersion))
	require.NoError(t, err)
	return docker
}

func stress(t *testing.T, args ...string) (containerID string, closeFunc func()) {
	t.Helper()

	arguments := []string{
		"run", "-d",
		"--name", containerName,
		"--cpus", fmt.Sprint(cpus),
		"--memory", memLimitStr,
		imageTag}
	arguments = append(arguments, args...)
	cmd := exec.Command("docker", arguments...)
	stdout := bytes.Buffer{}
	cmd.Stdout = &stdout
	stderr := bytes.Buffer{}
	cmd.Stderr = &stderr
	err := cmd.Run()
	outb, _ := stdout.ReadBytes('\n')
	log.Println(string(outb))
	errb, _ := stderr.ReadBytes(0)
	log.Println(string(errb))
	assert.NoError(t, err)

	return strings.Trim(string(outb), "\n\r"), func() {
		cmd := exec.Command("docker", "rm", "-f", containerName)
		out, err := cmd.CombinedOutput()
		log.Println(string(out))
		if err != nil {
			log.Println("error removing container", err)
		}
	}
}
