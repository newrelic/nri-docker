package raw

import (
	"time"
)

type Metrics struct {
	Time        time.Time
	ContainerID string
	Memory      Memory
	Network     Network
	CPU         CPU
	Pids        Pids
	Blkio       Blkio
}

type Memory struct {
	UsageLimit uint64
	Cache      uint64
	RSS        uint64
	SwapUsage  uint64
	FuzzUsage  uint64
}

type CPU struct {
	TotalUsage        uint64
	UsageInUsermode   uint64
	UsageInKernelmode uint64
	PercpuUsage       []uint64
	ThrottledPeriods  uint64
	ThrottledTimeNS   uint64
	SystemUsage       uint64
	OnlineCPUs        uint
}

type Pids struct {
	Current uint64
	Limit   uint64
}

type Blkio struct {
	IoServiceBytesRecursive []BlkioEntries
	IoServicedRecursive     []BlkioEntries
}

type BlkioEntries struct {
	Op    string
	Value uint64
}

type Network struct {
	RxBytes   int64
	RxDropped int64
	RxErrors  int64
	RxPackets int64
	TxBytes   int64
	TxDropped int64
	TxErrors  int64
	TxPackets int64
}

type MetricsFetcher struct {
	cgroups *cgroupsFetcher
	network *networkFetcher
}

type Fetcher interface {
	Fetch(containerID string, containerPID int) (Metrics, error)
}

func NewFetcher(hostRoot string) MetricsFetcher {
	return MetricsFetcher{
		cgroups: newCGroupsFetcher(hostRoot),
		network: newNetworkFetcher(hostRoot),
	}
}

func (mf *MetricsFetcher) Fetch(containerID string, containerPID int) (Metrics, error) {
	metrics, err := mf.cgroups.fetch(containerID)
	if err != nil {
		return metrics, err
	}
	metrics.ContainerID = containerID
	metrics.Network, err = mf.network.Fetch(containerPID)
	return metrics, err
}
