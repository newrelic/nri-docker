package stats

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	path2 "path"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/persist"
)

const (
	cgroupPath = "/sys/fs/cgroup"
)

type CGroupsProvider struct {
	store persist.Storer
}

func NewCGroupsProvider() (*CGroupsProvider, error) {
	store, err := persist.NewFileStore( // TODO: make the following options configurable
		persist.DefaultPath("container_cpus"),
		log.NewStdErr(true),
		60*time.Second)

	return &CGroupsProvider{store: store}, err
}

func (cg *CGroupsProvider) PersistStats() error {
	return cg.store.Save()
}

func parseUintFile(file string) (value uint64, err error) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Scan()
	if err = scanner.Err(); err != nil {
		return
	}

	return strconv.ParseUint(scanner.Text(), 10, 64)
}

func (cg *CGroupsProvider) Fetch(containerID string) (Cooked, error) {
	stats := types.Stats{}

	stats.Read = time.Now()
	if err := cg.readPidsStats(containerID, &stats.PidsStats); err != nil {
		return Cooked(stats), err
	}

	if err := cg.readBlkioStats(containerID, &stats.BlkioStats); err != nil {
		return Cooked(stats), err
	}

	if err := cg.readCPUStats(containerID, &stats.CPUStats); err != nil {
		return Cooked(stats), err
	}

	if err := cg.readMemoryStats(containerID, &stats.MemoryStats); err != nil {
		return Cooked(stats), err
	}
	var preStats struct {
		UnixTime int64
		CPUStats types.CPUStats
	}
	// Reading previous CPU stats
	if _, err := cg.store.Get(containerID, &preStats); err == nil {
		stats.PreRead = time.Unix(preStats.UnixTime, 0)
		stats.PreCPUStats = preStats.CPUStats
	}
	// Storing current CPU stats for the next execution
	preStats.UnixTime = stats.Read.Unix()
	preStats.CPUStats = stats.CPUStats
	_ = cg.store.Set(containerID, preStats)

	return Cooked(stats), nil
}

func (cg *CGroupsProvider) readPidsStats(containerID string, stats *types.PidsStats) (err error) {
	path := path2.Join(cgroupPath, "pids", "docker", containerID)

	stats.Current, err = parseUintFile(path2.Join(path, "pids.current"))
	if err != nil {
		return err
	}

	body, err := ioutil.ReadFile(path2.Join(path, "/pids.max"))
	if err != nil {
		return err
	}
	value := string(body)
	if value == "max\n" {
		stats.Limit = 0
	} else {
		stats.Limit, err = strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cg *CGroupsProvider) readBlkio(path string, ioStat string) ([]types.BlkioStatEntry, error) {
	entries := []types.BlkioStatEntry{}

	f, err := os.Open(path2.Join(path, ioStat))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.FieldsFunc(sc.Text(), func(r rune) bool {
			return r == ' ' || r == ':'
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

func (cg *CGroupsProvider) readBlkioStats(containerID string, stats *types.BlkioStats) (err error) {
	path := path2.Join(cgroupPath, "blkio", "docker", containerID)

	if stats.IoMergedRecursive, err = cg.readBlkio(path, "blkio.io_merged_recursive"); err != nil {
		return err
	}

	if stats.IoServiceBytesRecursive, err = cg.readBlkio(path, "blkio.io_service_bytes_recursive"); err != nil {
		return err
	}

	if stats.IoServicedRecursive, err = cg.readBlkio(path, "blkio.io_serviced_recursive"); err != nil {
		return err
	}

	if stats.IoQueuedRecursive, err = cg.readBlkio(path, "blkio.io_queued_recursive"); err != nil {
		return err
	}

	if stats.IoServiceTimeRecursive, err = cg.readBlkio(path, "blkio.io_service_time_recursive"); err != nil {
		return err
	}

	if stats.IoWaitTimeRecursive, err = cg.readBlkio(path, "blkio.io_wait_time_recursive"); err != nil {
		return err
	}

	if stats.IoTimeRecursive, err = cg.readBlkio(path, "blkio.time_recursive"); err != nil {
		return err
	}

	if stats.SectorsRecursive, err = cg.readBlkio(path, "blkio.sectors_recursive"); err != nil {
		return err
	}

	return nil
}

func (cg *CGroupsProvider) readCPUUsage(path string, cpu *types.CPUUsage) error {
	var err error
	cpu.TotalUsage, err = parseUintFile(path2.Join(path, "cpuacct.usage"))
	if err != nil {
		return err
	}

	f, err := os.Open(path2.Join(path, "cpuacct.stat"))
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Split(sc.Text(), " ")
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return err
		}

		switch fields[0] {
		case "user":
			cpu.UsageInUsermode = value * uint64(time.Millisecond)
		case "system":
			cpu.UsageInKernelmode = value * uint64(time.Millisecond)
		}
	}

	body, err := ioutil.ReadFile(path2.Join(path, "cpuacct.usage_percpu"))
	if err != nil {
		return err
	}
	fields := strings.Split(string(body), " ")
	cpu.PercpuUsage = make([]uint64, len(fields))
	for i, num := range fields {
		if num == "\n" {
			continue
		}
		value, err := strconv.ParseUint(num, 10, 64)
		if err != nil {
			return err
		}
		cpu.PercpuUsage[i] = value
	}

	return nil
}

func (cg *CGroupsProvider) readCPUStats(containerID string, stats *types.CPUStats) (err error) {
	path := path2.Join(cgroupPath, "cpu", "docker", containerID)

	if err := cg.readCPUUsage(path, &stats.CPUUsage); err != nil {
		return err
	}
	return nil
}

func (cg *CGroupsProvider) readMemoryStats(containerID string, stats *types.MemoryStats) (err error) {
	path := path2.Join(cgroupPath, "memory", "docker", containerID)

	if stats.Usage, err = parseUintFile(path2.Join(path, "memory.usage_in_bytes")); err != nil {
		return err
	}

	if stats.MaxUsage, err = parseUintFile(path2.Join(path, "memory.max_usage_in_bytes")); err != nil {
		return err
	}

	if stats.Limit, err = parseUintFile(path2.Join(path, "memory.limit_in_bytes")); err != nil {
		return err
	}

	if stats.Failcnt, err = parseUintFile(path2.Join(path, "memory.failcnt")); err != nil {
		return err
	}

	stats.Stats = map[string]uint64{}

	f, err := os.Open(path2.Join(path, "memory.stat"))
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Split(sc.Text(), " ")
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return err
		}

		stats.Stats[fields[0]] = value
	}

	return nil
}
