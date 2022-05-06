package raw

import (
	"fmt"
	"github.com/newrelic/infra-integrations-sdk/log"
	"io"
	"os"
	"path/filepath"
)

const (
	mountsFilePath    = "/proc/mounts"
	cgroupFilePathTpl = "/proc/%d/cgroup"
)

type CgroupDetector interface {
	GetPaths(hostRoot string, pid int) (CgroupPaths, error)
}

type CgroupPaths interface {
	MountPoint(point string) (string, error)
	Path(path string) (string, error)
	GetSingleFileUintStat(system string, stat string) (uint64, error)
}

type (
	fileOpenFn             func(string) (io.ReadCloser, error)
	parseCgroupMountPoints func(hostRoot string, mountFileInfo io.Reader) (map[string]string, error)
	parseCgroupPath        func(cgroupFile io.Reader) (map[string]string, error)
)

func defaultFileOpenFn(filePath string) (io.ReadCloser, error) {
	return os.Open(filePath)
}

func getMountsFile(
	hostRoot string, parseCgroupMountPoints parseCgroupMountPoints, fileOpen fileOpenFn) (map[string]string, error) {
	path := filepath.Join(hostRoot, mountsFilePath)
	mountsFile, err := fileOpen(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %s, while detecting cgroup mountpoints error: %v",
			mountsFilePath, err)
	}
	defer func() {
		if closeErr := mountsFile.Close(); closeErr != nil {
			log.Error("Error occurred while closing the file: %v", closeErr)
		}
	}()

	return parseCgroupMountPoints(hostRoot, mountsFile)
}

func getCgroupFilePath(hostRoot string, pid int, parseCgroupPath parseCgroupPath, fileOpen fileOpenFn) (map[string]string, error) {
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
	return parseCgroupPath(cgroupFile)
}
