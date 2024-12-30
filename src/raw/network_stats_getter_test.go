//go:build linux
// +build linux

package raw

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNetDevNetworkStatsGetter_network(t *testing.T) {
	filesMap := map[string]string{
		"/host-correct/proc/pid/net/dev": `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
  eth0:    3402      28   10    2    0     0          0         0        5       4    3    2    0     0       0          0`,
		"/host-not-correct/proc/pid/net/dev": `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame
    lo:       0       0    0    0    0     0
  eth0:    3402      28   10    2    0     0`,
	}

	testCases := []struct {
		name          string
		statFile      map[string]string
		hostRoot      string
		errorExpected bool
		expected      Network
	}{
		{
			name:          "When the file doesn't exist an error is returned",
			statFile:      filesMap,
			hostRoot:      "/host-not-exists",
			errorExpected: true,
			expected:      Network{},
		},
		{
			name:          "When the file is correct the Network struct is returned with expected values",
			statFile:      filesMap,
			hostRoot:      "/host-correct",
			errorExpected: false,
			expected:      Network{RxBytes: 3402, RxDropped: 2, RxErrors: 10, RxPackets: 28, TxBytes: 5, TxDropped: 2, TxErrors: 3, TxPackets: 4},
		},
		{
			name:          "When the file is not correct no error and an empty Network struct is returned",
			statFile:      filesMap,
			hostRoot:      "/host-not-correct",
			errorExpected: false,
			expected:      Network{},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			netDevStatsGetter := NetDevNetworkStatsGetter{openFn: createFileOpenFnMock(tt.statFile)}
			network, err := netDevStatsGetter.GetForContainer(tt.hostRoot, "pid", "a-container")
			require.Equal(t, tt.errorExpected, err != nil)
			require.Equal(t, tt.expected, network)
		})
	}
}
