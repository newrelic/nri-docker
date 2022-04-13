package biz

import (
	"github.com/docker/docker/api/types"
	"github.com/newrelic/nri-docker/src/raw"
	"time"
)

var mockedTime = time.Date(2022, time.January, 1, 4, 3, 2, 0, time.UTC)

// CgroupsFetcherMock is a wrapper of CgroupsFetcher to mock:
// The cpuUsage metrics got from /proc/stat
// the timestamp of the metric
type CgroupsFetcherMock struct {
	cgroupsFetcher *raw.CgroupsFetcher
}

// NewCgroupsFetcherMock creates a new cgroups data fetcher.
func NewCgroupsFetcherMock(hostRoot, cgroupDriver, cgroupMountPoint string) (*CgroupsFetcherMock, error) {
	cgroupsFetcher, err := raw.NewCgroupsFetcher(hostRoot, cgroupDriver, cgroupMountPoint)
	if err != nil {
		return nil, err
	}

	return &CgroupsFetcherMock{cgroupsFetcher}, nil
}

// Fetch calls the wrapped fetcher and overrides the CPU.SystemUsage and the Time
func (cgf *CgroupsFetcherMock) Fetch(c types.ContainerJSON) (raw.Metrics, error) {
	metrics, err := cgf.cgroupsFetcher.Fetch(c)
	if err != nil {
		return raw.Metrics{}, err
	}

	metrics.CPU.SystemUsage = 19026130000000
	metrics.Time = mockedTime
	return metrics, nil
}
