package raw

import (
	"fmt"
	"github.com/containerd/cgroups"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCgroupMountPoints(t *testing.T) {

	mountInfoFileContains := `tmpfs /dev/shm tmpfs rw,nosuid,nodev 0 0
tmpfs /run/lock tmpfs rw,nosuid,nodev,noexec,relatime,size=5120k 0 0
tmpfs /sys/fs/cgroup tmpfs ro,nosuid,nodev,noexec,mode=755 0 0
cgroup /sys/fs/cgroup/systemd cgroup rw,nosuid,nodev,noexec,relatime,xattr,name=systemd 0 0
cgroup /sys/fs/cgroup/cpu,cpuacct cgroup rw,nosuid,nodev,noexec,relatime,cpu,cpuacct 0 0`
	mountFileInfo := strings.NewReader(mountInfoFileContains)

	expected := map[string]string{
		"cpu":     "/sys/fs/cgroup",
		"systemd": "/sys/fs/cgroup",
		"cpuacct": "/sys/fs/cgroup",
	}

	actual, err := parseCgroupMountPoints(mountFileInfo)
	assert.NoError(t, err)

	assert.Equal(t, expected, actual)
}

func TestParseCgroupPaths(t *testing.T) {
	cgroupFileContains := `4:pids:/system.slice/docker-ea06501e021b11a0d46a09de007b3d71bd6f37537cceabd2c3cbfa7f9b3da1ee.scope
	3:cpuset:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0
	2:cpu,cpuacct:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0
	1:name=systemd:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0`
	cgroupPaths := strings.NewReader(cgroupFileContains)

	expected := map[string]string{
		"pids":         "/system.slice/docker-ea06501e021b11a0d46a09de007b3d71bd6f37537cceabd2c3cbfa7f9b3da1ee.scope",
		"cpuset":       "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0",
		"cpu":          "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0",
		"cpuacct":      "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0",
		"name=systemd": "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0",
	}

	actual, err := parseCgroupPaths(cgroupPaths)
	assert.NoError(t, err)

	assert.Equal(t, expected, actual)
}

func TestCgroupPathsGetFullPath(t *testing.T) {

	cgroupInfo := &cgroupPaths{
		hostRoot: "/custom/host",
		mountPoints: map[string]string{
			"cpu":     "/sys/fs/cgroup",
			"systemd": "/sys/fs/cgroup",
			"cpuacct": "/sys/fs/cgroup",
		},
		paths: map[string]string{
			"pids":         "/system.slice/docker-ea06501e021b11a0d46a09de007b3d71bd6f37537cceabd2c3cbfa7f9b3da1ee.scope",
			"cpuset":       "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0",
			"cpu":          "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0",
			"cpuacct":      "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0",
			"name=systemd": "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0",
		},
	}

	fullPathCPU, err := cgroupInfo.getFullPath(cgroups.Cpu)
	assert.NoError(t, err)
	assert.Equal(t, "/custom/host/sys/fs/cgroup/cpu/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0", fullPathCPU)

	fullPathCpuacct, err := cgroupInfo.getFullPath(cgroups.Cpuacct)
	assert.NoError(t, err)
	assert.Equal(t, "/custom/host/sys/fs/cgroup/cpuacct/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0", fullPathCpuacct)
}

func TestCgroupPathsGetMountPoint(t *testing.T) {
	cgroupInfo := &cgroupPaths{
		hostRoot: "/custom/host",
		mountPoints: map[string]string{
			"cpu":     "/sys/fs/cgroup",
			"systemd": "/sys/fs/cgroup",
			"cpuacct": "/sys/fs/cgroup",
		},
	}

	mountPoint, err := cgroupInfo.getMountPoint(cgroups.Cpu)
	assert.NoError(t, err)
	assert.Equal(t, "/custom/host/sys/fs/cgroup", mountPoint)
}

func TestCgroupPathsFetcherParse(t *testing.T) {

	filesMap := map[string]string{
		"/custom/host/proc/mounts": `tmpfs /dev/shm tmpfs rw,nosuid,nodev 0 0
tmpfs /run/lock tmpfs rw,nosuid,nodev,noexec,relatime,size=5120k 0 0
tmpfs /sys/fs/cgroup tmpfs ro,nosuid,nodev,noexec,mode=755 0 0
cgroup /sys/fs/cgroup/systemd cgroup rw,nosuid,nodev,noexec,relatime,xattr,name=systemd 0 0
cgroup /sys/fs/cgroup/cpu,cpuacct cgroup rw,nosuid,nodev,noexec,relatime,cpu,cpuacct 0 0`,
		"/custom/host/proc/123/cgroup": `4:pids:/system.slice/docker-ea06501e021b11a0d46a09de007b3d71bd6f37537cceabd2c3cbfa7f9b3da1ee.scope
3:cpuset:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0
2:cpu,cpuacct:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0
1:name=systemd:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0`,
	}

	cgroupInfo, err := cgroupPathsFetch("/custom/host",123, createFileOpenFnMock(filesMap))

	expected := &cgroupPaths{
		hostRoot: "/custom/host",
		mountPoints: map[string]string{
			"cpu":     "/sys/fs/cgroup",
			"systemd": "/sys/fs/cgroup",
			"cpuacct": "/sys/fs/cgroup",
		},
		paths: map[string]string{
			"pids":         "/system.slice/docker-ea06501e021b11a0d46a09de007b3d71bd6f37537cceabd2c3cbfa7f9b3da1ee.scope",
			"cpuset":       "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0",
			"cpu":          "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0",
			"cpuacct":      "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0",
			"name=systemd": "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0",
		},
	}

	assert.NoError(t, err)
	assert.Equal(t, expected, cgroupInfo)
}

func TestCgroupPathsSubsystems(t *testing.T) {
	filesMap := map[string]string{
		"/custom/host/proc/mounts": `sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
udev /dev devtmpfs rw,nosuid,relatime,size=1007920k,nr_inodes=251980,mode=755 0 0
devpts /dev/pts devpts rw,nosuid,noexec,relatime,gid=5,mode=620,ptmxmode=000 0 0
tmpfs /run tmpfs rw,nosuid,noexec,relatime,size=204116k,mode=755 0 0
/dev/sda1 / ext4 rw,relatime,data=ordered 0 0
securityfs /sys/kernel/security securityfs rw,nosuid,nodev,noexec,relatime 0 0
tmpfs /dev/shm tmpfs rw,nosuid,nodev 0 0
tmpfs /run/lock tmpfs rw,nosuid,nodev,noexec,relatime,size=5120k 0 0
tmpfs /sys/fs/cgroup tmpfs ro,nosuid,nodev,noexec,mode=755 0 0
cgroup /sys/fs/cgroup/unified cgroup2 rw,nosuid,nodev,noexec,relatime 0 0
cgroup /sys/fs/cgroup/systemd cgroup rw,nosuid,nodev,noexec,relatime,xattr,name=systemd 0 0
pstore /sys/fs/pstore pstore rw,nosuid,nodev,noexec,relatime 0 0
cgroup /sys/fs/cgroup4/memory cgroup rw,nosuid,nodev,noexec,relatime,memory 0 0
cgroup /sys/fs/cgroup/hugetlb cgroup rw,nosuid,nodev,noexec,relatime,hugetlb 0 0
cgroup /sys/fs/cgroup/devices cgroup rw,nosuid,nodev,noexec,relatime,devices 0 0
cgroup /sys/fs/cgroup/freezer cgroup rw,nosuid,nodev,noexec,relatime,freezer 0 0
cgroup /sys/fs/cgroup2/cpuset cgroup rw,nosuid,nodev,noexec,relatime,cpuset 0 0
cgroup /sys/fs/cgroup/net_cls,net_prio cgroup rw,nosuid,nodev,noexec,relatime,net_cls,net_prio 0 0
cgroup /sys/fs/cgroup/rdma cgroup rw,nosuid,nodev,noexec,relatime,rdma 0 0
cgroup /sys/fs/cgroup3/cpu,cpuacct cgroup rw,nosuid,nodev,noexec,relatime,cpu,cpuacct 0 0
cgroup /sys/fs/cgroup/perf_event cgroup rw,nosuid,nodev,noexec,relatime,perf_event 0 0
cgroup /sys/fs/cgroup1/pids cgroup rw,nosuid,nodev,noexec,relatime,pids 0 0
cgroup /sys/fs/cgroup5/blkio cgroup rw,nosuid,nodev,noexec,relatime,blkio 0 0
systemd-1 /proc/sys/fs/binfmt_misc autofs rw,relatime,fd=26,pgrp=1,timeout=0,minproto=5,maxproto=5,direct,pipe_ino=10963 0 0
hugetlbfs /dev/hugepages hugetlbfs rw,relatime,pagesize=2M 0 0
mqueue /dev/mqueue mqueue rw,relatime 0 0
debugfs /sys/kernel/debug debugfs rw,relatime 0 0
configfs /sys/kernel/config configfs rw,relatime 0 0`,
		"/custom/host/proc/123/cgroup": `4:pids:/system.slice/docker-ea06501e021b11a0d46a09de007b3d71bd6f37537cceabd2c3cbfa7f9b3da1ee.scope
3:cpuset:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0
2:cpu,cpuacct:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0
1:name=systemd:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0`,
	}
	expected := []cgroups.Subsystem{
		cgroups.NewPids("/sys/fs/cgroup1"),
		cgroups.NewCputset("/sys/fs/cgroup2"),
		cgroups.NewCpu("/sys/fs/cgroup3"),
		cgroups.NewCpuacct("/sys/fs/cgroup3"),
		cgroups.NewMemory("/sys/fs/cgroup4"),
		cgroups.NewBlkio("/sys/fs/cgroup5"),
	}

	cgroupInfo, err := cgroupPathsFetch("/custom/host",123,createFileOpenFnMock(filesMap))
	assert.NoError(t, err)

	actual, err := cgroupInfo.getHierarchyFn()()
	assert.NoError(t, err)

	assert.ElementsMatch(t, expected, actual)
}

func createFileOpenFnMock(filesMap map[string]string) func(string) (io.ReadCloser, error) {
	return func(filePath string) (io.ReadCloser, error) {
		if fileContent, ok := filesMap[filePath]; ok {
			return ioutil.NopCloser(strings.NewReader(fileContent)), nil
		}

		return nil, fmt.Errorf("file not found by path: %s", filePath)
	}
}
