package raw

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
	"strings"
	"testing"
)

func TestGetEnv(t *testing.T) {
	result := getEnv("NRIA_NO_VAR", "/procs")
	assert.Equal(t, result, "/procs")

	result = getEnv("NRIA_NO_VAR", "/procs", "/test")
	assert.Equal(t, result, "/procs/test")

	result = getEnv("NRIA_NO_VAR", "/procs", "/test", "/test2")
	assert.Equal(t, result, "/procs/test/test2")

	os.Setenv("NRIA_NO_VAR", "/diff_path")
	result = getEnv("NRIA_NO_VAR", "/procs", "/test")
	assert.Equal(t, result, "/diff_path/test")
}

func TestGetFirstExistingNonEmptyPath(t *testing.T) {
	actual, found := getFirstExistingNonEmptyPath([]string{
		"zzzzzttttteeest",
	})
	assert.Equal(t, "", actual)
	assert.False(t, found)

	tempDir := os.TempDir() + "/TestGetFirstExistingPath"
	err := os.MkdirAll(tempDir, os.ModePerm)
	assert.NoError(t, err)

	f, err := ioutil.TempFile(tempDir, "prefix")
	assert.NoError(t, err)

	defer func() {
		err := os.Remove(f.Name())
		assert.NoError(t, err)
		err = os.Remove(tempDir)
		assert.NoError(t, err)
	}()

	actual, found = getFirstExistingNonEmptyPath([]string{
		"zzzzzttttteeest",
		tempDir,
	})

	assert.Equal(t, tempDir, actual)
	assert.Equal(t, found, true)
}

var mountsFileContent = `sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0
tmpfs /sys/fs/cgroup tmpfs ro,nosuid,nodev,noexec,mode=755 0 0
cgroup /sys/fs/cgroup/unified cgroup2 rw,nosuid,nodev,noexec,relatime 0 0
:/Users/cciutea/workspace/nr/gotest/src /go/src fuse.sshfs rw,nosuid,nodev,relatime,user_id=0,group_id=0,allow_other 0 0
`

var expectedMounts = []*mount{
	{
		Device:     "sysfs",
		MountPoint: "/sys",
		FSType:     "sysfs",
		Options:    "rw,nosuid,nodev,noexec,relatime",
	},
	{
		Device:     "tmpfs",
		MountPoint: "/sys/fs/cgroup",
		FSType:     "tmpfs",
		Options:    "ro,nosuid,nodev,noexec,mode=755",
	},
	{
		Device:     "cgroup",
		MountPoint: "/sys/fs/cgroup/unified",
		FSType:     "cgroup2",
		Options:    "rw,nosuid,nodev,noexec,relatime",
	},
	{
		Device:     ":/Users/cciutea/workspace/nr/gotest/src",
		MountPoint: "/go/src",
		FSType:     "fuse.sshfs",
		Options:    "rw,nosuid,nodev,relatime,user_id=0,group_id=0,allow_other",
	},
}

func TestGetMounts(t *testing.T) {
	file := strings.NewReader(mountsFileContent)

	mounts, err := getMounts(file)
	assert.NoError(t, err)

	assert.Equal(t, len(expectedMounts), len(mounts), "unexpected result array size")

	for i, expectedMount := range expectedMounts {
		assert.Equal(t, *expectedMount, *mounts[i], "actual different than expected")
	}
}
