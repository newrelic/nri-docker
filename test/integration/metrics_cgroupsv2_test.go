//go:build linux

package integration

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/newrelic/nri-docker/src/biz"
	"github.com/newrelic/nri-docker/src/raw"
	"github.com/newrelic/nri-docker/src/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	InspectorPIDCgroupsV2                     = 667
	relativePathToTestdataFilesystemCgroupsV2 = "testdata/cgroupsV2_host/"
	memoryReservationValue                    = 104857600
	// cpuSharesValue is passed as HostConfig.CPUShares. On cgroup v2 the fetcher
	// reads cpu.weight from the cgroup file instead, so this value does NOT appear
	// in the resulting sample.Shares — it is kept only to exercise the fallback path.
	cpuSharesValue = 2048
	// cpuWeightTestValue is the value written to cpu.weight in the test data
	// (testdata/cgroupsV2_host/.../cpu.weight = 79). Converted to v1 shares:
	// (79-1)*262142/9999 + 2 = 2046.
	cpuWeightTestValue          = 79
	cpuWeightTestValueAsShares  = 2046
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
			// Shares is converted from cpu.weight=79 in the test data, NOT from HostConfig.CPUShares.
			Shares: cpuWeightTestValueAsShares,
		},
		Memory: biz.Memory{
			UsageBytes:            141561856,
			CacheUsageBytes:       210780160,
			RSSUsageBytes:         141561856,
			MemLimitBytes:         0,
			UsagePercent:          0,
			KernelUsageBytes:      20634808,
			SwapUsageBytes:        uint64ToPointer(1141561856),
			SwapOnlyUsageBytes:    uint64ToPointer(1000000000),
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
		UsageInUsermode:   utils.ToPointer(uint64(726716405000)),
		UsageInKernelmode: utils.ToPointer(uint64(187444559000)),
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
	hostConfig.CPUShares = cpuSharesValue
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

	inputCgroups, err := os.ReadFile(filepath.Join(relativePathToTestdataFilesystemCgroupsV2, "my-container", "cgroup"))
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(hostRoot, "proc", strconv.Itoa(InspectorPIDCgroupsV2), "cgroup"), inputCgroups, 0755)
}

func mockedCgroupsV2ProcNetDevFile(hostRoot string) error {
	err := os.Mkdir(filepath.Join(hostRoot, "proc", strconv.Itoa(InspectorPIDCgroupsV2), "net"), 0755)
	if err != nil {
		return err
	}

	inputNetDev, err := os.ReadFile(filepath.Join(relativePathToTestdataFilesystemCgroupsV2, "my-container", "dev"))
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(hostRoot, "proc", strconv.Itoa(InspectorPIDCgroupsV2), "net", "dev"), inputNetDev, 0755)
}

// TestCgroupsV2SharesReadFromCPUWeightFile verifies that the fetcher reads cpuShares
// from the cgroup v2 cpu.weight file rather than from Docker's HostConfig.CPUShares.
// This is the regression test for NR-555188: on Amazon Linux 2023 (cgroup v2) the
// Docker API was returning a stale/incorrectly round-tripped CPUShares value, while
// the kernel's cpu.weight file always holds the authoritative allocation.
func TestCgroupsV2SharesReadFromCPUWeightFile(t *testing.T) {
	hostRoot := t.TempDir()

	err := mockedCgroupsV2FileSystem(t, hostRoot)
	require.NoError(t, err)

	// Set HostConfig.CPUShares to a value that is deliberately different from what
	// cpu.weight=79 would convert to (2046), so the test can confirm the source.
	hostConfig := &container.HostConfig{}
	hostConfig.MemoryReservation = memoryReservationValue
	hostConfig.CPUShares = 9999 // wrong value — must NOT appear in the result
	inspector := NewInspectorMock(InspectorContainerID, InspectorPIDCgroupsV2, 2, hostConfig)

	cgroupFetcher, err := NewCgroupsV2FetcherMock(hostRoot, mockedTimeForAllMetricsTest, 75*1e9)
	require.NoError(t, err)

	inspectResp, err := inspector.ContainerInspect(nil, InspectorContainerID)
	require.NoError(t, err)

	metrics, err := cgroupFetcher.Fetch(inspectResp)
	require.NoError(t, err)

	assert.Equal(t, uint64(cpuWeightTestValueAsShares), metrics.CPU.Shares,
		"Shares should be converted from cpu.weight=%d in the cgroup file", cpuWeightTestValue)
	assert.NotEqual(t, uint64(hostConfig.CPUShares), metrics.CPU.Shares,
		"Shares must NOT come from HostConfig.CPUShares")
}

// TestCgroupsV2SharesFallbackToHostConfigWhenCPUWeightMissing verifies that when
// cpu.weight cannot be read, the fetcher falls back to HostConfig.CPUShares so that
// no metric is silently lost in environments where the file may be absent.
func TestCgroupsV2SharesFallbackToHostConfigWhenCPUWeightMissing(t *testing.T) {
	hostRoot := t.TempDir()

	// Build a cgroup v2 filesystem from the test data but omit the cpu.weight file.
	err := mockedCgroupsV2FileSystemWithoutCPUWeight(t, hostRoot)
	require.NoError(t, err)

	const fallbackShares = int64(2048)
	hostConfig := &container.HostConfig{}
	hostConfig.MemoryReservation = memoryReservationValue
	hostConfig.CPUShares = fallbackShares
	inspector := NewInspectorMock(InspectorContainerID, InspectorPIDCgroupsV2, 2, hostConfig)

	cgroupFetcher, err := NewCgroupsV2FetcherMock(hostRoot, mockedTimeForAllMetricsTest, 75*1e9)
	require.NoError(t, err)

	inspectResp, err := inspector.ContainerInspect(nil, InspectorContainerID)
	require.NoError(t, err)

	metrics, err := cgroupFetcher.Fetch(inspectResp)
	require.NoError(t, err)

	assert.Equal(t, uint64(fallbackShares), metrics.CPU.Shares,
		"Shares should fall back to HostConfig.CPUShares when cpu.weight is unreadable")
}

// mockedCgroupsV2FileSystemWithoutCPUWeight sets up the same mocked filesystem as
// mockedCgroupsV2FileSystem but copies the cgroup directory to a writable temp dir
// so that the cpu.weight file can be removed.
func mockedCgroupsV2FileSystemWithoutCPUWeight(t *testing.T, hostRoot string) error {
	t.Helper()

	// Copy test data into a new writable temp dir (we cannot modify the source symlink target).
	cgroupSrc, err := filepath.Abs(filepath.Join(relativePathToTestdataFilesystemCgroupsV2, "cgroup"))
	require.NoError(t, err)

	cgroupsDst := filepath.Join(hostRoot, "cgroup")
	require.NoError(t, copyDir(cgroupSrc, cgroupsDst))

	// Remove cpu.weight to trigger the fallback path.
	cpuWeightPath := filepath.Join(cgroupsDst, "system.slice", "containerd.service", "cpu.weight")
	if err := os.Remove(cpuWeightPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	require.NoError(t, os.Mkdir(filepath.Join(hostRoot, "proc"), 0755))
	require.NoError(t, mockedCgroupsV2ProcMountsFile(cgroupsDst, hostRoot))
	require.NoError(t, mockedCgroupsV2ProcPIDCGroupFile(hostRoot))
	return mockedCgroupsV2ProcNetDevFile(hostRoot)
}

// copyDir recursively copies src into dst.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
