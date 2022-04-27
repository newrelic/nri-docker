package raw

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type SystemCPUReader interface {
	ReadUsage() (uint64, error)
}

type PosixSystemCPUReader struct {
	statsFilePath string
}

var (
	ErrInvalidProcStatNumCPUFields = errors.New("invalid number of cpu fields")
	ErrInvalidProcStatFormat       = errors.New("invalid stat format. Error trying to parse the '/proc/stat' file")
)

const StatFileDefaultPath = "/proc/stat"

func NewPosixSystemCPUReader(statFilePath string) PosixSystemCPUReader {
	if statFilePath == "" {
		statFilePath = StatFileDefaultPath
	}
	return PosixSystemCPUReader{statsFilePath: statFilePath}
}

// ReadUsage returns the host system's cpu usage in
// nanoseconds. An error is returned if the format of the underlying
// file does not match.
//
// Uses /proc/stat defined by POSIX. Looks for the cpu
// statistics line and then sums up the first seven fields
// provided. See `man 5 proc` for details on specific field
// information.
func (r PosixSystemCPUReader) ReadUsage() (uint64, error) {
	f, err := os.Open(r.statsFilePath)
	if err != nil {
		return 0, err
	}

	bufReader := bufio.NewReaderSize(nil, 128)
	defer func() {
		bufReader.Reset(nil)
		f.Close()
	}()
	bufReader.Reset(f)

	for {
		line, err := bufReader.ReadString('\n')
		if err != nil {
			break
		}
		parts := strings.Fields(line)
		switch parts[0] {
		case "cpu":
			if len(parts) < 8 {
				return 0, ErrInvalidProcStatNumCPUFields
			}
			var totalClockTicks uint64
			for _, i := range parts[1:8] {
				v, err := strconv.ParseUint(i, 10, 64)
				if err != nil {
					return 0, fmt.Errorf("unable to convert value %s to int: %s", i, err)
				}
				totalClockTicks += v
			}
			return (totalClockTicks * nanoSecondsPerSecond) / 100, nil
		}
	}
	return 0, ErrInvalidProcStatFormat
}
