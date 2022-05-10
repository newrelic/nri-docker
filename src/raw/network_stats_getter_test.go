package raw

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNetDevNetworkStatsGetter_network(t *testing.T) {
	filesMap := map[string]string{
		"/proc/666/net/dev": `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
  eth0:    3402      28   10    2    0     0          0         0        5       4    3    2    0     0       0          0`,
		"/custom/host/proc/123/cgroup": `4:pids:/system.slice/docker-ea06501e021b11a0d46a09de007b3d71bd6f37537cceabd2c3cbfa7f9b3da1ee.scope
3:cpuset:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0
2:cpu,cpuacct:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0
1:name=systemd:/docker/f7bd95ecd8dc9deb33491d044567db18f537fd9cf26613527ff5f636e7d9bdb0`,
	}

	testCases := []struct {
		name          string
		statFile      map[string]string
		errorExpected error
		expected      uint64
	}{
		{
			name:          "When the path is incorrect Then an error is returned",
			statFile:      filesMap,
			errorExpected: syscall.ENOENT,
			expected:      0,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			netDevStatsGetter := NetDevNetworkStatsGetter{openFn: createFileOpenFnMock(tt.statFile)}
			network, err := netDevStatsGetter.GetForContainer("a-host-root", "a-pid", "a-container")
			require.NoError(t, err)
			require.Equal(t, tt.expected, network)
		})
	}
}
