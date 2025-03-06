// Copyright 2025 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package nri

import "github.com/newrelic/nri-docker/src/biz"

func memory(mem *biz.Memory) []entry {
	metrics := []entry{
		metricMemoryCacheBytes(mem.CacheUsageBytes),
		metricMemoryUsageBytes(mem.UsageBytes),
		metricMemoryResidentSizeBytes(mem.RSSUsageBytes),
		metricMemoryKernelUsageBytes(mem.KernelUsageBytes),
	}
	if mem.MemLimitBytes > 0 {
		metrics = append(metrics,
			metricMemorySizeLimitBytes(mem.MemLimitBytes),
			metricMemoryUsageLimitPercent(mem.UsagePercent),
		)
	}
	if mem.SoftLimitBytes > 0 {
		metrics = append(metrics, metricMemorySoftLimitBytes(mem.SoftLimitBytes))
	}
	if mem.SwapLimitBytes > 0 {
		metrics = append(metrics, metricMemorySwapLimitBytes(mem.SwapLimitBytes))
	}
	if mem.SwapLimitBytes > 0 && mem.SwapLimitUsagePercent != nil {
		metrics = append(metrics, metricMemorySwapLimitUsagePercent(*mem.SwapLimitUsagePercent))
	}
	if mem.SwapUsageBytes != nil {
		metrics = append(metrics, metricMemorySwapUsageBytes(*mem.SwapUsageBytes))
	}
	if mem.SwapOnlyUsageBytes != nil {
		metrics = append(metrics, metricMemorySwapOnlyUsageBytes(*mem.SwapOnlyUsageBytes))
	}
	return metrics
}

func cpu(cpu *biz.CPU) []entry {
	return []entry{
		metricCPUUsedCores(cpu.UsedCores),
		metricCPUUsedCoresPercent(cpu.UsedCoresPercent),
		metricCPULimitCores(cpu.LimitCores),
		metricCPUPercent(cpu.CPUPercent),
		metricCPUKernelPercent(cpu.KernelPercent),
		metricCPUUserPercent(cpu.UserPercent),
		metricCPUThrottlePeriods(cpu.ThrottlePeriods),
		metricCPUThrottleTimeMS(cpu.ThrottledTimeMS),
		metricCPUShares(cpu.Shares),
	}
}

func blkio(bio *biz.BlkIO) []entry {
	var entries []entry

	if bio.TotalReadCount != nil {
		entries = append(entries, metricIOTotalReadCount(*bio.TotalReadCount), metricIOReadCountPerSecond(*bio.TotalReadCount))
	}
	if bio.TotalWriteCount != nil {
		entries = append(entries, metricIOTotalWriteCount(*bio.TotalWriteCount), metricIOWriteCountPerSecond(*bio.TotalWriteCount))
	}
	if bio.TotalReadBytes != nil {
		entries = append(entries, metricIOTotalReadBytes(*bio.TotalReadBytes), metricIOReadBytesPerSecond(*bio.TotalReadBytes))
	}
	if bio.TotalWriteBytes != nil {
		entries = append(entries, metricIOTotalWriteBytes(*bio.TotalWriteBytes), metricIOWriteBytesPerSecond(*bio.TotalWriteBytes))
	}

	if bio.TotalReadBytes != nil || bio.TotalWriteBytes != nil {
		totalBytes := 0.0
		if bio.TotalReadBytes != nil {
			totalBytes += *bio.TotalReadBytes
		}
		if bio.TotalWriteBytes != nil {
			totalBytes += *bio.TotalWriteBytes
		}
		entries = append(entries, metricIOTotalBytes(totalBytes))
	}

	return entries
}
