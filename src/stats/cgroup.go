package stats

import (
	"bufio"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"C"
	"time"
)

const (
	cgroupPath = "/sys/fs/cgroup"
	nanoSecondsPerSecond = 1e9
)

type CGroupsProvider struct {
	docker *client.Client
}

func NewCGroupsProvider(docker *client.Client) *CGroupsProvider {
	return &CGroupsProvider{docker: docker}
}

func (cg *CGroupsProvider) Fetch(containerID string) (Cooked, error) {
	stats := types.Stats{}

	if err := cg.readPidsStats(containerID, &stats); err != nil {
		return Cooked(stats), err;
	}

	if err := cg.readBlkioStats(containerID, &stats); err != nil {
		return Cooked(stats), err;
	}

	if err := cg.readCPUStats(containerID, "cpu/docker", &stats.CPUStats); err != nil {
		return Cooked(stats), err;
	}

	return Cooked(stats), nil
}

func (cg *CGroupsProvider) readPidsStats(containerID string, stats *types.Stats) error {
	path := fmt.Sprintf("%s/%s/%s", cgroupPath, "pids/docker", containerID);

	body, err := ioutil.ReadFile(path + "/pids.current")
	if err != nil {
		return err;
	}
	stats.PidsStats.Current, err = strconv.ParseUint(string(body), 10, 64)
	if err != nil {
		return err;
	}

	body, err = ioutil.ReadFile(path + "/pids.max")
	if err != nil {
		return err;
	}
	value := string(body)
	if value == "max" {
		stats.PidsStats.Limit = 0;
	} else {
		stats.PidsStats.Limit, err = strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err;
		}
	}

	return nil
}

func (cg *CGroupsProvider) readBlkio(path string, ioStat string) ([]types.BlkioStatEntry, error) {
	entries := []types.BlkioStatEntry{}

	f, err := os.Open(fmt.Sprintf("%s/%s", path, ioStat));
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields :=  strings.FieldsFunc(sc.Text(), func(r rune) bool {
			return r == ' ' || r == ':';
		})
		if len(fields) < 3 {
			if len(fields) == 2 && fields[0] == "Total" {
				// skip total line
				continue
			} else {
				return nil, fmt.Errorf("Invalid line found while parsing %s: %s", path, sc.Text())
			}
		}

		v, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return nil, err
		}
		major := v

		v, err = strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, err
		}
		minor := v

		op := ""
		valueField := 2
		if len(fields) == 4 {
			op = fields[2]
			valueField = 3
		}
		v, err = strconv.ParseUint(fields[valueField], 10, 64)
		if err != nil {
			return nil, err
		}
		entries = append(entries, types.BlkioStatEntry{Major: major, Minor: minor, Op: op, Value: v})
	}

	return entries, nil
}

func (cg *CGroupsProvider) readBlkioStats(containerID string, stats *types.Stats) (err error) {
	path := fmt.Sprintf("%s/%s/%s", cgroupPath, "blkio/docker", containerID);

	if stats.BlkioStats.IoMergedRecursive, err = cg.readBlkio(path, "io_merged_recursive"); err != nil {
		return err;
	}

	if stats.BlkioStats.IoServiceBytesRecursive, err = cg.readBlkio(path, "io_service_bytes_recursive"); err != nil {
		return err;
	}

	if stats.BlkioStats.IoServicedRecursive, err = cg.readBlkio(path, "io_serviced_recursive"); err != nil {
		return err;
	}

	if stats.BlkioStats.IoQueuedRecursive, err = cg.readBlkio(path, "io_queued_recursive"); err != nil {
		return err;
	}

	if stats.BlkioStats.IoServiceTimeRecursive, err = cg.readBlkio(path, "io_service_time_recursive"); err != nil {
		return err;
	}

	if stats.BlkioStats.IoWaitTimeRecursive, err = cg.readBlkio(path, "io_wait_time_recursive"); err != nil {
		return err;
	}

	if stats.BlkioStats.IoTimeRecursive, err = cg.readBlkio(path, "time_recursive"); err != nil {
		return err;
	}

	if stats.BlkioStats.SectorsRecursive, err = cg.readBlkio(path, "sectors_recursive"); err != nil {
		return err;
	}

	return nil
}

func (cg *CGroupsProvider) readCPUUsage(path string, cpu *types.CPUUsage) error {

	body, err := ioutil.ReadFile(path + "/cpuacct.usage")
	if err != nil {
		return err;
	}
	cpu.TotalUsage, err = strconv.ParseUint(string(body), 10, 64)
	if err != nil {
		return err;
	}

	f, err := os.Open(path + "/cpuacct.stat");
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Split(sc.Text(), " ")
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return err;
		}

		if (fields[0] == "user") {
			cpu.UsageInUsermode = value * uint64(time.Millisecond)
		} else if (fields[1] == "system") {
			cpu.UsageInKernelmode = value * uint64(time.Millisecond)
		}
	}

	body, err = ioutil.ReadFile(path + "/cpuacct.usage_percpu")
	if err != nil {
		return err;
	}
	fields := strings.Split(string(body), " ")
	cpu.PercpuUsage = make([]uint64, len(fields));
	for i, num := range fields {
		value, err := strconv.ParseUint(num, 10, 64)
		if err != nil {
			return err;
		}
		cpu.PercpuUsage[i] = value
	}

	return nil
}

func GetClockTicks() uint64 {
	return uint64(C.sysconf(C._SC_CLK_TCK))
}

// getSystemCPUUsage returns the host system's cpu usage in
// nanoseconds. An error is returned if the format of the underlying
// file does not match.
//
// Uses /proc/stat defined by POSIX. Looks for the cpu
// statistics line and then sums up the first seven fields
// provided. See `man 5 proc` for details on specific field
// information.
func (cg *CGroupsProvider) getSystemCPUUsage() (uint64, error) {
	bufReader :=  bufio.NewReaderSize(nil, 128)
	var line string
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, err
	}
	defer func() {
		bufReader.Reset(nil)
		f.Close()
	}()
	bufReader.Reset(f)
	err = nil
	for err == nil {
		line, err = bufReader.ReadString('\n')
		if err != nil {
			break
		}
		parts := strings.Fields(line)
		switch parts[0] {
		case "cpu":
			if len(parts) < 8 {
				return 0, fmt.Errorf("Error bad cpu fields")
			}
			var totalClockTicks uint64
			for _, i := range parts[1:8] {
				v, err := strconv.ParseUint(i, 10, 64)
				if err != nil {
					return 0, err
				}
				totalClockTicks += v
			}
			return (totalClockTicks * nanoSecondsPerSecond) / GetClockTicks(), nil
		}
	}
	return 0, fmt.Errorf("error bad stat format")
}

func (cg *CGroupsProvider) readCPUStats(containerID string, path string, stats *types.CPUStats) (err error) {
	path = fmt.Sprintf("%s/%s/%s", cgroupPath, path, containerID);

	if err := cg.readCPUUsage(path, &stats.CPUUsage); err != nil {
		return err
	}

	if stats.SystemUsage, err = cg.getSystemCPUUsage(); err != nil {
		return err
	}

	stats.OnlineCPUs = uint32(len(stats.CPUUsage.PercpuUsage))
	// TODO: stats.ThrottlingData

	return nil
}


/*
type Stats struct {
	// Common stats
	Read    time.Time `json:"read"`
	PreRead time.Time `json:"preread"`


	// Shared stats
	CPUStats    CPUStats    `json:"cpu_stats,omitempty"`
	PreCPUStats CPUStats    `json:"precpu_stats,omitempty"` // "Pre"="Previous"
	MemoryStats MemoryStats `json:"memory_stats,omitempty"`
 */