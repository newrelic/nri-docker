package raw

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// NewCgroupsFetcher creates the proper metrics fetcher for the used cgroups version.
func NewCgroupsFetcher(
	hostRoot string,
	cgroupInfo *CgroupInfo,
	systemCPUReader SystemCPUReader,
	networkStatsGetter NetworkStatsGetter,
) (Fetcher, error) {
	if cgroupInfo.Version == CgroupV2 {
		return NewCgroupsV2Fetcher(hostRoot, cgroupInfo.Driver, NewCgroupV2PathParser(), systemCPUReader, networkStatsGetter)
	}

	return NewCgroupsV1Fetcher(hostRoot, NewCgroupV1PathParser(), systemCPUReader, networkStatsGetter)
}

func countCpusetCPUsFromPath(path string) (uint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	cpusetInfo := strings.TrimSpace(string(data))
	return countCpusetCPUs(cpusetInfo)
}

// countCpusetCPUs returns the number of CPUs given a cpuset.cpu information.
// See <https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html#cpuset-interface-files> for format details.
// Example: "0-4,8,10,12-16"
func countCpusetCPUs(cpusetInfo string) (uint, error) {
	var numCPUs uint
	if cpusetInfo == "" {
		return 0, errors.New("empty cpuset info")
	}
	intervals := strings.Split(cpusetInfo, ",")
	for _, interval := range intervals {
		limits := strings.Split(interval, "-")
		switch len(limits) {
		case 1: // one element, Eg: "1"
			if _, err := strconv.Atoi(limits[0]); err != nil {
				return 0, fmt.Errorf("invalid %q cpuset format: %s", cpusetInfo, err)
			}
			numCPUs++
		case 2: // proper interval, Eg: "0-4"
			lowerLimit, err := strconv.Atoi(limits[0])
			if err != nil {
				return 0, fmt.Errorf("invalid %q cpuset format: %s", cpusetInfo, err)
			}
			upperLimit, err := strconv.Atoi(limits[1])
			if err != nil {
				return 0, fmt.Errorf("invalid %q cpuset format: %s", cpusetInfo, err)
			}
			if lowerLimit >= upperLimit {
				return 0, fmt.Errorf("invalid %q cpuset format: invalid interval %s", cpusetInfo, interval)
			}
			numCPUs += uint(upperLimit - lowerLimit)
		default:
			return 0, fmt.Errorf("invalid %q cpuset format", cpusetInfo)
		}
	}
	return numCPUs, nil
}
