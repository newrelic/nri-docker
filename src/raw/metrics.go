package raw

import (
	"time"

	"github.com/docker/docker/api/types"
)

const (
	blkioReadOp  = "Read"
	blkioWriteOp = "Write"
)

// Metrics holds containers raw metric values as they are extracted from the system
type Metrics struct {
	Time        time.Time
	ContainerID string
	Memory      Memory
	Network     Network
	CPU         CPU
	Pids        Pids
	Blkio       Blkio
}

// Memory usage snapshot
type Memory struct {
	UsageLimit        uint64
	Cache             uint64
	RSS               uint64
	SwapUsage         *uint64
	FuzzUsage         uint64
	KernelMemoryUsage uint64
	SwapLimit         uint64
	SoftLimit         uint64
}

// CPU usage snapshot
type CPU struct {
	TotalUsage        uint64
	UsageInUsermode   uint64
	UsageInKernelmode uint64
	PercpuUsage       []uint64
	ThrottledPeriods  uint64
	ThrottledTimeNS   uint64
	SystemUsage       uint64
	OnlineCPUs        uint
	Shares            uint64
}

// Pids inside the container
type Pids struct {
	Current uint64
	Limit   uint64
}

// Blkio stores multiple entries of the Block I/O stats
type Blkio struct {
	IoServiceBytesRecursive []BlkioEntry
	IoServicedRecursive     []BlkioEntry
}

// BlkioEntry stores basic information of a simple blkio operation
type BlkioEntry struct {
	Op    string
	Value uint64
}

// Network transmission and receive metrics
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

// Fetcher is the minimal abstraction of any raw metrics fetcher implementation
type Fetcher interface {
	Fetch(types.ContainerJSON) (Metrics, error)
}

// OnlineCPUsWithFallback gets onlineCPUs value falling back to percpuUsage length in case OnlineCPUs is not defined
func (c *CPU) OnlineCPUsWithFallback() int {
	if c.OnlineCPUs != 0 {
		return int(c.OnlineCPUs)
	}

	// Calculate OnlineCPUs from PercpuUsage by checking the positions with cpu usage higher than 0, because in some OS,
	// the array returned has more entries than the number of OnlineCpus (sometimes bigger than the available),
	// causing the length calculation to give a wrong value.
	var onlineCPUs int
	for _, cpuUsage := range c.PercpuUsage {
		if cpuUsage > 0 {
			onlineCPUs++
		}
	}
	return onlineCPUs
}
