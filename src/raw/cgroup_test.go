package raw

import (
	"github.com/containerd/cgroups"
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

func TestParseCgroupInfo(t *testing.T) {

	// file io.Reader

	mountInfoFileContains := `tmpfs /dev/shm tmpfs rw,nosuid,nodev 0 0
tmpfs /run/lock tmpfs rw,nosuid,nodev,noexec,relatime,size=5120k 0 0
tmpfs /sys/fs/cgroup tmpfs ro,nosuid,nodev,noexec,mode=755 0 0
cgroup /sys/fs/cgroup/unified cgroup2 rw,nosuid,nodev,noexec,relatime 0 0
cgroup /sys/fs/cgroup/systemd cgroup rw,nosuid,nodev,noexec,relatime,xattr,name=systemd 0 0
pstore /sys/fs/pstore pstore rw,nosuid,nodev,noexec,relatime 0 0
cgroup /sys/fs/cgroup/cpu,cpuacct cgroup rw,nosuid,nodev,noexec,relatime,cpu,cpuacct 0 0
cgroup /sys/fs/cgroup/hugetlb cgroup rw,nosuid,nodev,noexec,relatime,hugetlb 0 0`

	mountFileInfo := strings.NewReader(mountInfoFileContains)
	cgroupFileContains := `5:cpuset:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0
4:perf_event:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0
3:hugetlb:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0
2:cpu,cpuacct:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0
1:name=systemd:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0`
	cgroupFileInfo := strings.NewReader(cgroupFileContains)

	cgroupInfo, err := parseCgroupInfo(mountFileInfo, cgroupFileInfo)
	assert.NoError(t, err)

	assert.Equal(t, "/sys/fs/cgroup", cgroupInfo.GetMount(cgroups.Cpu))
	assert.Equal(t, "/sys/fs/cgroup", cgroupInfo.GetMount(cgroups.SystemdDbus))

	assert.Equal(t, "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0", cgroupInfo.GetPath(cgroups.Cpu))
	assert.Equal(t, "/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0", cgroupInfo.GetPath(cgroups.SystemdDbus))

	assert.Equal(t, "/sys/fs/cgroup/cpu/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0", cgroupInfo.GetFullPath(cgroups.Cpu))
	assert.Equal(t, "/sys/fs/cgroup/systemd/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0", cgroupInfo.GetFullPath(cgroups.SystemdDbus))
}
