// Package raw fetches raw system-level metrics as they are presented by the operating system
package raw

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/containerd/cgroups"
	"github.com/docker/docker/api/types"
	"github.com/newrelic/infra-integrations-sdk/log"
)

const cgroupDir = "/cgroup"
const cgroupDevice = "cgroup"

var localCgroupPaths = []string{
	"/sys/fs/cgroup",
	"/cgroup",
}

//type CgroupMountPoints map[cgroups.Name]string

//func extractCgroupMountPoints(mounts []*mount) CgroupMountPoints {
//	cgroupMountPoints := make(CgroupMountPoints)
//
//	for _, mount := range mounts {
//		if mount.Device == cgroupDevice {
//			for _, subsystem := range strings.Split(filepath.Base(mount.MountPoint), ",") {
//				// @todo validate cgroup
//				cgroupMountPoints[cgroups.Name(subsystem)] = filepath.Dir(mount.MountPoint)
//			}
//		}
//	}
//	return cgroupMountPoints
//}

// CgroupsFetcher fetches the metrics that can be found in cgroups file system
type CgroupsFetcher struct {
	subsystems     cgroups.Hierarchy
	networkFetcher *networkFetcher
	cgroupInfoFetcher *CgroupInfoFetcher
}

// NewCGroupsFetcher creates a new cgroups data fetcher.
func NewCGroupsFetcher(hostRoot, cgroup string) (*CgroupsFetcher, error) {

	// TODO handle cgroup

	return &CgroupsFetcher{
		subsystems:        subsystems(),
		networkFetcher:    newNetworkFetcher(hostRoot),
		cgroupInfoFetcher: newCgroupInfoFetcher(hostRoot),
	}, nil
}

// gets the relative path to a cgroup container based on the container metadata
func staticPath(c types.ContainerJSON) cgroups.Path {
	var parent string
	if c.HostConfig == nil || c.HostConfig.CgroupParent == "" {
		parent = "docker"
	} else {
		parent = c.HostConfig.CgroupParent
	}
	return cgroups.StaticPath(path.Join(parent, c.ID))
}

// returns a Metrics without the network: TODO: populate also network from libcgroups
func (cg *CgroupsFetcher) Fetch(c types.ContainerJSON) (Metrics, error) {
	stats := Metrics{}

	control, err := cgroups.Load(subsystems(), staticPath(c))
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

	if stats.Blkio, err = cg.blkio(c.ID); err != nil {
		log.Error("couldn't read blkio stats: %v", err)
	}

	if stats.CPU, err = cpu(metrics); err != nil {
		log.Error("couldn't read cpu stats: %v", err)
	}

	if stats.Memory, err = memory(metrics); err != nil {
		log.Error("couldn't read memory stats: %v", err)
	}

	stats.ContainerID = c.ID
	stats.Network, err = cg.networkFetcher.Fetch(c.State.Pid)
	if err != nil {
		log.Error(
			"couldn't fetch network stats for container %s from cgroups: %v",
			c.ID,
			err,
		)
		return stats, err
	}

	return stats, nil
}

func pids(metrics *cgroups.Metrics) (Pids, error) {
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
func (cg *CgroupsFetcher) blkio(containerID string) (Blkio, error) {
	cpath := path.Join("cg.cgroupPath", "blkio", "docker", containerID)

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

func cpu(metric *cgroups.Metrics) (CPU, error) {
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

func memory(metric *cgroups.Metrics) (Memory, error) {
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

// returns the subsystems where cgroups library has to look for, attaching the
// hostContainerPath prefix to the folder if the integration is running inside a container
func subsystems() cgroups.Hierarchy {
	return func() ([]cgroups.Subsystem, error) {
		subsystems := []cgroups.Subsystem{}

		//if cpusetMountPoint, ok := mountPoints[cgroups.Cpuset]; ok {
		//	subsystems = append(subsystems, cgroups.NewCputset(cpusetMountPoint))
		//}
		//if cpuMountPoint, ok := mountPoints[cgroups.Cpu]; ok {
		//	subsystems = append(subsystems, cgroups.NewCpu(cpuMountPoint))
		//}
		//if cpuacctMountPoint, ok := mountPoints[cgroups.Cpuacct]; ok {
		//	subsystems = append(subsystems, cgroups.NewCpuacct(cpuacctMountPoint))
		//}
		//if memoryMountPoint, ok := mountPoints[cgroups.Memory]; ok {
		//	subsystems = append(subsystems, cgroups.NewMemory(memoryMountPoint))
		//}
		//if blkioMountPoint, ok := mountPoints[cgroups.Blkio]; ok {
		//	subsystems = append(subsystems, cgroups.NewBlkio(blkioMountPoint))
		//}
		//if pidsMountPoint, ok := mountPoints[cgroups.Pids]; ok {
		//	subsystems = append(subsystems, cgroups.NewPids(pidsMountPoint))
		//}

		return subsystems, nil
	}
}


















type CgroupInfoFetcher struct {
	fileOpenFn func(string) (io.ReadCloser, error)
	root       string
}

func newCgroupInfoFetcher(root string) *CgroupInfoFetcher {
	return &CgroupInfoFetcher{
		fileOpenFn: func(filePath string) (io.ReadCloser, error) {
			return os.Open(filePath)
		},
		root: root,
	}
}

const (
	mountsFilePathTpl    = "%s/proc/mounts"
	cgroupFilePathTpl = "%s/proc/%d/cgroup"
)

func (f *CgroupInfoFetcher) Parse(pid int) (*CgroupInfo, error) {

	mountsFilePath := fmt.Sprintf(mountsFilePathTpl,f.root)
	mountsFile, err := f.fileOpenFn(mountsFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %s, while detecting cgroup mountpoints error: %v",
			mountsFilePath, err)
	}
	defer func() {
		if closeErr := mountsFile.Close(); closeErr != nil {
			log.Error("Error occurred while closing the file: %v", closeErr)
		}
	}()

	cgroupMountPoints, err := parseCgroupMountPoints(mountsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cgroup mountpoints error: %v", err)
	}

	cgroupFilePath := fmt.Sprintf(cgroupFilePathTpl,f.root, pid)
	cgroupFile, err := f.fileOpenFn(cgroupFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %s, while detecting cgroup paths error: %v",
			cgroupFilePath, err)
	}
	defer func() {
		if closeErr := cgroupFile.Close(); closeErr != nil {
			log.Error("Error occurred while closing the file: %v", closeErr)
		}
	}()
	cgroupPaths, err := parseCgroupPaths(cgroupFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cgroup paths error: %v", err)
	}

	return &CgroupInfo{
		mountPoints: cgroupMountPoints,
		paths:       cgroupPaths,
	}, nil
}

type CgroupInfo struct {
	mountPoints map[string]string
	paths       map[string]string
}


func (cgi *CgroupInfo) getPath(name cgroups.Name) (string, error) {

	if result, ok := cgi.paths[string(name)]; ok {
		return result, nil
	}

	return "", fmt.Errorf("cgroup path not found for subsystem %s", name)
}

func (cgi *CgroupInfo) getMountPoint(name cgroups.Name) (string, error) {

	if result, ok := cgi.mountPoints[string(name)]; ok {
		return result, nil
	}

	return "", fmt.Errorf("cgroup mount point not found for subsystem %s", name)
}

func (cgi *CgroupInfo) getFullPath(name cgroups.Name) (string, error) {

	cgroupMountPoint, err := cgi.getMountPoint(name)
	if err != nil {
		return "", err
	}

	cgroupPath, err := cgi.getPath(name)
	if err != nil {
		return "", err
	}

	return filepath.Join(cgroupMountPoint, string(name), cgroupPath), nil
}


// returns the subsystems where cgroups library has to look for, attaching the
// hostContainerPath prefix to the folder if the integration is running inside a container
func (cgi *CgroupInfo) getHierarchyFn() cgroups.Hierarchy {
	return func() ([]cgroups.Subsystem, error) {
		subsystems := []cgroups.Subsystem{}

		//if cpusetMountPoint, ok := mountPoints[cgroups.Cpuset]; ok {
		//	subsystems = append(subsystems, cgroups.NewCputset(cpusetMountPoint))
		//}
		//if cpuMountPoint, ok := mountPoints[cgroups.Cpu]; ok {
		//	subsystems = append(subsystems, cgroups.NewCpu(cpuMountPoint))
		//}
		//if cpuacctMountPoint, ok := mountPoints[cgroups.Cpuacct]; ok {
		//	subsystems = append(subsystems, cgroups.NewCpuacct(cpuacctMountPoint))
		//}
		//if memoryMountPoint, ok := mountPoints[cgroups.Memory]; ok {
		//	subsystems = append(subsystems, cgroups.NewMemory(memoryMountPoint))
		//}
		//if blkioMountPoint, ok := mountPoints[cgroups.Blkio]; ok {
		//	subsystems = append(subsystems, cgroups.NewBlkio(blkioMountPoint))
		//}
		//if pidsMountPoint, ok := mountPoints[cgroups.Pids]; ok {
		//	subsystems = append(subsystems, cgroups.NewPids(pidsMountPoint))
		//}

		return subsystems, nil
	}
}


func parseCgroupMountPoints(mountFileInfo io.Reader) (map[string]string, error) {
	mountPoints := make(map[string]string)

	sc := bufio.NewScanner(mountFileInfo)
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)

		if len(fields) != 6 || fields[0] != "cgroup" {
			continue
		}

		for _, subsystem := range strings.Split(filepath.Base(fields[1]), ",") {
			mountPoints[subsystem] = filepath.Dir(fields[1])
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	return mountPoints, nil
}

func parseCgroupPaths(cgroupFile io.Reader) (map[string]string, error) {
	cgroupPaths := make(map[string]string)

	sc := bufio.NewScanner(cgroupFile)
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Split(line, ":")

		if len(fields) != 3 {
			return nil, fmt.Errorf("unexpected cgroup file format: \"%s\"", line)
		}

		for _, subsystem := range strings.Split(fields[1], ",") {
			cgroupPaths[subsystem] = fields[2]
		}
	}

	if err := sc.Err(); err != nil {
		return nil, err
	}

	return cgroupPaths, nil
}

