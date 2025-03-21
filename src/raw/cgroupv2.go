//go:build linux

// Package raw fetches raw system-level metrics as they are presented by the operating system
package raw

import (
	"errors"
	"path/filepath"
	"strconv"
	"time"

	cgroupsV2 "github.com/containerd/cgroups/v2"
	cgroupstatsV2 "github.com/containerd/cgroups/v2/stats"
	"github.com/docker/docker/api/types"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
)

// CgroupsV2Fetcher fetches the metrics that can be found in cgroups (v2) file system
type CgroupsV2Fetcher struct {
	cgroupDriver       string
	hostRoot           string
	cgroupDetector     CgroupV2Detector
	systemCPUReader    SystemCPUReader
	networkStatsGetter NetworkStatsGetter
	cpuCounter         func(effectiveCPUsPath string) (uint, error)
}

// NewCgroupsV2Fetcher creates a new cgroups data fetcher.
func NewCgroupsV2Fetcher(
	hostRoot string,
	cgroupDriver string,
	systemCPUReader SystemCPUReader,
) (*CgroupsV2Fetcher, error) {
	return &CgroupsV2Fetcher{
		cgroupDriver:       cgroupDriver,
		hostRoot:           hostRoot,
		cgroupDetector:     NewCgroupV2PathParser(),
		systemCPUReader:    systemCPUReader,
		networkStatsGetter: NewNetDevNetworkStatsGetter(),
		cpuCounter:         countCpusetCPUsFromPath,
	}, nil
}

// Fetch get the metrics that can be found in cgroups v2 file system
// Unlike v1, cgroup v2 has only single hierarchy.
func (cg *CgroupsV2Fetcher) Fetch(containerInfo types.ContainerJSON) (Metrics, error) {
	stats := Metrics{}

	pid := containerInfo.State.Pid
	containerID := containerInfo.ID

	cgroupInfo, err := cg.cgroupDetector.Paths(cg.hostRoot, pid)
	if err != nil {
		return stats, err
	}

	manager, err := cgroupsV2.LoadManager(cgroupInfo.getMountPoint(), cgroupInfo.getGroup())
	if err != nil {
		return stats, err
	}

	metrics, err := manager.Stat()
	if err != nil {
		return stats, err
	}

	stats.Time = time.Now()

	if stats.Pids, err = cg.pids(metrics); err != nil {
		log.Error("couldn't read pids stats: %v", err)
	}

	if stats.CPU, err = cg.cpu(metrics); err != nil {
		log.Error("couldn't read cpu stats: %v", err)
	}

	if containerInfo.HostConfig != nil {
		stats.CPU.Shares = uint64(containerInfo.HostConfig.CPUShares)
	} else {
		log.Debug("CPUShares metric could not be fetched for container %q because host configuration is not available")
	}

	cpusetPath := filepath.Join(cgroupInfo.getFullPath(), "cpuset.cpus.effective")
	if stats.CPU.OnlineCPUs, err = cg.cpuCounter(cpusetPath); err != nil {
		log.Error("couldn't get cpu count: %v", err)
	}

	if stats.Memory, err = cg.memory(metrics, containerInfo); err != nil {
		log.Error("couldn't read memory stats: %v", err)
	}

	if stats.Blkio, err = cg.io(metrics); err != nil {
		log.Error("couldn't read io stats: %v", err)
	}

	stats.ContainerID = containerID
	stats.Network, err = cg.networkStatsGetter.GetForContainer(cg.hostRoot, strconv.Itoa(pid), containerID)

	return stats, err
}

func (cg *CgroupsV2Fetcher) cpu(metric *cgroupstatsV2.Metrics) (CPU, error) {
	if metric.CPU == nil || metric.CPU.UsageUsec == 0 {
		return CPU{}, errors.New("no CPU metrics information")
	}

	userUsage := microsecondsToNanoseconds(metric.CPU.UserUsec)
	kernelUsage := microsecondsToNanoseconds(metric.CPU.SystemUsec)

	cpu := CPU{
		TotalUsage:        microsecondsToNanoseconds(metric.CPU.UsageUsec),
		UsageInUsermode:   &userUsage,
		UsageInKernelmode: &kernelUsage,
	}

	if metric.CPU.NrThrottled != 0 {
		cpu.ThrottledPeriods = metric.CPU.NrThrottled
		cpu.ThrottledTimeNS = microsecondsToNanoseconds(metric.CPU.ThrottledUsec)
	}

	var err error
	cpu.SystemUsage, err = cg.systemCPUReader.ReadUsage()

	return cpu, err
}

func (cg *CgroupsV2Fetcher) memory(metric *cgroupstatsV2.Metrics, containerInfo types.ContainerJSON) (Memory, error) {
	mem := Memory{}
	if metric.Memory == nil {
		return mem, errors.New("no Memory metrics information")
	}
	if metric.Memory.Usage != 0 {
		mem.UsageLimit = metric.Memory.UsageLimit
		mem.FuzzUsage = metric.Memory.Usage
	}

	mem.Cache = metric.Memory.File
	mem.RSS = metric.Memory.Anon
	mem.SwapUsage = &metric.Memory.SwapUsage
	mem.SwapLimit = metric.Memory.SwapLimit
	mem.KernelMemoryUsage = metric.Memory.KernelStack + metric.Memory.Slab

	if containerInfo.HostConfig != nil {
		mem.SoftLimit = uint64(containerInfo.HostConfig.MemoryReservation)
	} else {
		log.Debug("Memory soft limit could not be fetched for container %q because host configuration is not available", containerInfo.ID)
	}

	return mem, nil
}

func (cg *CgroupsV2Fetcher) pids(metrics *cgroupstatsV2.Metrics) (Pids, error) {
	if metrics.Pids == nil {
		return Pids{}, errors.New("no PIDs information")
	}

	return Pids{
		Current: metrics.Pids.Current,
		Limit:   metrics.Pids.Limit,
	}, nil
}

func (cg *CgroupsV2Fetcher) io(metrics *cgroupstatsV2.Metrics) (Blkio, error) {
	stats := Blkio{}
	if metrics.Io == nil {
		return stats, errors.New("no IO information")
	}
	for _, m := range metrics.Io.Usage {
		stats.IoServiceBytesRecursive = append(
			stats.IoServiceBytesRecursive,
			BlkioEntry{Op: blkioReadOp, Value: m.Rbytes},
			BlkioEntry{Op: blkioWriteOp, Value: m.Wbytes},
		)
		stats.IoServicedRecursive = append(
			stats.IoServicedRecursive,
			BlkioEntry{Op: blkioReadOp, Value: m.Rios},
			BlkioEntry{Op: blkioWriteOp, Value: m.Wios},
		)
	}
	return stats, nil
}
