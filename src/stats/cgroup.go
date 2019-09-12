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

// TODO: on int parse error, not return error, just ignore and continue other metrics
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

	if stats.IoServiceBytesRecursive, err = cg.readBlkio(path, "blkio.throttle.io_service_bytes"); err != nil {
		return err
	}

	if stats.IoServicedRecursive, err = cg.readBlkio(path, "blkio.throttle.io_serviced"); err != nil {
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
			cpu.UsageInUsermode = value * (uint64(time.Second) / 100) // uint64(C.sysconf(C._SC_CLK_TCK))
		case "system":
			cpu.UsageInKernelmode = value * (uint64(time.Second) / 100) // uint64(C.sysconf(C._SC_CLK_TCK))
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

func (cg *CGroupsProvider) readCPUStats(containerID string, stats *types.CPUStats) error {
	path := path2.Join(cgroupPath, "cpu", "docker", containerID)

	if err := cg.readCPUUsage(path, &stats.CPUUsage); err != nil {
		return err
	}
	return nil
}

func (cg *CGroupsProvider) readMemoryStats(containerID string, stats *types.MemoryStats) error {
	path := path2.Join(cgroupPath, "memory", "docker", containerID)

	for _, metric := range []struct {
		file string
		dest *uint64
	}{
		{"memory.max_usage_in_bytes", &stats.MaxUsage},
		{"memory.limit_in_bytes", &stats.Limit},
	} {
		var err error
		if *metric.dest, err = parseUintFile(path2.Join(path, metric.file)); err != nil {
			log.Debug("error reading %s: %s", metric.file, err.Error())
		}
	}

	stats.Stats = map[string]uint64{}

	f, err := os.Open(path2.Join(path, "memory.stat"))
	if err != nil {
		log.Debug("error reading memory.stat: %s", err.Error())
	}

	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Split(sc.Text(), " ")
		if len(fields) < 2 {
			log.Debug("%q has less than two fields", sc.Text())
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			log.Debug("error parsing memory.stat %q: %s", fields[0], err.Error())
			continue
		}

		stats.Stats[fields[0]] = value
	}

	/*
		Calculating usage instead of `memory.usage_in_bytes` file contents.
		https://www.kernel.org/doc/Documentation/cgroup-v1/memory.txt
		For efficiency, as other kernel components, memory cgroup uses some optimization
		to avoid unnecessary cacheline false sharing. usage_in_bytes is affected by the
		method and doesn't show 'exact' value of memory (and swap) usage, it's a fuzz
		value for efficient access. (Of course, when necessary, it's synchronized.)
		If you want to know more exact memory usage, you should use RSS+CACHE(+SWAP)
		value in memory.stat(see 5.2).
		However, as the `docker stats` cli tool does, page cache is intentionally
		excluded to avoid misinterpretation of the output.
	*/
	stats.Usage = stats.Stats["rss"] + stats.Stats["swap"]

	return nil
}
