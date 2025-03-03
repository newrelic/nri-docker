package aws

import (
	"time"

	"github.com/newrelic/infra-integrations-sdk/v3/log"
	"github.com/newrelic/nri-docker/src/raw"
)

func fargateRawMetrics(fargateStats FargateStats) map[string]*raw.Metrics {
	rawMetrics := make(map[string]*raw.Metrics, len(fargateStats))
	now := time.Now()

	for containerID, stats := range fargateStats {
		if stats == nil {
			log.Debug("did not find container stats for %s, skipping", containerID)
			continue
		}
		network := computeNetworkStats(stats)
		rawMetrics[containerID] = &raw.Metrics{
			Time:        now,
			ContainerID: containerID,
			Memory: raw.Memory{
				UsageLimit: stats.MemoryStats.Limit,
				Cache:      stats.MemoryStats.Stats["cache"],
				RSS:        stats.MemoryStats.Stats["rss"],
				FuzzUsage:  0,
			},
			Network: network,
			CPU: raw.CPU{
				TotalUsage:        stats.CPUStats.CPUUsage.TotalUsage,
				UsageInUsermode:   &stats.CPUStats.CPUUsage.UsageInUsermode,
				UsageInKernelmode: &stats.CPUStats.CPUUsage.UsageInKernelmode,
				PercpuUsage:       stats.CPUStats.CPUUsage.PercpuUsage,
				ThrottledPeriods:  stats.CPUStats.ThrottlingData.ThrottledPeriods,
				ThrottledTimeNS:   stats.CPUStats.ThrottlingData.ThrottledTime,
				SystemUsage:       stats.CPUStats.SystemUsage,
			},
			Pids: raw.Pids{
				Current: stats.PidsStats.Current,
				Limit:   stats.PidsStats.Limit,
			},
			Blkio: raw.Blkio{},
		}

		for _, s := range stats.BlkioStats.IoServiceBytesRecursive {
			entry := raw.BlkioEntry{Op: s.Op, Value: s.Value}
			rawMetrics[containerID].Blkio.IoServiceBytesRecursive = append(
				rawMetrics[containerID].Blkio.IoServiceBytesRecursive,
				entry,
			)
		}

		for _, s := range stats.BlkioStats.IoServicedRecursive {
			entry := raw.BlkioEntry{Op: s.Op, Value: s.Value}
			rawMetrics[containerID].Blkio.IoServicedRecursive = append(
				rawMetrics[containerID].Blkio.IoServicedRecursive,
				entry,
			)
		}
	}

	return rawMetrics
}

func computeNetworkStats(stats *timedDockerStats) raw.Network {
	n := raw.Network{}
	var RxBytes, RxDropped, RxErrors, RxPackets, TxBytes, TxDropped, TxErrors, TxPackets uint64
	if len(stats.Networks) > 0 {
		for _, n := range stats.Networks {
			RxBytes += n.RxBytes
			RxDropped += n.RxDropped
			RxErrors += n.RxErrors
			RxPackets += n.RxPackets
			TxBytes += n.TxBytes
			TxDropped += n.TxDropped
			TxErrors += n.TxErrors
			TxPackets += n.TxPackets
		}
		n = raw.Network{
			//Note that for big integers the cast is not safe possibly causing an overflow
			RxBytes:   int64(RxBytes),
			RxDropped: int64(RxDropped),
			RxErrors:  int64(RxErrors),
			RxPackets: int64(RxPackets),
			TxBytes:   int64(TxBytes),
			TxDropped: int64(TxDropped),
			TxErrors:  int64(TxErrors),
			TxPackets: int64(TxPackets),
		}
	}
	return n
}
