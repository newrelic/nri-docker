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
	"github.com/newrelic/infra-integrations-sdk/log"
)

// CgroupsV2Fetcher fetches the metrics that can be found in cgroups (v2) file system
type CgroupsV2Fetcher struct {
	hostRoot string
}

// NewCgroupsV2Fetcher creates a new cgroups data fetcher.
func NewCgroupsV2Fetcher(hostRoot string) (*CgroupsFetcher, error) {
	return &CgroupsFetcher{
		hostRoot: hostRoot,
	}, nil
}

// Fetch get the metrics that can be found in cgroups v2 file system
// Unlike v1, cgroup v2 has only single hierarchy.
//TODO: populate also network from libcgroups
func (cg *CgroupsV2Fetcher) Fetch(c types.ContainerJSON) (Metrics, error) {
	stats := Metrics{}

	pid := c.State.Pid
	containerID := c.ID

	manager, err := cgroupsV2.LoadManager(mountsFilePath, cg.hostRoot)
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

	// TODO: missing metric
	//cgroupFullPathBlkio, err := cgroupInfo.getFullPath(cgroups.Blkio)
	//
	//if err == nil {
	//	if stats.Blkio, err = blkio(cgroupFullPathBlkio); err != nil {
	//		log.Error("couldn't read blkio stats: %v", err)
	//	}
	//} else {
	//	log.Error("couldn't read blkio stats: %v", err)
	//}

	// TODO: missing metric
	//if stats.CPU.Shares, err = cgroupInfo.getSingleFileUintStat(cgroups.Cpu, "cpu.shares"); err != nil {
	//	log.Error("couldn't read cpu shares: %v", err)
	//}

	if stats.Memory, err = cg.memory(metrics); err != nil {
		log.Error("couldn't read memory stats: %v", err)
	}

	stats.ContainerID = containerID

	netMetricsPath := filepath.Join(cg.hostRoot, "/proc", strconv.Itoa(pid), "net", "dev")
	stats.Network, err = network(netMetricsPath)
	if err != nil {
		log.Error(
			"couldn't fetch network stats for container %s from cgroups: %v",
			containerID,
			err,
		)
		return stats, err
	}

	return stats, nil
}

func (cg *CgroupsV2Fetcher) cpu(metric *cgroupstatsV2.Metrics) (CPU, error) {
	if metric.CPU == nil || metric.CPU.UsageUsec == 0 {
		return CPU{}, errors.New("no CPU metrics information")
	}

	cpu := CPU{
		TotalUsage:        metric.CPU.UsageUsec * uint64(time.Microsecond),
		UsageInUsermode:   metric.CPU.UserUsec * uint64(time.Microsecond),
		UsageInKernelmode: metric.CPU.SystemUsec * uint64(time.Microsecond),
		// PercpuUsage:       metric.CPU.Usage.PerCPU, // TODO: missing metric
	}
	if metric.CPU.NrThrottled != 0 {
		cpu.ThrottledPeriods = metric.CPU.NrThrottled
		cpu.ThrottledTimeNS = metric.CPU.ThrottledUsec * uint64(time.Microsecond)
	}

	var err error
	cpu.SystemUsage, err = readSystemCPUUsage()
	cpu.Shares = metric.Memory.Shmem

	return cpu, err
}

func (cg *CgroupsV2Fetcher) memory(metric *cgroupstatsV2.Metrics) (Memory, error) {
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
	mem.SwapUsage = metric.Memory.SwapUsage
	mem.SwapLimit = metric.Memory.SwapLimit
	mem.KernelMemoryUsage = metric.Memory.KernelStack + metric.Memory.Slab
	mem.SoftLimit = metric.MemoryEvents.Low

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
