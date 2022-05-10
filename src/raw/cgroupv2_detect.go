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

type CgroupV2Detector interface {
	Paths(hostRoot string, pid int) (CgroupV2PathGetter, error)
}

type CgroupV2PathGetter interface {
	getFullPath() string
	getSingleFileUintStat(stat string) (uint64, error)
	getMountPoint() string
	getGroup() string
}

type CgroupV2PathParser struct {
	openFn fileOpenFn
}

type V2MountPoint string

func NewCgroupV2PathParser() *CgroupV2PathParser {
	return &CgroupV2PathParser{openFn: defaultFileOpenFn}
}

func (cgd *CgroupV2PathParser) Paths(hostRoot string, pid int) (CgroupV2PathGetter, error) {
	fullpath, err := cgd.cgroupV2FullPath(hostRoot, pid)
	if err != nil {
		return nil, err
	}
	mountPoint, group := cgd.splitMountPointAndGroup(fullpath)
	return &cgroupV2Paths{
		mountPoint: mountPoint,
		group:      group,
	}, nil
}

// cgroupV2FullPath returns the cgroup mount point which is built joining info from mountsFile (Eg: /proc/mounts)
// and pid's cgroup file (Eg: /proc/<pid>/cgroup)
func (cgd *CgroupV2PathParser) cgroupV2FullPath(hostRoot string, pid int) (string, error) {
	mountPoints := make(map[string]string)
	err := getMountsFile(hostRoot, mountPoints, cgroupV2MountName, cgd.openFn)
	if err != nil {
		return "", fmt.Errorf("failed to parse cgroups2 mountpoint: %s", err)
	}
	if mountPoints[cgroupV2UnifiedFilesystem] == "" {
		return "", v2MountPointNotFoundErr
	}

	cgroupPaths := make(map[string]string)
	err = getCgroupFilePaths(hostRoot, pid, cgroupPaths, cgroupV2MountName, cgd.openFn)
	if err != nil {
		return "", fmt.Errorf("failed to parse cgroups2 paths, error: %s", err)
	}
	if cgroupPaths[cgroupV2UnifiedFilesystem] == "" {
		return "", fmt.Errorf("error parsing cgroup file, %v", v2PathNotFoundErr)
	}

	return filepath.Join(mountPoints[cgroupV2UnifiedFilesystem], cgroupPaths[cgroupV2UnifiedFilesystem]), nil
}

func (cgd *CgroupV2PathParser) splitMountPointAndGroup(fullpath string) (string, string) {
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

func (cgi *cgroupV2Paths) getMountPoint() string {
	return cgi.mountPoint
}

func (cgi *cgroupV2Paths) getGroup() string {
	return cgi.group
}
