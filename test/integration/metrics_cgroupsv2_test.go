package integration

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/newrelic/nri-docker/src/biz"
	"github.com/newrelic/nri-docker/src/raw"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	InspectorPIDCgroupsV2                     = 667
	relativePathToTestdataFilesystemCgroupsV2 = "testdata/cgroupsV2_host/"
	memoryReservationValue                    = 104857600
)

func TestCgroupsv2AllMetricsPresent(t *testing.T) {
	expectedSample := biz.Sample{
		Pids: biz.Pids{
			Current: InspectorPIDCgroupsV2,
			Limit:   8000,
		},
		Network: biz.Network{
			RxBytes:   1086,
			RxDropped: 1,
			RxErrors:  2,
			RxPackets: 13,
			TxBytes:   5,
			TxDropped: 1,
			TxErrors:  2,
			TxPackets: 3,
		},
		BlkIO: biz.BlkIO{
			TotalReadCount:  float64ToPointer(14203),
			TotalWriteCount: float64ToPointer(40554),
			TotalReadBytes:  float64ToPointer(135932),
			TotalWriteBytes: float64ToPointer(207296),
		},
		CPU: biz.CPU{
			CPUPercent:       191.3611027027027,
			KernelPercent:    21,
			UserPercent:      353.77101,
			UsedCores:        3.5401804,
			LimitCores:       2,
			UsedCoresPercent: 177.00902,
			ThrottlePeriods:  0,
			ThrottledTimeMS:  0,
			Shares:           100,
		},
		Memory: biz.Memory{
			UsageBytes:            141561856,
			CacheUsageBytes:       210780160,
			RSSUsageBytes:         141561856,
			MemLimitBytes:         0,
			UsagePercent:          0,
			KernelUsageBytes:      20634808,
			SwapUsageBytes:        uint64ToPointer(18446744073415594084),
			SwapOnlyUsageBytes:    uint64ToPointer(18446744073274032228),
			SwapLimitBytes:        0,
			SwapLimitUsagePercent: float64ToPointer(0),
			SoftLimitBytes:        104857600,
		},
		RestartCount: 2,
	}

	hostRoot := t.TempDir()

	err := mockedCgroupsV2FileSystem(t, hostRoot)
	require.NoError(t, err)

	previousCPUState := raw.CPU{
		TotalUsage:        916236261000,
		UsageInUsermode:   726716405000,
		UsageInKernelmode: 187444559000,
		PercpuUsage:       nil,
		ThrottledPeriods:  1,
		ThrottledTimeNS:   1,
		SystemUsage:       1e9, // seconds in ns
		OnlineCPUs:        1,
		Shares:            1,
	}
	storer := inMemoryStorerWithCustomPreviousCPUState(previousCPUState)

	hostConfig := &container.HostConfig{}
	hostConfig.MemoryReservation = memoryReservationValue
	inspector := NewInspectorMock(InspectorContainerID, InspectorPIDCgroupsV2, 2, hostConfig)

	currentSystemUsage := 75 * 1e9 // seconds in ns
	cgroupFetcher, err := NewCgroupsV2FetcherMock(hostRoot, mockedTimeForAllMetricsTest, uint64(currentSystemUsage))
	require.NoError(t, err)

	t.Run("Given a mockedFilesystem and previous CPU state Then processed metrics are as expected", func(t *testing.T) {
		metrics := biz.NewProcessor(storer, cgroupFetcher, inspector, 0)
		metrics.WithRuntimeNumCPUfunc(func() int { return 2 }) // Mocked cgroups are extracted from a 2 CPU machine.

		sample, err := metrics.Process(InspectorContainerID)
		require.NoError(t, err)

		assert.Equal(t, expectedSample, sample)
	})
}

func mockedCgroupsV2FileSystem(t *testing.T, hostRoot string) error {
	t.Helper()

	// Create our mocked container cgroups filesystem in the tempDir directory
	// that will be a symLink to our cgroups testdata
	cgroupsFolder := filepath.Join(hostRoot, "cgroup")
	mockFilesystem, err := filepath.Abs(filepath.Join(relativePathToTestdataFilesystemCgroupsV2, "cgroup"))
	require.NoError(t, err)

	err = os.Symlink(mockFilesystem, cgroupsFolder)
	require.NoError(t, err)

	// Create mocked directory proc
	err = os.Mkdir(filepath.Join(hostRoot, "proc"), 0755)
	require.NoError(t, err)

	err = mockedCgroupsV2ProcMountsFile(cgroupsFolder, hostRoot)
	require.NoError(t, err)

	err = mockedCgroupsV2ProcPIDCGroupFile(hostRoot)
	require.NoError(t, err)

	err = mockedCgroupsV2ProcNetDevFile(hostRoot)
	require.NoError(t, err)
	return err
}

func mockedCgroupsV2ProcMountsFile(cgroupsFolder, hostRoot string) error {
	mountsContent := `cgroup2 <HOST_ROOT> cgroup2 rw,nosuid,nodev,noexec,relatime,pids 0 0
`
	mountsContent = strings.ReplaceAll(mountsContent, "<HOST_ROOT>", cgroupsFolder)
	return os.WriteFile(filepath.Join(hostRoot, "proc", "mounts"), []byte(mountsContent), 0755)
}

func mockedCgroupsV2ProcPIDCGroupFile(hostRoot string) error {
	err := os.Mkdir(filepath.Join(hostRoot, "proc", strconv.Itoa(InspectorPIDCgroupsV2)), 0755)
	if err != nil {
		return err
	}

	inputCgroups, err := ioutil.ReadFile(filepath.Join(relativePathToTestdataFilesystemCgroupsV2, "my-container", "cgroup"))
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(hostRoot, "proc", strconv.Itoa(InspectorPIDCgroupsV2), "cgroup"), inputCgroups, 0755)
}

func mockedCgroupsV2ProcNetDevFile(hostRoot string) error {
	err := os.Mkdir(filepath.Join(hostRoot, "proc", strconv.Itoa(InspectorPIDCgroupsV2), "net"), 0755)
	if err != nil {
		return err
	}

	inputNetDev, err := ioutil.ReadFile(filepath.Join(relativePathToTestdataFilesystemCgroupsV2, "my-container", "dev"))
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(hostRoot, "proc", strconv.Itoa(InspectorPIDCgroupsV2), "net", "dev"), inputNetDev, 0755)
}
