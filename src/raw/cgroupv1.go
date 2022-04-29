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
	cgroupstats "github.com/containerd/cgroups/stats/v1"
	"github.com/docker/docker/api/types"
	"github.com/newrelic/infra-integrations-sdk/log"
)

const nanoSecondsPerSecond = 1e9

// CgroupsV1Fetcher fetches the metrics that can be found in cgroups (v1) file system
type CgroupsV1Fetcher struct {
	hostRoot string
}

func NewCgroupsV1Fetcher(hostrRoot string) (*CgroupsV1Fetcher, error) {
	return &CgroupsV1Fetcher{hostRoot: hostrRoot}, nil
}

// Fetch get the metrics that can be found in cgroups file system:
//TODO: populate also network from libcgroups
func (cg *CgroupsV1Fetcher) Fetch(c types.ContainerJSON) (Metrics, error) {
	stats := Metrics{}

	pid := c.State.Pid
	containerID := c.ID

	var (
		cgroupInfo *cgroupV1Paths
		err        error
	)
	cgroupInfo, err = getCgroupV1Paths(cg.hostRoot, pid)

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

	if stats.Pids, err = cg.pids(metrics); err != nil {
		log.Error("couldn't read pids stats: %v", err)
	}

	cgroupFullPathBlkio, err := cgroupInfo.getFullPath(cgroups.Blkio)

	if err == nil {
		if stats.Blkio, err = cg.blkio(cgroupFullPathBlkio); err != nil {
			log.Error("couldn't read blkio stats: %v", err)
		}
	} else {
		log.Error("couldn't read blkio stats: %v", err)
	}

	if stats.CPU, err = cg.cpu(metrics); err != nil {
		log.Error("couldn't read cpu stats: %v", err)
	}

	if stats.CPU.Shares, err = cgroupInfo.getSingleFileUintStat(cgroups.Cpu, "cpu.shares"); err != nil {
		log.Error("couldn't read cpu shares: %v", err)
	}

	if stats.Memory, err = cg.memory(metrics); err != nil {
		log.Error("couldn't read memory stats: %v", err)
	}

	if stats.Memory.SoftLimit, err = cgroupInfo.getSingleFileUintStat(cgroups.Memory, "memory.soft_limit_in_bytes"); err != nil {
		log.Debug("couldn't read soft_limit_in_bytes stats: %v", err)
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

func (cg *CgroupsV1Fetcher) pids(metrics *cgroupstats.Metrics) (Pids, error) {
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
func (cg *CgroupsV1Fetcher) blkioEntries(blkioPath string, ioStat string) ([]BlkioEntry, error) {
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
func (cg *CgroupsV1Fetcher) blkio(cpath string) (Blkio, error) {

	stats := Blkio{}
	var err error
	stats.IoServiceBytesRecursive, err = cg.blkioEntries(cpath, "blkio.throttle.io_service_bytes")
	if err != nil {
		return stats, err
	}

	stats.IoServicedRecursive, err = cg.blkioEntries(cpath, "blkio.throttle.io_serviced")
	return stats, err
}

// readSystemCPUUsage returns the host system's cpu usage in
// nanoseconds. An error is returned if the format of the underlying
// file does not match.
//
// Uses /proc/stat defined by POSIX. Looks for the cpu
// statistics line and then sums up the first seven fields
// provided. See `man 5 proc` for details on specific field
// information.
// TODO: we should inject the route of /proc/stat in order to be able to mock the file and test the method
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

func (cg *CgroupsV1Fetcher) cpu(metric *cgroupstats.Metrics) (CPU, error) {
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

func (cg *CgroupsV1Fetcher) memory(metric *cgroupstats.Metrics) (Memory, error) {
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
	mem.SwapLimit = metric.Memory.Swap.Limit
	mem.KernelMemoryUsage = metric.Memory.Kernel.Usage
	return mem, nil
}
