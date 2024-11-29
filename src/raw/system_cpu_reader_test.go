// +build linux

package raw

import (
	"github.com/stretchr/testify/require"

	"testing"
)

func TestPosixSystemCPUReader_ReadUsage(t *testing.T) {
	filesMap := map[string]string{
		"/stat-incorrect-cpu": `cpu  10
cpu0 128760 1045 10194 865073 604 0 431 0 0 0
intr 5049109 36 12124 0 0 0 0 0 0 0 0 0 0 9244 0 0 10489 0 0 36090 6707 93564 57096 26 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
ctxt 9891383
btime 1650954735
processes 4776
procs_running 1
procs_blocked 0
softirq 2922584 4 2589888 1986 7408 58962 0 9245 0 0 255091
`,
		"/stat-no-cpu": `cpu0 128760 1045 10194 865073 604 0 431 0 0 0
intr 5049109 36 12124 0 0 0 0 0 0 0 0 0 0 9244 0 0 10489 0 0 36090 6707 93564 57096 26 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
ctxt 9891383
btime 1650954735
processes 4776
procs_running 1
procs_blocked 0
softirq 2922584 4 2589888 1986 7408 58962 0 9245 0 0 255091
`,
		"/stat": `cpu  10 10 10 10 10 10 10 0 0 0
cpu0 128760 1045 10194 865073 604 0 431 0 0 0
intr 5049109 36 12124 0 0 0 0 0 0 0 0 0 0 9244 0 0 10489 0 0 36090 6707 93564 57096 26 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
ctxt 9891383
btime 1650954735
processes 4776
procs_running 1
procs_blocked 0
softirq 2922584 4 2589888 1986 7408 58962 0 9245 0 0 255091`,
	}

	testCases := []struct {
		name          string
		statFile      map[string]string
		filePath      string
		errorExpected error
		expected      uint64
	}{
		{
			name:          "When the file has less than 8 cpu blocks Then an ErrInvalidProcStatNumCPUFields is returned",
			statFile:      filesMap,
			filePath:      "/stat-incorrect-cpu",
			errorExpected: ErrInvalidProcStatNumCPUFields,
			expected:      0,
		},
		{
			name:          "When the file has no cpu row Then an ErrInvalidProcStatFormat is returned",
			statFile:      filesMap,
			filePath:      "/stat-no-cpu",
			errorExpected: ErrInvalidProcStatFormat,
			expected:      0,
		},
		{
			name:          "When the stat file is correct then the returned CPUUsage should be as expected",
			statFile:      filesMap,
			filePath:      "/stat",
			errorExpected: nil,
			expected:      uint64(700000000),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cpuReader := PosixSystemCPUReader{openFn: createFileOpenFnMock(tt.statFile), statFilePath: tt.filePath}
			cpuUsage, err := cpuReader.ReadUsage()

			require.ErrorIs(t, err, tt.errorExpected)
			require.Equal(t, tt.expected, cpuUsage)
		})
	}
}
