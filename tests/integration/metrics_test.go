package biz

import (
	"bytes"
	"fmt"
	"github.com/newrelic/nri-docker/src/biz"
	"github.com/newrelic/nri-docker/test"
	"log"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/client"
	"github.com/newrelic/infra-integrations-sdk/persist"
	"github.com/newrelic/nri-docker/src/raw"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	eventuallyTimeout       = time.Minute
	testDockerClientVersion = "1.24"
	imageTag                = "stress:latest"
	containerName           = "nri_docker_test"
	cpus                    = 0.5
	memLimitStr             = "100M"
	memLimit                = 100 * 1024 * 1024 // 100 MB of memory
)

func TestHighCPU(t *testing.T) {
	// GIVEN a container consuming a lot of CPU
	containerID, dockerRM := stress(t, "stress-ng", "-c", "4", "-l 100", "-t", "5m")
	defer dockerRM()

	// WHEN its metrics are sampled and processed
	docker := newDocker(t)
	defer docker.Close()

	cgroupFetcher, err := raw.NewCgroupsFetcher("/", "cgroupfs", "")
	require.NoError(t, err)

	metrics := biz.NewProcessor(
		persist.NewInMemoryStore(),
		cgroupFetcher,
		docker,
		0)
	sample, err := metrics.Process(containerID)
	require.NoError(t, err)

	// THEN the CPU static metrics belong to the container
	assert.InDelta(t, cpus, sample.CPU.LimitCores, 0.01)
	assert.True(t, sample.Pids.Current >= 4, "pids need to be >= 4") // because we invoked stress-ng -c 4

	test.Eventually(t, eventuallyTimeout, func(t require.TestingT) {
		time.Sleep(100 * time.Millisecond)
		sample, err := metrics.Process(containerID)
		require.NoError(t, err)

		cpu := sample.CPU

		// AND the samples tend to show CPU metrics near to their limits
		assert.InDelta(t, 100*cpus, cpu.CPUPercent, 15)
		assert.InDelta(t, 100, cpu.UsedCoresPercent, 15)
		assert.InDelta(t, cpus, cpu.UsedCores, 0.2)

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
	containerID, dockerRM := stress(t, "stress-ng", "-c", "0", "-l", "0", "-t", "5m")
	defer dockerRM()

	// WHEN its metrics are sampled and processed
	docker := newDocker(t)
	defer docker.Close()

	cgroupFetcher, err := raw.NewCgroupsFetcher("/", "cgroupfs", "")
	require.NoError(t, err)

	metrics := biz.NewProcessor(
		persist.NewInMemoryStore(),
		cgroupFetcher,
		docker,
		0)
	sample, err := metrics.Process(containerID)
	require.NoError(t, err)

	// THEN the CPU static metrics belong to the container
	assert.InDelta(t, cpus, sample.CPU.LimitCores, 0.01)
	assert.True(t, sample.Pids.Current > 0, "pids can't be 0")

	test.Eventually(t, eventuallyTimeout, func(t require.TestingT) {
		time.Sleep(100 * time.Millisecond)
		sample, err := metrics.Process(containerID)
		require.NoError(t, err)

		cpu := sample.CPU

		// AND the samples tend to show CPU metrics near zero
		assert.InDelta(t, 0, cpu.CPUPercent, 10)
		assert.InDelta(t, 0, cpu.UsedCoresPercent, 10)
		assert.InDelta(t, 0, cpu.UsedCores, 0.1)
	})
}

func TestMemory(t *testing.T) {
	// GIVEN a container with a process consuming 60M of its limit of 100M
	containerID, dockerRM := stress(t, "stress-ng", "-c", "0", "-l", "0", "--vm", "1", "--vm-bytes", "60M", "-t", "5m")
	defer dockerRM()

	// WHEN its metrics are sampled and processed
	docker := newDocker(t)
	defer docker.Close()

	cgroupFetcher, err := raw.NewCgroupsFetcher("/", "cgroupfs", "")
	require.NoError(t, err)

	metrics := biz.NewProcessor(
		persist.NewInMemoryStore(),
		cgroupFetcher,
		docker,
		0)
	// Then the Memory metrics are reported according to the usage and limits
	test.Eventually(t, eventuallyTimeout, func(t require.TestingT) {
		sample, err := metrics.Process(containerID)
		require.NoError(t, err)

		mem := sample.Memory
		const expectedUsage = 60 * 1024 * 1024 // 60MB
		assert.Truef(t, mem.UsageBytes >= expectedUsage,
			"reported usage %v should be >= 60MB (%v)", mem.UsageBytes, expectedUsage)
		assert.Truef(t, mem.RSSUsageBytes >= expectedUsage,
			"reported RSS %v should be >= 60MB (%v)", mem.RSSUsageBytes, expectedUsage)
		expectedPercent := float64(expectedUsage) * 100 / memLimit
		assert.Truef(t, mem.UsagePercent >= expectedPercent,
			"reported Usage Percent %v should be >= %v", mem.RSSUsageBytes, expectedPercent)
		// todo: test cachebytes against a fixed value
		assert.True(t, mem.CacheUsageBytes > 0, "reported cache bytes %v should not be zero")

		assert.EqualValues(t, memLimit, mem.MemLimitBytes)
	})
}

func newDocker(t *testing.T) *client.Client {
	t.Helper()
	docker, err := client.NewEnvClient()
	require.NoError(t, err)
	docker.UpdateClientVersion(testDockerClientVersion)
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

func TestExitedContainersWithTTL(t *testing.T) {
	containerID, dockerRM := stress(t, "stress-ng", "-c", "0", "-l", "0", "--vm", "1", "--vm-bytes", "60M", "-t", "1s")
	defer dockerRM()

	docker := newDocker(t)
	defer docker.Close()

	cgroupFetcher, err := raw.NewCgroupsFetcher("/", "cgroupfs", "")
	require.NoError(t, err)

	metrics := biz.NewProcessor(persist.NewInMemoryStore(), cgroupFetcher, docker, 1*time.Second)

	test.Eventually(t, eventuallyTimeout, func(t require.TestingT) {
		samples, err := metrics.Process(containerID)
		require.Error(t, err)
		assert.Empty(t, samples)
	})
}

func TestExitedContainersWithoutTTL(t *testing.T) {
	containerID, dockerRM := stress(t, "stress-ng", "-c", "0", "-l", "0", "--vm", "1", "--vm-bytes", "60M", "-t", "1s")
	defer dockerRM()

	docker := newDocker(t)
	defer docker.Close()

	cgroupFetcher, err := raw.NewCgroupsFetcher("/", "cgroupfs", "")
	require.NoError(t, err)

	metrics := biz.NewProcessor(persist.NewInMemoryStore(), cgroupFetcher, docker, 0)

	test.Eventually(t, eventuallyTimeout, func(t require.TestingT) {
		sample, err := metrics.Process(containerID)
		require.Error(t, err)
		assert.Empty(t, sample)
	})
}
