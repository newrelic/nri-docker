package raw

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCgroupV2GroupPath(t *testing.T) {
	cases := []struct {
		Driver        string
		ContainerID   string
		Expected      string
		ExpectedError error
	}{
		{
			Driver:      "systemd",
			ContainerID: "<long-id>",
			Expected:    "/docker-<long-id>.scope",
		},
		{
			Driver:      "cgroupfs",
			ContainerID: "<long-id>",
			Expected:    "/<long-id>",
		},
		{
			Driver:        "invalid-driver",
			ExpectedError: errors.New(`invalid cgroup2 driver "invalid-driver"`),
		},
	}

	for _, c := range cases {
		t.Run("With driver "+c.Driver, func(t *testing.T) {
			path, err := cgroupV2GroupPath(c.Driver, c.ContainerID)
			assert.Equal(t, c.ExpectedError, err)
			assert.Equal(t, c.Expected, path)
		})
	}
}

func TestGetCgroupV2MountPoint(t *testing.T) {
	cgroup2MountfileContent := `sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
tmpfs /run tmpfs rw,nosuid,nodev,noexec,relatime,size=99460k,mode=755,inode64 0 0
/dev/mapper/ubuntu--vg-ubuntu--lv / ext4 rw,relatime 0 0
cgroup2 /sys/fs/cgroup cgroup2 rw,nosuid,nodev,noexec,relatime,nsdelegate,memory_recursiveprot 0 0
pstore /sys/fs/pstore pstore rw,nosuid,nodev,noexec,relatime 0 0
vagrant /vagrant vboxsf rw,nodev,relatime,iocharset=utf8,uid=1000,gid=1000 0 0
tmpfs /run/user/1000 tmpfs rw,nosuid,nodev,relatime,size=99456k,nr_inodes=24864,mode=700,uid=1000,gid=1000,inode64 0 0`

	cgroup2MountfileContentCustomHostRoot := `sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
tmpfs /run tmpfs rw,nosuid,nodev,noexec,relatime,size=99460k,mode=755,inode64 0 0
/dev/mapper/ubuntu--vg-ubuntu--lv / ext4 rw,relatime 0 0
cgroup2 /custom/host/sys/fs/cgroup cgroup2 rw,nosuid,nodev,noexec,relatime,nsdelegate,memory_recursiveprot 0 0
pstore /sys/fs/pstore pstore rw,nosuid,nodev,noexec,relatime 0 0
vagrant /vagrant vboxsf rw,nodev,relatime,iocharset=utf8,uid=1000,gid=1000 0 0
tmpfs /run/user/1000 tmpfs rw,nosuid,nodev,relatime,size=99456k,nr_inodes=24864,mode=700,uid=1000,gid=1000,inode64 0 0`

	cgroup2PathsFileContent := "0::/system.slice/docker.service\n"

	cases := []struct {
		Name         string
		HostRoot     string
		Pid          int
		FilesContent map[string]string
		Expected     string
	}{
		{
			Name:     "Empty hostRoot",
			Pid:      42,
			HostRoot: "",
			FilesContent: map[string]string{
				"/proc/mounts":    cgroup2MountfileContent,
				"/proc/42/cgroup": cgroup2PathsFileContent,
			},
			Expected: "/sys/fs/cgroup/system.slice/docker.service",
		},
		{
			Name:     "HostRoot as /",
			Pid:      42,
			HostRoot: "/",
			FilesContent: map[string]string{
				"/proc/mounts":    cgroup2MountfileContent,
				"/proc/42/cgroup": cgroup2PathsFileContent,
			},
			Expected: "/sys/fs/cgroup/system.slice/docker.service",
		},
		{
			Name:     "HostRoot as /custom/host",
			Pid:      42,
			HostRoot: "/custom/host",
			FilesContent: map[string]string{
				"/custom/host/proc/mounts":    cgroup2MountfileContentCustomHostRoot,
				"/custom/host/proc/42/cgroup": cgroup2PathsFileContent,
			},
			Expected: "/custom/host/sys/fs/cgroup/system.slice/docker.service",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			fn := createFileOpenFnMock(c.FilesContent)
			mountPoint, err := cgroupV2MountPoint(c.HostRoot, c.Pid, fn)
			require.NoError(t, err)
			assert.Equal(t, c.Expected, mountPoint)
		})
	}
}

func TestGetCgroupV2MountPointErrors(t *testing.T) {
	cgroup2MountfileContentNoCgroup2 := `sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
tmpfs /run tmpfs rw,nosuid,nodev,noexec,relatime,size=99460k,mode=755,inode64 0 0
/dev/mapper/ubuntu--vg-ubuntu--lv / ext4 rw,relatime 0 0
pstore /sys/fs/pstore pstore rw,nosuid,nodev,noexec,relatime 0 0
vagrant /vagrant vboxsf rw,nodev,relatime,iocharset=utf8,uid=1000,gid=1000 0 0
tmpfs /run/user/1000 tmpfs rw,nosuid,nodev,relatime,size=99456k,nr_inodes=24864,mode=700,uid=1000,gid=1000,inode64 0 0`

	cgroup2MountfileContentNoMatchingHostRoot := `sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
tmpfs /run tmpfs rw,nosuid,nodev,noexec,relatime,size=99460k,mode=755,inode64 0 0
/dev/mapper/ubuntu--vg-ubuntu--lv / ext4 rw,relatime 0 0
cgroup2 /sys/fs/cgroup cgroup2 rw,nosuid,nodev,noexec,relatime,nsdelegate,memory_recursiveprot 0 0
pstore /sys/fs/pstore pstore rw,nosuid,nodev,noexec,relatime 0 0
vagrant /vagrant vboxsf rw,nodev,relatime,iocharset=utf8,uid=1000,gid=1000 0 0
tmpfs /run/user/1000 tmpfs rw,nosuid,nodev,relatime,size=99456k,nr_inodes=24864,mode=700,uid=1000,gid=1000,inode64 0 0`

	cgroup2PathsFileContent := "0::/system.slice/docker.service\n"

	cases := []struct {
		Name         string
		Pid          int
		HostRoot     string
		FilesContent map[string]string
	}{
		{
			Name:     "No cgroups2 in mounts file",
			Pid:      42,
			HostRoot: "",
			FilesContent: map[string]string{
				"/proc/mounts":    cgroup2MountfileContentNoCgroup2,
				"/proc/42/cgroup": cgroup2PathsFileContent,
			},
		},
		{
			Name:         "mountfs file not found",
			Pid:          42,
			HostRoot:     "",
			FilesContent: map[string]string{},
		},
		{
			Name:     "No matching custom hostRoot",
			Pid:      42,
			HostRoot: "/custom/host",
			FilesContent: map[string]string{
				"/custom/host/proc/mounts": cgroup2MountfileContentNoMatchingHostRoot,
				"/proc/42/cgroup":          cgroup2PathsFileContent,
			},
		},
		{
			Name:     "No matching custom hostRoot",
			Pid:      42,
			HostRoot: "/custom/host",
			FilesContent: map[string]string{
				"/custom/host/proc/mounts": cgroup2MountfileContentNoMatchingHostRoot,
			},
		},
		{
			Name:     "Paths file not found",
			Pid:      42,
			HostRoot: "/custom/host",
			FilesContent: map[string]string{
				"/proc/mounts": cgroup2MountfileContentNoMatchingHostRoot,
			},
		},
		{
			Name:     "Paths file with bad format",
			Pid:      42,
			HostRoot: "/custom/host",
			FilesContent: map[string]string{
				"/proc/mounts":    cgroup2MountfileContentNoMatchingHostRoot,
				"/proc/42/cgroup": "\n",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			fn := createFileOpenFnMock(c.FilesContent)
			_, err := cgroupV2MountPoint(c.HostRoot, c.Pid, fn)
			require.Error(t, err)
		})
	}
}
