package integration_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/newrelic/nri-docker/src/biz"
	"github.com/newrelic/nri-docker/test"

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
	containerID, dockerRM := stress(t, "stress-ng", "-c", "4", "-l", "100", "-t", "5m")
	defer dockerRM()

	// WHEN its metrics are sampled and processed
	docker := newDocker(t)
	defer docker.Close()

	cgroupFetcher, err := raw.NewCgroupsV1Fetcher(
		"/",
		raw.NewCgroupV1PathParser(),
		raw.NewPosixSystemCPUReader(),
		raw.NewNetDevNetworkStatsGetter(),
	)
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

	cgroupFetcher, err := raw.NewCgroupsV1Fetcher(
		"/",
		raw.NewCgroupV1PathParser(),
		raw.NewPosixSystemCPUReader(),
		raw.NewNetDevNetworkStatsGetter(),
	)
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

	cgroupFetcher, err := raw.NewCgroupsV1Fetcher(
		"/",
		raw.NewCgroupV1PathParser(),
		raw.NewPosixSystemCPUReader(),
		raw.NewNetDevNetworkStatsGetter(),
	)
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

func TestExitedContainersWithTTL(t *testing.T) {
	containerID, dockerRM := stress(t, "stress-ng", "-c", "0", "-l", "0", "--vm", "1", "--vm-bytes", "60M", "-t", "1s")
	defer dockerRM()

	docker := newDocker(t)
	defer docker.Close()

	cgroupFetcher, err := raw.NewCgroupsV1Fetcher(
		"/",
		raw.NewCgroupV1PathParser(),
		raw.NewPosixSystemCPUReader(),
		raw.NewNetDevNetworkStatsGetter(),
	)
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

	cgroupFetcher, err := raw.NewCgroupsV1Fetcher(
		"/",
		raw.NewCgroupV1PathParser(),
		raw.NewPosixSystemCPUReader(),
		raw.NewNetDevNetworkStatsGetter(),
	)
	require.NoError(t, err)

	metrics := biz.NewProcessor(persist.NewInMemoryStore(), cgroupFetcher, docker, 0)

	test.Eventually(t, eventuallyTimeout, func(t require.TestingT) {
		sample, err := metrics.Process(containerID)
		require.Error(t, err)
		assert.Empty(t, sample)
	})
}

const (
	relativePathToTestdataFileystem = "testdata/cgroupsV1_host/"
	InspectorContainerID            = "my-container"
	InspectorPID                    = 666
)

var mockedTimeForAllMetricsTest = time.Date(2022, time.January, 1, 4, 3, 2, 0, time.UTC)

func TestAllMetricsPresent(t *testing.T) {
	expectedSample := biz.Sample{
		Pids: biz.Pids{
			Current: InspectorPID,
			Limit:   8000,
		},
		Network: biz.Network{
			RxBytes:   3402,
			RxDropped: 2,
			RxErrors:  10,
			RxPackets: 28,
			TxBytes:   5,
			TxDropped: 2,
			TxErrors:  3,
			TxPackets: 4,
		},
		BlkIO: biz.BlkIO{
			TotalReadCount:  39,
			TotalWriteCount: 89,
			TotalReadBytes:  2387968,
			TotalWriteBytes: 50,
		},
		CPU: biz.CPU{
			CPUPercent:    28.546433726285464,
			KernelPercent: 1.19999999,
			UserPercent:   100,
			UsedCores:     2.8546433726,
			// This is calculated with this call in biz.metrics
			LimitCores:       float64(runtime.NumCPU()),
			UsedCoresPercent: float64(100) * 2.8546433726 / float64(runtime.NumCPU()),
			ThrottlePeriods:  2384,
			ThrottledTimeMS:  96.578349164,
			Shares:           1024,
		},
		Memory: biz.Memory{
			UsageBytes:            11620352,
			CacheUsageBytes:       2310144,
			RSSUsageBytes:         11620352,
			MemLimitBytes:         104857600,
			UsagePercent:          11.08203125,
			KernelUsageBytes:      724992,
			SwapUsageBytes:        1620352,
			SwapOnlyUsageBytes:    18446744073699551616,
			SwapLimitBytes:        209715200,
			SwapLimitUsagePercent: 0.77264404296875,
			SoftLimitBytes:        262144000,
		},
		RestartCount: 2,
	}

	// Create a tempDir that will be the root of our mocked filsystem
	hostRoot := t.TempDir()

	// Create all the mocked fileSystem for the test
	err := mockedFileSystem(t, hostRoot)

	// CgroupsFetcherMock is the raw CgroupsFetcher with mocked cpu.systemUsage and time
	// The hostRoot is our mocked filesystem
	cgroupFetcher, err := NewCgroupsFetcherMock(hostRoot, mockedTimeForAllMetricsTest, uint64(100000000000))
	require.NoError(t, err)

	storer := inMemoryStorerWithPreviousCPUState()
	inspector := NewInspectorMock(InspectorContainerID, InspectorPID, 2)

	t.Run("Given a mockedFilesystem and previous CPU state Then processed metrics are as expected", func(t *testing.T) {
		metrics := biz.NewProcessor(storer, cgroupFetcher, inspector, 0)

		sample, err := metrics.Process(InspectorContainerID)
		require.NoError(t, err)
		assert.Equal(t, expectedSample, sample)
	})
}

func mockedFileSystem(t *testing.T, hostRoot string) error {
	// Create our mocked container cgroups filesystem in the tempDir directory
	// that will be a symLink to our cgroups testdata
	cgroupsFolder := filepath.Join(hostRoot, "cgroup")
	mockFilesystem, err := filepath.Abs(filepath.Join(relativePathToTestdataFileystem, "cgroup"))
	require.NoError(t, err)

	err = os.Symlink(mockFilesystem, cgroupsFolder)
	require.NoError(t, err)

	// Create mocked directory proc
	err = os.Mkdir(filepath.Join(hostRoot, "proc"), 0755)
	require.NoError(t, err)

	err = mockedProcMountsFile(cgroupsFolder, hostRoot)
	require.NoError(t, err)

	err = mockedProcPIDCGroupFile(hostRoot)
	require.NoError(t, err)

	err = mockedProcNetDevFile(hostRoot)
	require.NoError(t, err)
	return err
}

// inMemoryStorerWithPreviousCPUState creates a storere with a previous CPU state
// in order to make the processor calculate a Cpu Delta
func inMemoryStorerWithPreviousCPUState() persist.Storer {
	var previous struct {
		Time int64
		CPU  raw.CPU
	}

	storer := persist.NewInMemoryStore()
	// We set the time as 10 seconds before the timestamp for the metrics
	previous.Time = mockedTimeForAllMetricsTest.Add(-time.Second * 10).Unix()
	previous.CPU = raw.CPU{
		TotalUsage:        1,
		UsageInUsermode:   1,
		UsageInKernelmode: 1,
		PercpuUsage:       nil,
		ThrottledPeriods:  1,
		ThrottledTimeNS:   1,
		SystemUsage:       1,
		OnlineCPUs:        1,
		Shares:            1,
	}
	storer.Set(InspectorContainerID, previous)
	return storer
}

func mockedProcMountsFile(cgroupsFolder, hostRoot string) error {
	mountsContent := `cgroup <HOST_ROOT>/blkio cgroup rw,nosuid,nodev,noexec,relatime,blkio 0 0
cgroup <HOST_ROOT>/pids cgroup rw,nosuid,nodev,noexec,relatime,pids 0 0
cgroup <HOST_ROOT>/cpu,cpuacct cgroup rw,nosuid,nodev,noexec,relatime,cpu,cpuacct 0 0
cgroup <HOST_ROOT>/cpuset cgroup rw,nosuid,nodev,noexec,relatime,cpuset 0 0
cgroup <HOST_ROOT>/memory cgroup rw,nosuid,nodev,noexec,relatime,memory 0 0
`
	mountsContent = strings.ReplaceAll(mountsContent, "<HOST_ROOT>", cgroupsFolder)
	return os.WriteFile(filepath.Join(hostRoot, "proc", "mounts"), []byte(mountsContent), 0755)
}

func mockedProcPIDCGroupFile(hostRoot string) error {
	err := os.Mkdir(filepath.Join(hostRoot, "proc", strconv.Itoa(InspectorPID)), 0755)
	if err != nil {
		return err
	}

	inputCgroups, err := ioutil.ReadFile(filepath.Join(relativePathToTestdataFileystem, "my-container", "cgroup"))
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(hostRoot, "proc", strconv.Itoa(InspectorPID), "cgroup"), inputCgroups, 0755)
}

func mockedProcNetDevFile(hostRoot string) error {
	err := os.Mkdir(filepath.Join(hostRoot, "proc", strconv.Itoa(InspectorPID), "net"), 0755)
	if err != nil {
		return err
	}

	inputNetDev, err := ioutil.ReadFile(filepath.Join(relativePathToTestdataFileystem, "my-container", "dev"))
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(hostRoot, "proc", strconv.Itoa(InspectorPID), "net", "dev"), inputNetDev, 0755)
}
