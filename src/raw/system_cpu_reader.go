//go:build linux

package raw

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type SystemCPUReader interface {
	ReadUsage() (uint64, error)
}

type PosixSystemCPUReader struct {
	openFn       fileOpenFn
	statFilePath string
}

var (
	ErrInvalidProcStatNumCPUFields = errors.New("invalid number of cpu fields")
	ErrInvalidProcStatFormat       = errors.New("invalid stat format. Error trying to parse the '/proc/stat' file")
)

const statFilePath = "/proc/stat"

func NewPosixSystemCPUReader() PosixSystemCPUReader {
	return PosixSystemCPUReader{openFn: defaultFileOpenFn, statFilePath: statFilePath}
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
	f, err := r.openFn(r.statFilePath)
	if err != nil {
		return 0, err
	}

	defer func() {
		f.Close()
	}()

	return r.parseAndGetUsage(f)
}

func (r PosixSystemCPUReader) parseAndGetUsage(f io.ReadCloser) (uint64, error) {
	bufReader := bufio.NewReaderSize(nil, 128)
	defer func() {
		bufReader.Reset(nil)
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
