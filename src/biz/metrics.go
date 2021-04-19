// Package biz provides business-value metrics from system raw metrics
package biz

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/persist"

	"github.com/newrelic/nri-docker/src/raw"
)

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
	TotalReadCount  float64
	TotalWriteCount float64
	TotalReadBytes  float64
	TotalWriteBytes float64
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
}

// Memory metrics
type Memory struct {
	UsageBytes            uint64
	CacheUsageBytes       uint64
	RSSUsageBytes         uint64
	MemLimitBytes         uint64
	UsagePercent          float64 // Usage percent from the limit, if any
	KernelUsageBytes      uint64
	SwapUsageBytes        uint64
	SwapOnlyUsageBytes    uint64
	SwapLimitBytes        uint64
	SwapLimitUsagePercent float64
	SoftLimitBytes        uint64
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
	inspector          Inspector
	exitedContainerTTL time.Duration
}

// Inspector is the abstraction of the only method that we require from the docker go client
type Inspector interface {
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
}

// NewProcessor creates a MetricsFetcher from implementations of its required components
func NewProcessor(store persist.Storer, fetcher raw.Fetcher, inspector Inspector, exitedContainerTTL time.Duration) *MetricsFetcher {
	return &MetricsFetcher{
		store:              store,
		fetcher:            fetcher,
		inspector:          inspector,
		exitedContainerTTL: exitedContainerTTL,
	}
}

// ErrExitedContainerExpired is the error type used when exited containers have exceed the TTL that would allow the
// integration to keep reporting them.
type ErrExitedContainerExpired struct {
	s string
}

func (e *ErrExitedContainerExpired) Error() string {
	return e.s
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

	if mc.exitedContainerTTL != 0 && json.State != nil && strings.ToLower(json.State.Status) == "exited" {
		exitTimestamp, err := time.Parse(time.RFC3339Nano, json.State.FinishedAt)
		if err != nil {
			return metrics, fmt.Errorf("invalid finished_at timestamp for exited container %s: %s (%v)",
				containerID,
				json.State.FinishedAt,
				err,
			)
		}
		if time.Since(exitTimestamp) > mc.exitedContainerTTL {
			return metrics, &ErrExitedContainerExpired{
				fmt.Sprintf("container %s exited after TTL (%v), skipping", containerID, mc.exitedContainerTTL),
			}
		}
	}

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

func (mc *MetricsFetcher) cpu(metrics raw.Metrics, json *types.ContainerJSON) CPU {
	var previous struct {
		Time int64
		CPU  raw.CPU
	}
	// store current metrics to be the "previous" metrics in the next CPU sampling
	defer func() {
		previous.Time = metrics.Time.Unix()
		previous.CPU = metrics.CPU
		mc.store.Set(metrics.ContainerID, previous)
	}()

	cpu := CPU{}
	// TODO: if newrelic-infra is in a limited cpus container, this may report the number of cpus of the
	// newrelic-infra container if the container has no CPU quota
	cpu.LimitCores = float64(runtime.NumCPU())
	if json.HostConfig != nil && json.HostConfig.NanoCPUs != 0 {
		cpu.LimitCores = float64(json.HostConfig.NanoCPUs) / 1e9
	}

	// Reading previous CPU stats
	if _, err := mc.store.Get(metrics.ContainerID, &previous); err != nil {
		log.Debug("could not retrieve previous CPU stats for container %v: %v", metrics.ContainerID, err.Error())
		return cpu
	}

	// calculate the change for the cpu usage of the container in between readings
	durationNS := float64(metrics.Time.Sub(time.Unix(previous.Time, 0)).Nanoseconds())
	if durationNS <= 0 {
		return cpu
	}

	maxVal := float64(len(metrics.CPU.PercpuUsage) * 100)

	cpu.CPUPercent = cpuPercent(previous.CPU, metrics.CPU)

	userDelta := float64(metrics.CPU.UsageInUsermode - previous.CPU.UsageInUsermode)
	cpu.UserPercent = math.Min(maxVal, userDelta*100/durationNS)

	kernelDelta := float64(metrics.CPU.UsageInKernelmode - previous.CPU.UsageInKernelmode)
	cpu.KernelPercent = math.Min(maxVal, kernelDelta*100/durationNS)

	cpu.UsedCores = float64(metrics.CPU.TotalUsage-previous.CPU.TotalUsage) / durationNS

	cpu.ThrottlePeriods = metrics.CPU.ThrottledPeriods
	cpu.ThrottledTimeMS = float64(metrics.CPU.ThrottledTimeNS) / 1e9 // nanoseconds to second

	cpu.UsedCoresPercent = 100 * cpu.UsedCores / cpu.LimitCores

	cpu.Shares = metrics.CPU.Shares

	return cpu
}

func (mc *MetricsFetcher) memory(mem raw.Memory) Memory {
	memLimits := mem.UsageLimit
	// ridiculously large memory limits are set to 0 (no limit)
	if memLimits > math.MaxInt64/2 {
		memLimits = 0
	}

	usagePercent := float64(0)
	if memLimits > 0 {
		usagePercent = 100 * float64(mem.RSS) / float64(memLimits)
	}

	/* https://www.kernel.org/doc/Documentation/cgroup-v1/memory.txt
	For efficiency, as other kernel components, memory cgroup uses some optimization
	to avoid unnecessary cacheline false sharing. usage_in_bytes is affected by the
	method and doesn't show 'exact' value of memory (and swap) usage, it's a fuzz
	value for efficient access. (Of course, when necessary, it's synchronized.)
	If you want to know more exact memory usage, you should use RSS+CACHE(+SWAP)
	value in memory.stat(see 5.2).
	However, as the `docker stats` cli tool does, page cache is intentionally
	excluded to avoid misinterpretation of the output.
	Also mem.SwapUsage is parsed from memory.memsw.usage_in_bytes, which
	according to the documentation reports the sum of current memory usage
	plus swap space used by processes in the cgroup (in bytes). That's why
	Usage is subtracted from the Swap: to get the actual swap.
	*/
	var swapOnlyUsage uint64
	if mem.SwapUsage != 0 { // for systems that have swap disabled
		swapOnlyUsage = mem.SwapUsage - mem.FuzzUsage
	}
	swapUsage := mem.RSS + swapOnlyUsage

	swapLimit := mem.SwapLimit
	if mem.SwapLimit > math.MaxInt64/2 {
		swapLimit = 0
	}
	swapLimitUsagePercent := float64(0)
	if swapLimit > 0 {
		swapLimitUsagePercent = 100 * float64(swapUsage) / float64(swapLimit)
	}

	/* Dockers includes non-swap memory into the swap limit
	(https://docs.docker.com/config/containers/resource_constraints/#--memory-swap-details)
	For example running the following container
	docker run --memory-swap=400m --memory=300m --memory-reservation=250m stress stress-ng --vm 1 --vm-bytes 350m
	will generate the following memory metrics:
	"memoryCacheBytes": 0.3mb,
	"memoryResidentSizeBytes": 298.4 m,
	"memorySizeLimitBytes": 300 m,

	"memorySoftLimitBytes": 250 m,
	"memorySwapLimitBytes": 400 m,
	"memorySwapLimitUsagePercent": 88.27 %,
	"memorySwapUsageBytes": 354.83 m,
	"memorySwapOnlyUsageBytes": 54.83 m,

	"memoryUsageBytes": 298.4 m,
	"memoryUsageLimitPercent": 99.42 %,

	Reference:
	* Metrics with no swap reference in the name have no swap components
	* Metrics with swap reference have memory+swap unless the contrary is specified like in memorySwapOnlyUsageBytes
	*/

	softLimit := mem.SoftLimit
	if mem.SoftLimit > math.MaxInt64/2 {
		softLimit = 0
	}

	return Memory{
		MemLimitBytes:         memLimits,
		CacheUsageBytes:       mem.Cache,
		RSSUsageBytes:         mem.RSS,
		UsageBytes:            mem.RSS,
		UsagePercent:          usagePercent,
		KernelUsageBytes:      mem.KernelMemoryUsage,
		SwapUsageBytes:        swapUsage,
		SwapOnlyUsageBytes:    swapOnlyUsage,
		SwapLimitBytes:        swapLimit,
		SoftLimitBytes:        softLimit,
		SwapLimitUsagePercent: swapLimitUsagePercent,
	}
}

func (mc *MetricsFetcher) blkIO(blkio raw.Blkio) BlkIO {
	bio := BlkIO{}
	for _, svc := range blkio.IoServicedRecursive {
		if len(svc.Op) == 0 {
			continue
		}
		switch svc.Op[0] {
		case 'r', 'R':
			bio.TotalReadCount += float64(svc.Value)
		case 'w', 'W':
			bio.TotalWriteCount += float64(svc.Value)
		}
	}
	for _, bytes := range blkio.IoServiceBytesRecursive {
		if len(bytes.Op) == 0 {
			continue
		}
		switch bytes.Op[0] {
		case 'r', 'R':
			bio.TotalReadBytes += float64(bytes.Value)
		case 'w', 'W':
			bio.TotalWriteBytes += float64(bytes.Value)
		}
	}
	return bio
}

func cpuPercent(previous, current raw.CPU) float64 {
	var (
		cpuPercent = 0.0
		// calculate the change for the cpu usage of the container in between readings
		cpuDelta = float64(current.TotalUsage - previous.TotalUsage)
		// calculate the change for the entire system between readings
		systemDelta = float64(current.SystemUsage - previous.SystemUsage)
		onlineCPUs  = float64(len(current.PercpuUsage))
	)

	if systemDelta > 0.0 && cpuDelta > 0.0 {
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100.0
	}
	return cpuPercent
}
