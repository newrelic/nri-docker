package biz

import (
	"time"

	"github.com/docker/docker/api/types"
	"github.com/newrelic/nri-docker/src/raw"
)

// CgroupsFetcherMock is a wrapper of CgroupsFetcher to mock:
// The cpu SystemUsage metrics got from /proc/stat
// the timestamp of the metric
type CgroupsFetcherMock struct {
	cgroupsFetcher *raw.CgroupsFetcher
	time           time.Time
	systemUsage    uint64
}

// NewCgroupsFetcherMock creates a new cgroups data fetcher.
func NewCgroupsFetcherMock(hostRoot string, time time.Time, systemUsage uint64) (*CgroupsFetcherMock, error) {
	cgroupsFetcher, err := raw.NewCgroupsFetcher(hostRoot)
	if err != nil {
		return nil, err
	}

	return &CgroupsFetcherMock{
		cgroupsFetcher: cgroupsFetcher,
		time:           time,
		systemUsage:    systemUsage,
	}, nil
}

// Fetch calls the wrapped fetcher and overrides the CPU.SystemUsage and the Time
func (cgf *CgroupsFetcherMock) Fetch(c types.ContainerJSON) (raw.Metrics, error) {
	metrics, err := cgf.cgroupsFetcher.Fetch(c)
	if err != nil {
		return raw.Metrics{}, err
	}

	metrics.CPU.SystemUsage = cgf.systemUsage
	metrics.Time = cgf.time
	return metrics, nil
}
