package raw

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

var cgroupV2MountPointNotFoundErr = errors.New("cgroups2 mountpoint was not found")

const cgroupsV2UnifiedPath = "/"

type CgoupsV2Detector struct {
	openFn fileOpenFn
}

func NewCgroupsV2Detector() CgoupsV2Detector {
	return CgoupsV2Detector{openFn: defaultFileOpenFn}
}

func (cgd CgoupsV2Detector) GetPaths(hostRoot string, pid int) (CgroupPaths, error) {
	fullpath, err := cgd.cgroupV2FullPath(hostRoot, pid, cgd.openFn)
	if err != nil {
		return nil, err
	}
	mountPoint, group := cgd.splitMountPointAndGroup(fullpath)
	return &CgroupV2Paths{
		mountPoints: map[string]string{cgroupsV2UnifiedPath: mountPoint},
		paths:       map[string]string{cgroupsV2UnifiedPath: group},
	}, nil
}

// cgroupV2FullPath returns the cgroup mount point which is built joining info from mountsFile (Eg: /proc/mounts)
// and pid's cgroup file (Eg: /proc/<pid>/cgroup)
func (cgd CgoupsV2Detector) cgroupV2FullPath(hostRoot string, pid int, fileOpen fileOpenFn) (string, error) {
	mountPoint, err := getMountsFile(hostRoot, cgd.parseCgroupV2MountPoint, fileOpen)
	if err != nil {
		return "", fmt.Errorf("failed to parse cgroups2 mountpoint: %s", err)
	}

	cgroupPath, err := getCgroupFilePath(hostRoot, pid, cgd.parseCgroupV2Path, fileOpen)
	if err != nil {
		return "", fmt.Errorf("failed to parse cgroups2 paths, error: %s", err)
	}
	return filepath.Join(mountPoint[cgroupsV2UnifiedPath], cgroupPath[cgroupsV2UnifiedPath]), nil
}

func (cgd CgoupsV2Detector) splitMountPointAndGroup(fullpath string) (string, string) {
	mountPoint := filepath.Dir(fullpath)
	group := filepath.Base(fullpath)
	group = "/" + group
	return mountPoint, group
}

func (cgd CgoupsV2Detector) parseCgroupV2MountPoint(hostRoot string, mountFile io.Reader) (map[string]string, error) {
	sc := bufio.NewScanner(mountFile)
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.HasPrefix(fields[2], "cgroup2") && strings.HasPrefix(fields[1], hostRoot) {
			return map[string]string{cgroupsV2UnifiedPath: fields[1]}, nil
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return nil, cgroupV2MountPointNotFoundErr
}

func (cgd CgoupsV2Detector) parseCgroupV2Path(cgroupFile io.Reader) (map[string]string, error) {
	sc := bufio.NewScanner(cgroupFile)
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Split(line, ":")

		if len(fields) != 3 {
			return nil, fmt.Errorf("unexpected cgroup file format: \"%s\"", line)
		}
		if fields[0] == "0" && fields[1] == "" {
			return map[string]string{cgroupsV2UnifiedPath: fields[2]}, nil
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("error parsing cgroup file, cgroup2 path not found")
}

type CgroupV2Paths struct {
	mountPoints map[string]string
	paths       map[string]string
}

func (cgi *CgroupV2Paths) MountPoint(system string) (string, error) {
	if result, ok := cgi.mountPoints[system]; ok {
		return result, nil
	}

	return "", fmt.Errorf("cgroup mount point not found cgroups V2")
}

func (cgi *CgroupV2Paths) Path(path string) (string, error) {
	if result, ok := cgi.paths[path]; ok {
		return result, nil
	}

	return "", fmt.Errorf("cgroupV2 path not found %s", path)
}

func (c *cgroupV2Paths) FullPath() string {
	return filepath.Join(c.MountPoint, c.Group)
}

func (cgi *CgroupV2Paths) GetSingleFileUintStat(system, stat string) (uint64, error) {
	// get full path
	// /sys/fs/cgroup/system.slice/cpu.weight
	path, _ := cgi.MountPoint(system)
	group, _ := cgi.Path(path)
	fp := filepath.Join(path, group, stat)

	c, err := ParseStatFileContentUint64(filepath.Join(fp, stat))
	if err != nil {
		return 0, err
	}
	return c, nil
}
