// Package raw fetches raw system-level metrics as they are presented by the operating system
package raw

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/cgroups"
	cgroupsV1 "github.com/containerd/cgroups/stats/v1"
	"github.com/docker/docker/api/types"
	"github.com/newrelic/infra-integrations-sdk/log"
)

// CgroupsFetcher fetches the metrics that can be found in cgroups file system
type CgroupsFetcher struct {
	hostRoot         string
	cgroupDriver     string
	cgroupMountPoint string
}

// NewCgroupsFetcher creates a new cgroups data fetcher.
func NewCgroupsFetcher(hostRoot, cgroupDriver, cgroupMountPoint string) (*CgroupsFetcher, error) {
	return &CgroupsFetcher{
		hostRoot:         hostRoot,
		cgroupDriver:     cgroupDriver,
		cgroupMountPoint: cgroupMountPoint,
	}, nil
}

// Fetch get the metrics that can be found in cgroups file system:
//TODO: populate also network from libcgroups
func (cg *CgroupsFetcher) Fetch(c types.ContainerJSON) (Metrics, error) {
	stats := Metrics{}

	pid := c.State.Pid
	containerID := c.ID

	var (
		cgroupInfo *cgroupPaths
		err        error
	)
	if cg.cgroupMountPoint == "" {
		cgroupInfo, err = getCgroupPaths(cg.hostRoot, pid)
	} else {
		var parent string
		if c.HostConfig == nil || c.HostConfig.CgroupParent == "" {
			parent = "docker"
		} else {
			parent = c.HostConfig.CgroupParent
		}
		cgroupInfo, err = getStaticCgroupPaths(cg.cgroupDriver, filepath.Join(cg.hostRoot, cg.cgroupMountPoint), parent, containerID)
	}

	if err != nil {
		return stats, err
	}

	control, err := cgroups.Load(cgroupInfo.getHierarchyFn(), cgroupInfo.getPath)
	if err != nil {
		return stats, err
	}
	metrics, err := control.Stat(cgroups.IgnoreNotExist)
	if err != nil {
		return stats, err
	}

	stats.Time = time.Now()

	if stats.Pids, err = pids(metrics); err != nil {
		log.Error("couldn't read pids stats: %v", err)
	}

	cgroupFullPathBlkio, err := cgroupInfo.getFullPath(cgroups.Blkio)

	if err == nil {
		if stats.Blkio, err = blkio(cgroupFullPathBlkio); err != nil {
			log.Error("couldn't read blkio stats: %v", err)
		}
	} else {
		log.Error("couldn't read blkio stats: %v", err)
	}

	if stats.CPU, err = cpu(metrics); err != nil {
		log.Error("couldn't read cpu stats: %v", err)
	}

	if stats.CPU.Shares, err = cgroupInfo.getSingleFileUintStat(cgroups.Cpu, "cpu.shares"); err != nil {
		log.Error("couldn't read cpu shares: %v", err)
	}

	if stats.Memory, err = memory(metrics); err != nil {
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

func pids(metrics *cgroupsV1.Metrics) (Pids, error) {
	if metrics.Pids == nil {
		return Pids{}, errors.New("no PIDs information")
	}
	return Pids{
		Current: metrics.Pids.Current,
		Limit:   metrics.Pids.Limit,
	}, nil
}

// at the moment we don't use cgroups library because it doesn't seem to properly
// parse this data when you run the integration inside a container
// TODO: use cgroups library (as for readPidStats)
func blkioEntries(blkioPath string, ioStat string) ([]BlkioEntry, error) {
	entries := make([]BlkioEntry, 0)

	f, err := os.Open(path.Join(blkioPath, ioStat))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.FieldsFunc(sc.Text(), func(r rune) bool {
			return r == ' ' || r == ':'
		})
		if len(fields) < 3 {
			if len(fields) == 2 && fields[0] == "Total" {
				// skip total line
				continue
			} else {
				return nil, fmt.Errorf("invalid line found while parsing %s: %s", blkioPath, sc.Text())
			}
		}

		// major version: ignoring but checking for proper format check
		_, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return nil, err
		}

		// minor version: ignoring but checking for proper format check
		_, err = strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, err
		}

		op := ""
		valueField := 2
		if len(fields) == 4 {
			op = fields[2]
			valueField = 3
		}
		val, err := strconv.ParseUint(fields[valueField], 10, 64)
		if err != nil {
			return nil, err
		}
		entries = append(entries, BlkioEntry{Op: op, Value: val})
	}

	return entries, nil
}

// TODO: use cgroups library (as for readPidStats)
// cgroups library currently don't seem to work for blkio. We can fix it and submit a patch
func blkio(cpath string) (Blkio, error) {

	stats := Blkio{}
	var err error
	stats.IoServiceBytesRecursive, err = blkioEntries(cpath, "blkio.throttle.io_service_bytes")
	if err != nil {
		return stats, err
	}

	stats.IoServicedRecursive, err = blkioEntries(cpath, "blkio.throttle.io_serviced")
	return stats, err
}

const nanoSecondsPerSecond = 1e9

// readSystemCPUUsage returns the host system's cpu usage in
// nanoseconds. An error is returned if the format of the underlying
// file does not match.
//
// Uses /proc/stat defined by POSIX. Looks for the cpu
// statistics line and then sums up the first seven fields
// provided. See `man 5 proc` for details on specific field
// information.
func readSystemCPUUsage() (uint64, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}

	bufReader := bufio.NewReaderSize(nil, 128)
	defer func() {
		bufReader.Reset(nil)
		f.Close()
	}()
	bufReader.Reset(f)

	for {
		line, err := bufReader.ReadString('\n')
		if err != nil {
			break
		}
		parts := strings.Fields(line)
		switch parts[0] {
		case "cpu":
			if len(parts) < 8 {
				return 0, fmt.Errorf("invalid number of cpu fields")
			}
			var totalClockTicks uint64
			for _, i := range parts[1:8] {
				v, err := strconv.ParseUint(i, 10, 64)
				if err != nil {
					return 0, fmt.Errorf("Unable to convert value %s to int: %s", i, err)
				}
				totalClockTicks += v
			}
			return (totalClockTicks * nanoSecondsPerSecond) / 100, nil
		}
	}
	return 0, fmt.Errorf("invalid stat format. Error trying to parse the '/proc/stat' file")
}

func cpu(metric *cgroupsV1.Metrics) (CPU, error) {
	if metric.CPU == nil || metric.CPU.Usage == nil {
		return CPU{}, errors.New("no CPU metrics information")
	}

	cpu := CPU{
		TotalUsage:        metric.CPU.Usage.Total,
		UsageInUsermode:   metric.CPU.Usage.User,
		UsageInKernelmode: metric.CPU.Usage.Kernel,
		PercpuUsage:       metric.CPU.Usage.PerCPU,
	}
	if metric.CPU.Throttling != nil {
		cpu.ThrottledPeriods = metric.CPU.Throttling.ThrottledPeriods
		cpu.ThrottledTimeNS = metric.CPU.Throttling.ThrottledTime
	}

	var err error
	cpu.SystemUsage, err = readSystemCPUUsage()
	return cpu, err
}

func memory(metric *cgroupsV1.Metrics) (Memory, error) {
	mem := Memory{}
	if metric.Memory == nil {
		return mem, errors.New("no Memory metrics information")
	}
	if metric.Memory.Usage != nil {
		mem.UsageLimit = metric.Memory.Usage.Limit
		mem.FuzzUsage = metric.Memory.Usage.Usage
	}
	mem.Cache = metric.Memory.Cache
	mem.RSS = metric.Memory.RSS
	mem.SwapUsage = metric.Memory.Swap.Usage

	return mem, nil
}
