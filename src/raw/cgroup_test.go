package raw

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectCgroupPath(t *testing.T) {
	mounts := []*mount{
		{
			Device:     "sysfs",
			MountPoint: "/sys",
			FSType:     "sysfs",
			Options:    "rw,nosuid,nodev,noexec,relatime",
		},
		{
			Device:     "cgroup",
			MountPoint: "/sys/fs/cgroup/unified",
			FSType:     "cgroup2",
			Options:    "rw,nosuid,nodev,noexec,relatime",
		},
	}

	result, found := detectCgroupPathFromMounts(mounts[:1])
	assert.False(t, found)
	assert.Empty(t, result)

	result, found = detectCgroupPathFromMounts(mounts[1:])
	assert.True(t, found)
	assert.Equal(t, "/sys/fs/cgroup", result)
}
