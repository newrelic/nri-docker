package integration_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/newrelic/nri-docker/src/biz"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	cgroupDriver                              = "systemd"
	InspectorPIDCgroupsV2                     = 667
	relativePathToTestdataFilesystemCgroupsV2 = "testdata/cgroupsV2_host/"
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
			TotalReadCount:  14203,
			TotalWriteCount: 40554,
			TotalReadBytes:  135932,
			TotalWriteBytes: 207296,
		},
		CPU: biz.CPU{
			CPUPercent:    0,
			KernelPercent: 0,
			UserPercent:   0,
			UsedCores:     95.1638064999,
			// This is calculated with this call in biz.metrics
			LimitCores:       float64(runtime.NumCPU()),
			UsedCoresPercent: 4758.190324995,
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
			SwapUsageBytes:        18446744073415594084,
			SwapOnlyUsageBytes:    18446744073274032228,
			SwapLimitBytes:        0,
			SwapLimitUsagePercent: 0,
			SoftLimitBytes:        2,
		},
		RestartCount: 2,
	}

	hostRoot := t.TempDir()

	err := mockedCgroupsV2FileSystem(t, hostRoot)

	storer := inMemoryStorerWithPreviousCPUState()

	inspector := NewInspectorMock(InspectorContainerID, InspectorPIDCgroupsV2, 2)

	cgroupFetcher, err := NewCgroupsV2FetcherMock(hostRoot, mockedTimeForAllMetricsTest, uint64(100000000000))
	require.NoError(t, err)

	t.Run("Given a mockedFilesystem and previous CPU state Then processed metrics are as expected", func(t *testing.T) {
		metrics := biz.NewProcessor(storer, cgroupFetcher, inspector, 0)

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
