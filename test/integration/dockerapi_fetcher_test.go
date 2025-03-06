package integration

import (
	"context"
	"strconv"
	"testing"

	"github.com/newrelic/nri-docker/src/constants"
	"github.com/newrelic/nri-docker/src/raw/dockerapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDockerAPIFetcher(t *testing.T) {
	dockerClient := newDocker(t)

	info, err := dockerClient.Info(context.Background())
	require.NoError(t, err)
	if info.CgroupVersion != "2" {
		t.Skip("DockerAPIFetcher only supports cgroups v2 version")
	}

	fetcher := dockerapi.NewFetcher(dockerClient, constants.LinuxPlatformName)

	// run a container for testing purposes
	containerID, dockerRM := stress(t, "stress-ng", "-c", "2", "-l", "50", "-t", "5m", "--iomix", "10")
	defer dockerRM()

	inspectData, err := dockerClient.ContainerInspect(context.Background(), containerID)
	require.NoError(t, err)

	assert.EventuallyWithT(t, func(ct *assert.CollectT) {
		statsData, err := fetcher.Fetch(inspectData)
		require.NoError(ct, err)

		// CPU metrics
		assert.NotZero(ct, statsData.CPU.TotalUsage)
		assert.NotZero(ct, statsData.CPU.UsageInUsermode)
		assert.NotZero(ct, statsData.CPU.UsageInKernelmode)
		assert.NotZero(ct, statsData.CPU.SystemUsage)
		assert.NotZero(ct, statsData.CPU.ThrottledPeriods)
		assert.NotZero(ct, statsData.CPU.ThrottledTimeNS)
		assert.NotZero(ct, statsData.CPU.OnlineCPUs)
		assert.EqualValues(ct, cpuShares, statsData.CPU.Shares, "cpu-shares has been set, so it should be reported")

		// Memory metrics
		// SwapUsage cannot be fetched through docker API
		assert.EqualValues(ct, statsData.Memory.UsageLimit, memLimit, "the memory limit has been set, so it should be reported")
		assert.EqualValues(ct, statsData.Memory.SoftLimit, memReservation, "the memory soft limit has been set, so it should be reported")
		assert.NotZero(ct, statsData.Memory.SwapLimit)
		assert.NotZero(ct, statsData.Memory.Cache)
		assert.NotZero(ct, statsData.Memory.RSS)
		assert.NotZero(ct, statsData.Memory.KernelMemoryUsage)

		// Network metrics
		// Only RxBytes and RxPackets are generated
		assert.NotZero(ct, statsData.Network.RxBytes, "stress-ng should have generated RxBytes")
		assert.NotZero(ct, statsData.Network.RxPackets, "stress-ng should have generated RxPackets")

		// Pids metrics
		assert.Equal(ct, pidsLimit, strconv.FormatUint(statsData.Pids.Limit, 10), "the limit has been set so it should be reported")
		assert.NotZero(ct, statsData.Pids.Current, "amount of processes or threads should be grater than 0")

		// Blkio metrics
		// IoServicedRecursive can be empty if the io operations are not blocking
		assert.NotEmpty(ct, statsData.Blkio.IoServiceBytesRecursive)

	}, eventuallyTimeout, eventuallyTick)
}
