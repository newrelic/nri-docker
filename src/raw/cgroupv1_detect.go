package raw

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/containerd/cgroups"
	"github.com/newrelic/infra-integrations-sdk/log"
)

// getCgroupV1Paths will detect the cgroup paths for a container pid.
func getCgroupV1Paths(hostRoot string, pid int) (*cgroupV1Paths, error) {
	return cgroupV1PathsFetch(hostRoot, pid, defaultFileOpenFn)
}

func cgroupV1PathsFetch(hostRoot string, pid int, fileOpen fileOpenFn) (*cgroupV1Paths, error) {

	mountsFilePath := filepath.Join(hostRoot, mountsFilePath)
	mountsFile, err := fileOpen(mountsFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %s, while detecting cgroup mountpoints error: %v",
			mountsFilePath, err)
	}
	defer func() {
		if closeErr := mountsFile.Close(); closeErr != nil {
			log.Error("Error occurred while closing the file: %v", closeErr)
		}
	}()

	mountPoints, err := parseCgroupV1MountPoints(hostRoot, mountsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cgroup mountpoints error: %v", err)
	}

	cgroupFilePath := filepath.Join(hostRoot, fmt.Sprintf(cgroupFilePathTpl, pid))
	cgroupFile, err := fileOpen(cgroupFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %s, while detecting cgroup paths error: %v",
			cgroupFilePath, err)
	}
	defer func() {
		if closeErr := cgroupFile.Close(); closeErr != nil {
			log.Error("Error occurred while closing the file: %v", closeErr)
		}
	}()
	paths, err := parseCgroupV1Paths(cgroupFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cgroup paths error: %v", err)
	}

	return &cgroupV1Paths{
		mountPoints: mountPoints,
		paths:       paths,
	}, nil
}

type cgroupV1Paths struct {
	mountPoints map[string]string
	paths       map[string]string
}

func (cgi *cgroupV1Paths) getPath(name cgroups.Name) (string, error) {

	if result, ok := cgi.paths[string(name)]; ok {
		return result, nil
	}

	return "", fmt.Errorf("cgroup path not found for subsystem %s", name)
}

func (cgi *cgroupV1Paths) getFullPath(name cgroups.Name) (string, error) {
	cgroupMountPoint, exists := cgi.mountPoints[string(name)]
	if !exists {
		return "", fmt.Errorf("cgroup mount point not found for subsystem %s", name)
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

func parseCgroupV1MountPoints(hostRoot string, mountFileInfo io.Reader) (map[string]string, error) {
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

func parseCgroupV1Paths(cgroupFile io.Reader) (map[string]string, error) {
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
