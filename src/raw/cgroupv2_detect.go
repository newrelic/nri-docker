package raw

import (
	"errors"
	"fmt"
	"path/filepath"
)

var (
	v2MountPointNotFoundErr = errors.New("cgroups2 mountpoint was not found")
	v2PathNotFoundErr       = errors.New("cgroup2 path not found")
)

type CgoupsV2Detector struct {
	openFn fileOpenFn
	paths  *cgroupV2Paths
}

type V2MountPoint string

func NewCgroupsV2Detector() *CgoupsV2Detector {
	return &CgoupsV2Detector{openFn: defaultFileOpenFn}
}

func (cgd *CgoupsV2Detector) PopulatePaths(hostRoot string, pid int) error {
	fullpath, err := cgd.cgroupV2FullPath(hostRoot, pid)
	if err != nil {
		return err
	}
	mountPoint, group := cgd.splitMountPointAndGroup(fullpath)
	cgd.paths = &cgroupV2Paths{
		mountPoint: mountPoint,
		group:      group,
	}

	return nil
}

// cgroupV2FullPath returns the cgroup mount point which is built joining info from mountsFile (Eg: /proc/mounts)
// and pid's cgroup file (Eg: /proc/<pid>/cgroup)
func (cgd *CgoupsV2Detector) cgroupV2FullPath(hostRoot string, pid int) (string, error) {
	mountPoints := make(map[string]string)
	err := getMountsFile(hostRoot, mountPoints, cgroup2MountName, cgd.openFn)
	if err != nil {
		return "", fmt.Errorf("failed to parse cgroups2 mountpoint: %s", err)
	}
	if mountPoints[cgroup2UnifiedFilesystem] == "" {
		return "", v2MountPointNotFoundErr
	}

	cgroupPaths := make(map[string]string)
	err = getCgroupFilePaths(hostRoot, pid, cgroupPaths, cgroup2MountName, cgd.openFn)
	if err != nil {
		return "", fmt.Errorf("failed to parse cgroups2 paths, error: %s", err)
	}
	if cgroupPaths[cgroup2UnifiedFilesystem] == "" {
		return "", fmt.Errorf("error parsing cgroup file, %v", v2PathNotFoundErr)
	}

	return filepath.Join(mountPoints[cgroup2UnifiedFilesystem], cgroupPaths[cgroup2UnifiedFilesystem]), nil
}

func (cgd *CgoupsV2Detector) splitMountPointAndGroup(fullpath string) (string, string) {
	mountPoint := filepath.Dir(fullpath)
	group := filepath.Base(fullpath)
	group = "/" + group
	return mountPoint, group
}

type cgroupV2Paths struct {
	// MountPoint is the cgroup2 mount point, Eg: /sys/fs/cgroup
	mountPoint string
	// Group is the group path, with the same format that the third parameter in /proc/<pid>/cgroup.
	// Eg: /system.slice/docker.service
	group string
}

func (cgi *cgroupV2Paths) getFullPath() string {
	return filepath.Join(cgi.mountPoint, cgi.group)
}

func (cgi *cgroupV2Paths) getSingleFileUintStat(stat string) (uint64, error) {
	fp := filepath.Join(cgi.mountPoint, cgi.group, stat)

	c, err := ParseStatFileContentUint64(fp)
	if err != nil {
		return 0, err
	}
	return c, nil
}
