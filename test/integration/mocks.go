package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/system"
	"github.com/newrelic/nri-docker/src/raw"
	"github.com/stretchr/testify/mock"
)

const cgroupDriver = "systemd"

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
	cgroupsFetcher, err := raw.NewCgroupsV2Fetcher(hostRoot, cgroupDriver, raw.NewPosixSystemCPUReader())
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

// CgroupsFetcherMock is a wrapper of CgroupsFetcher to mock:
// The cpu SystemUsage metrics got from /proc/stat
// the timestamp of the metric
type CgroupsFetcherMock struct {
	cgroupsFetcher raw.Fetcher
	time           time.Time
	systemUsage    uint64
}

// NewCgroupsFetcherMock creates a new cgroups data fetcher.
func NewCgroupsFetcherMock(hostRoot string, time time.Time, systemUsage uint64) (*CgroupsFetcherMock, error) {
	cgroupsFetcher, err := raw.NewCgroupsV1Fetcher(hostRoot, NewSystemCPUReaderMock(systemUsage))
	if err != nil {
		return nil, err
	}

	return &CgroupsFetcherMock{
		cgroupsFetcher: cgroupsFetcher,
		time:           time,
		systemUsage:    systemUsage,
	}, nil
}

// Fetch calls the wrapped fetcher and overrides the Time
func (cgf *CgroupsFetcherMock) Fetch(c types.ContainerJSON) (raw.Metrics, error) {
	metrics, err := cgf.cgroupsFetcher.Fetch(c)
	if err != nil {
		return raw.Metrics{}, err
	}

	metrics.Time = cgf.time
	return metrics, nil
}

type SystemCPUReaderMock struct {
	systemUsage uint64
}

func NewSystemCPUReaderMock(systemUsage uint64) *SystemCPUReaderMock {
	return &SystemCPUReaderMock{
		systemUsage: systemUsage,
	}
}

func (s *SystemCPUReaderMock) ReadUsage() (uint64, error) {
	return s.systemUsage, nil
}

type InspectorMock struct {
	containerID  string
	pid          int
	restartCount int
}

func NewInspectorMock(containerID string, pid, restartCount int) InspectorMock {
	return InspectorMock{
		containerID:  containerID,
		pid:          pid,
		restartCount: restartCount,
	}
}

func (i InspectorMock) ContainerInspect(_ context.Context, _ string) (types.ContainerJSON, error) {
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID: i.containerID,
			State: &types.ContainerState{
				Pid: i.pid,
			},
			RestartCount: i.restartCount,
		},
	}, nil
}

func (i InspectorMock) Info(_ context.Context) (system.Info, error) {
	return system.Info{}, nil
}

type mockDockerStatsClient struct {
	mock.Mock
}

func (m *mockDockerStatsClient) ContainerStats(ctx context.Context, containerID string, stream bool) (types.ContainerStats, error) {
	args := m.Called()

	statsJSON, _ := json.Marshal(args.Get(0).(types.StatsJSON))

	return types.ContainerStats{
		Body: io.NopCloser(bytes.NewReader(statsJSON)),
	}, nil
}
