package raw_test

import (
	"syscall"

	"github.com/newrelic/nri-docker/src/raw"
	"github.com/stretchr/testify/require"

	"testing"
)

func TestPosixSystemCPUReader_ReadUsage(t *testing.T) {
	testCases := []struct {
		name          string
		statFilePath  string
		errorExpected error
		expected      uint64
	}{
		{
			name:          "When the path is incorrect Then an error is returned",
			statFilePath:  "testdata/wrong",
			errorExpected: syscall.ENOENT,
			expected:      0,
		},
		{
			name:          "When the file has less than 8 cpu blocks Then an ErrInvalidProcStatNumCPUFields is returned",
			statFilePath:  "testdata/stat_incorrect_cpu",
			errorExpected: raw.ErrInvalidProcStatNumCPUFields,
			expected:      0,
		},
		{
			name:          "When the file has no cpu row Then an ErrInvalidProcStatFormat is returned",
			statFilePath:  "testdata/stat_no_cpu",
			errorExpected: raw.ErrInvalidProcStatFormat,
			expected:      0,
		},
		{
			name:          "When the stat file is correct then the returned CPUUsage should be as expected",
			statFilePath:  "testdata/stat",
			errorExpected: nil,
			expected:      uint64(700000000),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cpuReader := raw.NewPosixSystemCPUReader(raw.CPUReaderWithStatFilePath(tt.statFilePath))
			cpuUsage, err := cpuReader.ReadUsage()

			require.ErrorIs(t, err, tt.errorExpected)
			require.Equal(t, tt.expected, cpuUsage)
		})
	}
}
