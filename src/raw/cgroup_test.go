package raw

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectCgroupPath(t *testing.T) {
	//mounts := []*mount{
	//	{
	//		Device:     "sysfs",
	//		MountPoint: "/sys",
	//		FSType:     "sysfs",
	//		Options:    "rw,nosuid,nodev,noexec,relatime",
	//	},
	//	{
	//		Device:     "cgroup",
	//		MountPoint: "/sys/fs/cgroup/unified",
	//		FSType:     "cgroup2",
	//		Options:    "rw,nosuid,nodev,noexec,relatime",
	//	},
	//}
	//
	//result, found := detectCgroupPathFromMounts(mounts[:1])
	//assert.False(t, found)
	//assert.Empty(t, result)
	//
	//result, found = detectCgroupPathFromMounts(mounts[1:])
	//assert.True(t, found)
	//assert.Equal(t, "/sys/fs/cgroup", result)
}

// parse one file into cgroup info obj

func TestParseCgroupMountPoints(t *testing.T) {

	mountInfoFileContains := `tmpfs /dev/shm tmpfs rw,nosuid,nodev 0 0
tmpfs /run/lock tmpfs rw,nosuid,nodev,noexec,relatime,size=5120k 0 0
tmpfs /sys/fs/cgroup tmpfs ro,nosuid,nodev,noexec,mode=755 0 0
cgroup /sys/fs/cgroup/systemd cgroup rw,nosuid,nodev,noexec,relatime,xattr,name=systemd 0 0
cgroup /sys/fs/cgroup/cpu,cpuacct cgroup rw,nosuid,nodev,noexec,relatime,cpu,cpuacct 0 0`
	mountFileInfo := strings.NewReader(mountInfoFileContains)

	expected := map[string]string{
		"cpu": "/sys/fs/cgroup",
		"systemd": "/sys/fs/cgroup",
		"cpuacct": "/sys/fs/cgroup",
	}

	actual, err := parseCgroupMountPoints(mountFileInfo)
	assert.NoError(t, err)

	assert.Equal(t, expected, actual)

	//mountPointCPU, err := cgroupInfo.GetMountPoint(cgroups.Cpu)
	//assert.NoError(t, err)
	//assert.Equal(t, "/sys/fs/cgroup", mountPointCPU)
	//
	//mountPointSystemD, err := cgroupInfo.GetMountPoint(cgroups.SystemdDbus)
	//assert.NoError(t, err)
	//assert.Equal(t, "/sys/fs/cgroup", mountPointSystemD)
	//
	//pathCPU, err := cgroupInfo.GetPath(cgroups.Cpu)
	//assert.NoError(t, err)
	//assert.Equal(t, "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0", pathCPU)
	//
	//pathSystemD, err := cgroupInfo.GetPath(cgroups.SystemdDbus)
	//assert.NoError(t, err)
	//assert.Equal(t, "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0", pathSystemD)
	//
	//fullPathCPU, err := cgroupInfo.GetFullPath(cgroups.Cpu)
	//assert.NoError(t, err)
	//assert.Equal(t, "/sys/fs/cgroup/cpu/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0", fullPathCPU)
	//
	//fullPathSystemD, err := cgroupInfo.GetFullPath(cgroups.SystemdDbus)
	//assert.NoError(t, err)
	//assert.Equal(t, "/sys/fs/cgroup/systemd/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0", fullPathSystemD)
}

func TestParseCgroupPaths(t *testing.T) {
		cgroupFileContains := `4:pids:/system.slice/docker-ea06501e021b11a0d46a09de007b3d71bd6f37537cceabd2c3cbfa7f9b3da1ee.scope
	3:cpuset:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0
	2:cpu,cpuacct:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0
	1:name=systemd:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0`
		cgroupPaths := strings.NewReader(cgroupFileContains)

	expected := map[string]string{
		"pids": "/system.slice/docker-ea06501e021b11a0d46a09de007b3d71bd6f37537cceabd2c3cbfa7f9b3da1ee.scope",
		"cpuset": "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0",
		"cpu": "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0",
		"cpuacct": "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0",
		"name=systemd": "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0",
	}

	actual, err := parseCgroupPaths(cgroupPaths)
	assert.NoError(t, err)

	assert.Equal(t, expected, actual)

}