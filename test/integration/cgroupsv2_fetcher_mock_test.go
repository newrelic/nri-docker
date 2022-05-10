package integration_test

import (
	"time"

	"github.com/docker/docker/api/types"
	"github.com/newrelic/nri-docker/src/raw"
)

// CgroupsFetcherV2Mock is a wrapper of CgroupsFetcher to mock:
// The cpu SystemUsage metrics got from /proc/stat
// the timestamp of the metric
type CgroupsFetcherV2Mock struct {
	cgroupsFetcher raw.Fetcher
	time           time.Time
	systemUsage    uint64
}

// NewCgroupsV2FetcherMock creates a new cgroups data fetcher.
func NewCgroupsV2FetcherMock(hostRoot string, time time.Time, systemUsage uint64) (*CgroupsFetcherV2Mock, error) {
	cgroupsFetcher, err := raw.NewCgroupsV2Fetcher(
		hostRoot,
		cgroupDriver,
		raw.NewCgroupV2PathParser(),
		raw.NewPosixSystemCPUReader(),
		raw.NetDevNetworkStatsGetter{},
	)
	if err != nil {
		return nil, err
	}

	return &CgroupsFetcherV2Mock{
		cgroupsFetcher: cgroupsFetcher,
		time:           time,
		systemUsage:    systemUsage,
	}, nil
}

// Fetch calls the wrapped fetcher and overrides the Time
func (cgf *CgroupsFetcherV2Mock) Fetch(c types.ContainerJSON) (raw.Metrics, error) {
	metrics, err := cgf.cgroupsFetcher.Fetch(c)
	if err != nil {
		return raw.Metrics{}, err
	}

	metrics.Time = cgf.time
	metrics.CPU.SystemUsage = cgf.systemUsage

	return metrics, nil
}
