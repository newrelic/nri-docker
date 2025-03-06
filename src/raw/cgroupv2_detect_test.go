//go:build linux

package raw

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitMountPointAndGroup(t *testing.T) {
	cases := []struct {
		fullPath          string
		group, mountPoint string
	}{
		{
			fullPath:   "/sys/fs/cgroup/system.slice/docker-c0bfac5c77c9f363977d5dace337293e91efae8995b5a5b84287702a8d377c0e.scope",
			mountPoint: "/sys/fs/cgroup/system.slice",
			group:      "/docker-c0bfac5c77c9f363977d5dace337293e91efae8995b5a5b84287702a8d377c0e.scope",
		},
		{
			fullPath:   "/sys/fs/cgroup/docker/c0bfac5c77c9f363977d5dace337293e91efae8995b5a5b84287702a8d377c0e",
			mountPoint: "/sys/fs/cgroup/docker",
			group:      "/c0bfac5c77c9f363977d5dace337293e91efae8995b5a5b84287702a8d377c0e",
		},
	}

	cgroupDetector := NewCgroupV2PathParser()

	for _, c := range cases {
		t.Run("check "+c.fullPath, func(t *testing.T) {
			mountpoint, group := cgroupDetector.splitMountPointAndGroup(c.fullPath)
			assert.Equal(t, c.mountPoint, mountpoint)
			assert.Equal(t, c.group, group)
		})
	}
}

func TestGetCgroupV2FullPath(t *testing.T) {
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

	cgroupDetector := NewCgroupV2PathParser()

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			cgroupDetector.openFn = createFileOpenFnMock(c.FilesContent)
			mountPoint, err := cgroupDetector.cgroupV2FullPath(c.HostRoot, c.Pid)
			require.NoError(t, err)
			assert.Equal(t, c.Expected, mountPoint)
		})
	}
}

func TestGetCgroupV2FullPathErrors(t *testing.T) {
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
	cgroup2PathsFileContentWrongFormat := "1::/system.slice/docker.service\n"

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
			HostRoot: "",
			FilesContent: map[string]string{
				"/proc/mounts": cgroup2MountfileContentNoMatchingHostRoot,
			},
		},
		{
			Name:     "Paths file with bad format",
			Pid:      42,
			HostRoot: "",
			FilesContent: map[string]string{
				"/proc/mounts":    cgroup2MountfileContentNoMatchingHostRoot,
				"/proc/42/cgroup": "\n",
			},
		},
		{
			Name:     "Paths file with bad format",
			Pid:      42,
			HostRoot: "",
			FilesContent: map[string]string{
				"/proc/mounts":    cgroup2MountfileContentNoMatchingHostRoot,
				"/proc/42/cgroup": cgroup2PathsFileContentWrongFormat,
			},
		},
	}

	cgroupDetector := NewCgroupV2PathParser()

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			cgroupDetector.openFn = createFileOpenFnMock(c.FilesContent)
			_, err := cgroupDetector.cgroupV2FullPath(c.HostRoot, c.Pid)
			require.Error(t, err)
		})
	}
}
