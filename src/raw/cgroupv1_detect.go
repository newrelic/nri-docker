package raw

import (
	"fmt"
	"path/filepath"

	"github.com/containerd/cgroups"
)

type CgoupsV1Detector struct {
	openFn fileOpenFn
	paths  *cgroupV1Paths
}

func NewCgroupsV1Detector() *CgoupsV1Detector {
	return &CgoupsV1Detector{openFn: defaultFileOpenFn}
}

func (cgd *CgoupsV1Detector) PopulatePaths(hostRoot string, pid int) error {
	mountPoints := make(map[string]string)
	err := getMountsFile(hostRoot, mountPoints, cgroup1MountName, cgd.openFn)
	if err != nil {
		return fmt.Errorf("failed to parse cgroups mountpoints: %v", err)
	}

	cgroupPaths := make(map[string]string)
	err = getCgroupFilePaths(hostRoot, pid, cgroupPaths, cgroup1MountName, cgd.openFn)
	if err != nil {
		return fmt.Errorf("failed to parse cgroup paths error: %v", err)
	}

	cgd.paths = &cgroupV1Paths{
		mountPoints: mountPoints,
		paths:       cgroupPaths,
	}

	return nil
}

type cgroupV1Paths struct {
	mountPoints map[string]string
	paths       map[string]string
}

func (cgi *cgroupV1Paths) getMountPoint(name cgroups.Name) (string, error) {
	if result, ok := cgi.mountPoints[string(name)]; ok {
		return result, nil
	}

	return "", fmt.Errorf("cgroup mount point not found for subsystem %s", name)
}

func (cgi *cgroupV1Paths) getPath(name cgroups.Name) (string, error) {
	if result, ok := cgi.paths[string(name)]; ok {
		return result, nil
	}

	return "", fmt.Errorf("cgroup path not found for subsystem %s", name)
}

func (cgi *cgroupV1Paths) getSingleFileUintStat(name cgroups.Name, stat string) (uint64, error) {
	fp, err := cgi.getFullPath(name)
	if err != nil {
		return 0, err
	}
	c, err := ParseStatFileContentUint64(filepath.Join(fp, stat))
	if err != nil {
		return 0, err
	}
	return c, nil
}

func (cgi *cgroupV1Paths) getFullPath(name cgroups.Name) (string, error) {
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
func (cgi *cgroupV1Paths) getHierarchyFn() cgroups.Hierarchy {
	return func() ([]cgroups.Subsystem, error) {
		var subsystems []cgroups.Subsystem

		if cpusetMountPoint, ok := cgi.mountPoints[string(cgroups.Cpuset)]; ok {
			subsystems = append(subsystems, cgroups.NewCpuset(cpusetMountPoint))
		}
		if cpuMountPoint, ok := cgi.mountPoints[string(cgroups.Cpu)]; ok {
			subsystems = append(subsystems, cgroups.NewCpu(cpuMountPoint))
		}
		if cpuacctMountPoint, ok := cgi.mountPoints[string(cgroups.Cpuacct)]; ok {
			subsystems = append(subsystems, cgroups.NewCpuacct(cpuacctMountPoint))
		}
		if memoryMountPoint, ok := cgi.mountPoints[string(cgroups.Memory)]; ok {
			subsystems = append(subsystems, cgroups.NewMemory(memoryMountPoint))
		}
		if blkioMountPoint, ok := cgi.mountPoints[string(cgroups.Blkio)]; ok {
			subsystems = append(subsystems, cgroups.NewBlkio(blkioMountPoint))
		}
		if pidsMountPoint, ok := cgi.mountPoints[string(cgroups.Pids)]; ok {
			subsystems = append(subsystems, cgroups.NewPids(pidsMountPoint))
		}

		return subsystems, nil
	}
}
