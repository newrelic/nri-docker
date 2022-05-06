package raw

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/containerd/cgroups"
)

type CgoupsV1Detector struct {
	openFn fileOpenFn
}

func NewCgroupsV1Detector() CgoupsV1Detector {
	return CgoupsV1Detector{openFn: defaultFileOpenFn}
}

func (cgd CgoupsV1Detector) GetPaths(hostRoot string, pid int) (CgroupPaths, error) {
	mountPoints, err := getMountsFile(hostRoot, cgd.parseCgroupV1MountPoints, cgd.openFn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cgroup mountpoints error: %v", err)
	}

	paths, err := getCgroupFilePath(hostRoot, pid, cgd.parseCgroupV1Paths, cgd.openFn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cgroup paths error: %v", err)
	}

	return &CgroupV1Paths{
		mountPoints: mountPoints,
		paths:       paths,
	}, nil
}

func (cgd CgoupsV1Detector) parseCgroupV1MountPoints(hostRoot string, mountFileInfo io.Reader) (map[string]string, error) {
	mountPoints := make(map[string]string)

	sc := bufio.NewScanner(mountFileInfo)
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)

		// Filter mount points if the type is not 'cgroup' or not mounted under </host>/sys
		if len(fields) < 3 || !strings.HasPrefix(fields[2], "cgroup") || !strings.HasPrefix(fields[1], hostRoot) {
			continue
		}

		for _, subsystem := range strings.Split(filepath.Base(fields[1]), ",") {
			if _, found := mountPoints[subsystem]; !found {
				mountPoints[subsystem] = filepath.Dir(fields[1])
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	return mountPoints, nil
}

func (cgd CgoupsV1Detector) parseCgroupV1Paths(cgroupFile io.Reader) (map[string]string, error) {
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

type CgroupV1Paths struct {
	mountPoints map[string]string
	paths       map[string]string
}

func (cgi *CgroupV1Paths) MountPoint(system string) (string, error) {
	if result, ok := cgi.mountPoints[system]; ok {
		return result, nil
	}

	return "", fmt.Errorf("cgroup mount point not found for subsystem %s", system)
}

func (cgi *CgroupV1Paths) Path(path string) (string, error) {
	if result, ok := cgi.paths[path]; ok {
		return result, nil
	}

	return "", fmt.Errorf("cgroup path not found for subsystem %s", path)
}

func (cgi *CgroupV1Paths) GetSingleFileUintStat(system string, stat string) (uint64, error) {
	fp, err := cgi.getFullPath(cgroups.Name(system))
	if err != nil {
		return 0, err
	}
	c, err := ParseStatFileContentUint64(filepath.Join(fp, stat))
	if err != nil {
		return 0, err
	}
	return c, nil
}

func (cgi *CgroupV1Paths) getPathCgroupFiltered(name cgroups.Name) (string, error) {
	return cgi.Path(string(name))
}

func (cgi *CgroupV1Paths) getFullPath(name cgroups.Name) (string, error) {
	cgroupMountPoint, err := cgi.MountPoint(string(name))
	if err != nil {
		return "", err
	}

	cgroupPath, err := cgi.Path(string(name))
	if err != nil {
		return "", err
	}

	return filepath.Join(cgroupMountPoint, string(name), cgroupPath), nil
}

// returns the subsystems where cgroups library has to look for, attaching the
// hostContainerPath prefix to the folder if the integration is running inside a container
func (cgi *CgroupV1Paths) getHierarchyFn() cgroups.Hierarchy {
	return func() ([]cgroups.Subsystem, error) {
		var subsystems []cgroups.Subsystem

		if cpusetMountPoint, err := cgi.MountPoint(string(cgroups.Cpuset)); err != nil {
			subsystems = append(subsystems, cgroups.NewCpuset(cpusetMountPoint))
		}
		if cpuMountPoint, err := cgi.MountPoint(string(cgroups.Cpu)); err != nil {
			subsystems = append(subsystems, cgroups.NewCpu(cpuMountPoint))
		}
		if cpuacctMountPoint, err := cgi.MountPoint(string(cgroups.Cpuacct)); err != nil {
			subsystems = append(subsystems, cgroups.NewCpuacct(cpuacctMountPoint))
		}
		if memoryMountPoint, err := cgi.MountPoint(string(cgroups.Memory)); err != nil {
			subsystems = append(subsystems, cgroups.NewMemory(memoryMountPoint))
		}
		if blkioMountPoint, err := cgi.MountPoint(string(cgroups.Blkio)); err != nil {
			subsystems = append(subsystems, cgroups.NewBlkio(blkioMountPoint))
		}
		if pidsMountPoint, err := cgi.MountPoint(string(cgroups.Pids)); err != nil {
			subsystems = append(subsystems, cgroups.NewPids(pidsMountPoint))
		}

		return subsystems, nil
	}
}
