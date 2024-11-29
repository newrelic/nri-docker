// +build linux

package raw

import (
	"bufio"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/newrelic/infra-integrations-sdk/v3/log"
)

const (
	mountsFilePath            = "/proc/mounts"
	cgroupFilePathTpl         = "/proc/%d/cgroup"
	cgroupV1MountName         = "cgroup"
	cgroupV2MountName         = "cgroup2"
	cgroupV2UnifiedFilesystem = "/"
)

func getMountsFile(hostRoot string, mountPoints map[string]string, cgroupMountPointName string, fileOpen fileOpenFn) error {
	path := filepath.Join(hostRoot, mountsFilePath)
	mountsFile, err := fileOpen(path)
	if err != nil {
		return fmt.Errorf("failed to open file: %s, while detecting cgroup mountpoints error: %v",
			mountsFilePath, err)
	}

	defer func() {
		if closeErr := mountsFile.Close(); closeErr != nil {
			log.Error("Error occurred while closing the file: %v", closeErr)
		}
	}()

	sc := bufio.NewScanner(mountsFile)
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)

		switch cgroupMountPointName {
		case cgroupV1MountName:
			// Filter mount points if the type is not 'cgroup' or not mounted under </host>/sys
			if len(fields) < 3 || !strings.HasPrefix(fields[2], cgroupV1MountName) || !strings.HasPrefix(fields[1], hostRoot) {
				continue
			}

			for _, subsystem := range strings.Split(filepath.Base(fields[1]), ",") {
				if _, found := mountPoints[subsystem]; !found {
					mountPoints[subsystem] = filepath.Dir(fields[1])
				}
			}
		case cgroupV2MountName:
			if len(fields) >= 3 && strings.HasPrefix(fields[2], cgroupV2MountName) && strings.HasPrefix(fields[1], hostRoot) {
				mountPoints[cgroupV2UnifiedFilesystem] = fields[1]
				return nil
			}
		}
	}

	return sc.Err()
}

func getCgroupFilePaths(
	hostRoot string,
	pid int,
	cgroupPaths map[string]string,
	cgroupMountPointName string,
	fileOpen fileOpenFn,
) error {
	cgroupFilePath := filepath.Join(hostRoot, fmt.Sprintf(cgroupFilePathTpl, pid))
	cgroupFile, err := fileOpen(cgroupFilePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %s, while detecting cgroup paths error: %v",
			cgroupFilePath, err)
	}

	defer func() {
		if closeErr := cgroupFile.Close(); closeErr != nil {
			log.Error("Error occurred while closing the file: %v", closeErr)
		}
	}()

	sc := bufio.NewScanner(cgroupFile)
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Split(line, ":")

		if len(fields) != 3 {
			return fmt.Errorf("unexpected cgroup file format: \"%s\"", line)
		}

		switch cgroupMountPointName {
		case cgroupV1MountName:
			for _, subsystem := range strings.Split(fields[1], ",") {
				cgroupPaths[subsystem] = fields[2]
			}
		case cgroupV2MountName:
			if fields[0] == "0" && fields[1] == "" {
				cgroupPaths[cgroupV2UnifiedFilesystem] = fields[2]
				return nil
			}
		}
	}

	return sc.Err()
}
