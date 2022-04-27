package raw

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/newrelic/infra-integrations-sdk/log"
)

const (
	cgroupV2SystemdTemplate  = "/docker-%s.scope"
	cgroupV2CgroupfsTemplate = "/%s"
)

var cgroupV2MountPointNotFoundErr = errors.New("cgroups2 mountpoint was not found")

type cgroupV2Paths struct {
	// MountPoint is the cgroup2 mount point, Eg: /sys/fs/cgroup/system.slice/docker.service
	MountPoint string
	// Group is the group path, with the same format that the third parameter in /proc/<pid>/cgroup.
	Group string
}

func getCgroupV2Paths(hostRoot string, pid int, driver string, containerID string) (*cgroupV2Paths, error) {
	mountPoint, err := cgroupV2MountPoint(hostRoot, pid, defaultFileOpenFn)
	if err != nil {
		return nil, err
	}
	cgroupPath, err := cgroupV2GroupPath(driver, containerID)
	if err != nil {
		return nil, err
	}
	return &cgroupV2Paths{MountPoint: mountPoint, Group: cgroupPath}, nil
}

func cgroupV2GroupPath(driver string, containerID string) (string, error) {
	switch driver {
	case CgroupSystemd:
		return fmt.Sprintf(cgroupV2SystemdTemplate, containerID), nil
	case CgroupGroupfs:
		return fmt.Sprintf(cgroupV2CgroupfsTemplate, containerID), nil
	}
	return "", fmt.Errorf("invalid cgroup2 driver %q", driver)
}

func cgroupV2MountPoint(hostRoot string, pid int, fileOpen fileOpenFn) (string, error) {
	path := filepath.Join(hostRoot, mountsFilePath)
	mountsFile, err := fileOpen(path)
	if err != nil {
		return "", fmt.Errorf(
			"failed to open file %s, while detecting cgroup2 mountpoint error: %s",
			path, err,
		)
	}
	defer func() {
		if err := mountsFile.Close(); err != nil {
			log.Error("Error occurred while closing the file %s", err)
		}
	}()
	rootMountPoint, err := parseCgroupV2MountPoint(hostRoot, mountsFile)
	if err != nil {
		return "", fmt.Errorf("failed to parse cgroups2 mountpoint: %s", err)
	}
	cgroupFilePath := filepath.Join(hostRoot, fmt.Sprintf(cgroupFilePathTpl, pid))
	cgroupFile, err := fileOpen(cgroupFilePath)
	if err != nil {
		return "", fmt.Errorf(
			"failed to open file: %s, while detecting cgroup paths error: %v",
			cgroupFilePath, err,
		)
	}
	defer func() {
		if closeErr := cgroupFile.Close(); closeErr != nil {
			log.Error("Error occurred while closing the file: %v", closeErr)
		}
	}()
	cgroupPath, err := parseCgroupV2Path(cgroupFile)
	if err != nil {
		return "", fmt.Errorf("failed to parse cgroups2 paths, error: %s", err)
	}
	return filepath.Join(rootMountPoint, cgroupPath), nil
}

func parseCgroupV2MountPoint(hostRoot string, mountFile io.Reader) (string, error) {
	sc := bufio.NewScanner(mountFile)
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.HasPrefix(fields[2], "cgroup2") && strings.HasPrefix(fields[1], hostRoot) {
			return fields[1], nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", cgroupV2MountPointNotFoundErr
}

func parseCgroupV2Path(cgroupFile io.Reader) (string, error) {
	sc := bufio.NewScanner(cgroupFile)
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Split(line, ":")

		if len(fields) != 3 {
			return "", fmt.Errorf("unexpected cgroup file format: \"%s\"", line)
		}
		if fields[0] == "0" && fields[1] == "" {
			return fields[2], nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", errors.New("error parsing cgroup file, cgroup2 path not found")
}
