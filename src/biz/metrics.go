// Package biz provides business-value metrics from system raw metrics
package biz

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/newrelic/infra-integrations-sdk/v3/log"
	"github.com/newrelic/infra-integrations-sdk/v3/persist"

	"github.com/newrelic/nri-docker/src/raw"
)

var ErrExitedContainerExpired = errors.New("container exited TTL expired")
var ErrExitedContainerUnexpired = errors.New("exited containers have no metrics to fetch")

type StoredCPUSample struct {
	Time int64
	CPU  raw.CPU
}

// Sample exports the valuable metrics from a container
type Sample struct {
	Pids         Pids
	Network      Network
	BlkIO        BlkIO
	CPU          CPU
	Memory       Memory
	RestartCount int
}

// Pids section of a container sample
type Pids raw.Pids

// Network section of a container sample
type Network raw.Network

// BlkIO stands for Block I/O stats
type BlkIO struct {
	TotalReadCount  *float64
	TotalWriteCount *float64
	TotalReadBytes  *float64
	TotalWriteBytes *float64
}

// CPU metrics
type CPU struct {
	CPUPercent       float64
	KernelPercent    float64
	UserPercent      float64
	UsedCores        float64
	LimitCores       float64
	UsedCoresPercent float64
	ThrottlePeriods  uint64
	ThrottledTimeMS  float64
	Shares           uint64
	NumProcs         uint32
}

// Memory metrics
type Memory struct {
	UsageBytes       uint64
	CacheUsageBytes  uint64
	RSSUsageBytes    uint64
	MemLimitBytes    uint64
	UsagePercent     float64 // Usage percent from the limit, if any
	KernelUsageBytes uint64
	SoftLimitBytes   uint64
	SwapLimitBytes   uint64

	// The swap metrics depending on swap usage are computed only if it is reported
	SwapUsageBytes        *uint64
	SwapOnlyUsageBytes    *uint64
	SwapLimitUsagePercent *float64

	// Windows specific metrics
	CommitBytes       uint64
	CommitPeakBytes   uint64
	PrivateWorkingSet uint64
}

// Processer defines the most essential interface of an exportable container Processer
type Processer interface {
	Process(containerID string) (Sample, error)
}

// MetricsFetcher fetches the container system-level metrics from different sources and processes it to export
// metrics with business-value
type MetricsFetcher struct {
	store              persist.Storer
	fetcher            raw.Fetcher
	inspector          raw.DockerInspector
	exitedContainerTTL time.Duration
	getRuntimeNumCPU   func() int
	platform           string
}

// NewProcessor creates a MetricsFetcher from implementations of its required components
func NewProcessor(store persist.Storer, fetcher raw.Fetcher, inspector raw.DockerInspector, exitedContainerTTL time.Duration) *MetricsFetcher {
	return &MetricsFetcher{
		store:              store,
		fetcher:            fetcher,
		inspector:          inspector,
		exitedContainerTTL: exitedContainerTTL,
		getRuntimeNumCPU:   runtime.NumCPU,
		platform:           runtime.GOOS,
	}
}

// WithRuntimeNumCPUfunc changes the NumCPU counting func so MetricsFetcher is mockable
func (mc *MetricsFetcher) WithRuntimeNumCPUfunc(rcFunc func() int) {
	mc.getRuntimeNumCPU = rcFunc
}

// Process returns a metrics Sample of the container with the given ID
func (mc *MetricsFetcher) Process(containerID string) (Sample, error) {
	metrics := Sample{}

	json, err := mc.inspector.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return metrics, err
	}
	if json.ContainerJSONBase == nil {
		return metrics, errors.New("empty container inspect result")
	}

	// TODO: move logic to skip container without State to Docker specific code.
	if json.State == nil {
		log.Debug("invalid container %s JSON: missing State", containerID)
	}

	if json.State != nil && strings.ToLower(json.State.Status) == "exited" {
		expired, err := mc.isExpired(json.State.FinishedAt) //nolint: govet //shadowed err
		if err != nil {
			return metrics, fmt.Errorf("verifying container expiration: %w", err)
		}
		if expired {
			return metrics, ErrExitedContainerExpired
		}

		// There are no metrics to fetch from an exited container.
		return metrics, ErrExitedContainerUnexpired
	}

	// Fetch metrics from non exited containers
	rawMetrics, err := mc.fetcher.Fetch(json)
	if err != nil {
		return metrics, err
	}

	metrics.Network = Network(rawMetrics.Network)
	metrics.BlkIO = mc.blkIO(rawMetrics.Blkio)
	metrics.CPU = mc.cpu(rawMetrics, &json)
	metrics.Pids = Pids(rawMetrics.Pids)
	metrics.Memory = mc.memory(rawMetrics.Memory)
	metrics.RestartCount = json.RestartCount

	return metrics, nil
}

func (mc *MetricsFetcher) isExpired(finishedAt string) (bool, error) {
	if mc.exitedContainerTTL == 0 {
		return false, nil
	}

	exitTimestamp, err := time.Parse(time.RFC3339Nano, finishedAt)
	if err != nil {
		return false, fmt.Errorf("invalid finished_at timestamp: %s (%w)", finishedAt, err)
	}

	if time.Since(exitTimestamp) > mc.exitedContainerTTL {
		return true, nil
	}

	return false, nil
}
