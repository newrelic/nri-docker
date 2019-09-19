package biz

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/newrelic/infra-integrations-sdk/persist"
	"github.com/newrelic/nri-docker/src/raw"
	"github.com/newrelic/nri-docker/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDockerClientVersion = "1.24"
	imageTag                = "stress:latest"
	containerName           = "nri_docker_test"
	cpus                    = 0.5
	mem                     = 1e8 // 100 MB of memory
)

func TestHighCPU(t *testing.T) {
	// GIVEN a container consuming a lot of CPU
	containerID, dockerRM := stress(t, "-c", "2", "-l 100", "-t", "5m")
	defer dockerRM()

	// WHEN its metrics are sampled and processed
	docker := newDocker(t)
	defer docker.Close()
	metrics := NewProcessor(
		persist.NewInMemoryStore(),
		raw.NewFetcher("/"),
		docker)
	sample, err := metrics.Process(containerID)
	require.NoError(t, err)

	// THEN the CPU static metrics belong to the container
	assert.InDelta(t, cpus, sample.CPU.LimitCores, 0.01)
	assert.True(t, sample.Pids.Current > 0, "pids can't be 0")

	test.Eventually(t, 15*time.Second, func(t require.TestingT) {
		time.Sleep(100 * time.Millisecond)
		sample, err := metrics.Process(containerID)
		require.NoError(t, err)

		cpu := sample.CPU

		// AND the samples tend to show CPU metrics near to their limits
		assert.InDelta(t, 100*cpus, cpu.CPUPercent, 10)
		assert.InDelta(t, 100, cpu.UsedCoresPercent, 10)
		assert.InDelta(t, cpus, cpu.UsedCores, 0.1)

		assert.True(t, cpu.UserPercent > 0,
			"user percent not > 0")
		assert.True(t, cpu.KernelPercent >= 0,
			"kernel percent not >= 0")

		assert.Truef(t, cpu.UserPercent+cpu.KernelPercent <= cpu.CPUPercent,
			"user %v%% + kernel %v%% is not < total %v%%",
			cpu.UserPercent, cpu.KernelPercent, cpu.CPUPercent)
	})
}

func TestLowCPU(t *testing.T) {
	// GIVEN a container consuming almost no CPU
	containerID, dockerRM := stress(t, "-c", "0", "-l", "0")
	defer dockerRM()

	// WHEN its metrics are sampled and processed
	docker := newDocker(t)
	defer docker.Close()
	metrics := NewProcessor(
		persist.NewInMemoryStore(),
		raw.NewFetcher("/"),
		docker)
	sample, err := metrics.Process(containerID)
	require.NoError(t, err)

	// THEN the CPU static metrics belong to the container
	assert.InDelta(t, cpus, sample.CPU.LimitCores, 0.01)
	assert.True(t, sample.Pids.Current > 0, "pids can't be 0")

	test.Eventually(t, 15*time.Second, func(t require.TestingT) {
		time.Sleep(100 * time.Millisecond)
		sample, err := metrics.Process(containerID)
		require.NoError(t, err)

		cpu := sample.CPU

		// AND the samples tend to show CPU metrics near zero
		assert.InDelta(t, 0, cpu.CPUPercent, 5)
		assert.InDelta(t, 0, cpu.UsedCoresPercent, 5)
		assert.InDelta(t, 0, cpu.UsedCores, 0.1)
	})
}

func newDocker(t *testing.T) *client.Client {
	t.Helper()
	docker, err := client.NewEnvClient()
	require.NoError(t, err)
	docker.UpdateClientVersion(testDockerClientVersion) // TODO: make it configurable
	return docker
}

func buildTestImage(t *testing.T) {
	t.Helper()

	cmd := exec.Command("docker", "build", "-t", imageTag, ".")
	require.NoError(t, cmd.Run())
	log.Println(cmd.CombinedOutput())
}

func stress(t *testing.T, args ...string) (containerID string, closeFunc func()) {
	t.Helper()

	arguments := []string{"run", "-d", "--name", containerName, "--rm", "--cpus", fmt.Sprint(cpus), imageTag}
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
