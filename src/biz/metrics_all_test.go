// Copyright 2025 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package biz

import (
	"reflect"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/newrelic/infra-integrations-sdk/v3/persist"
	"github.com/newrelic/nri-docker/src/raw"
	"github.com/newrelic/nri-docker/src/utils"
	"github.com/stretchr/testify/assert"
)

func TestMetricsFetcher_CPU_LimitCores(t *testing.T) {
	type args struct {
		cpu  raw.Metrics
		json *container.InspectResponse
	}

	tests := []struct {
		name               string
		args               args
		runtimeCPUMockFunc func() int // In order to avoid flaky test we use this mocked to simulate runtime.CPU call.
		want               float64
	}{
		{
			name: "LimitCores honors cpu quota",
			args: args{
				cpu: raw.Metrics{
					ContainerID: "test-container",
					CPU: raw.CPU{
						OnlineCPUs: 4,
					},
				},
				json: &container.InspectResponse{
					HostConfig: &container.HostConfig{
						Resources: container.Resources{
							NanoCPUs: 500000000,
						},
					},
				},
			},
			runtimeCPUMockFunc: func() int {
				return 2
			},
			want: 0.5,
		},
		{
			name: "LimitCores set to default runtime.NumCPU() when no CPU quota set",
			args: args{
				cpu: raw.Metrics{
					CPU: raw.CPU{},
				},
				json: &container.InspectResponse{
					HostConfig: &container.HostConfig{},
				},
			},
			runtimeCPUMockFunc: func() int {
				return 2
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := &MetricsFetcher{
				store:            persist.NewInMemoryStore(),
				getRuntimeNumCPU: tt.runtimeCPUMockFunc,
			}

			got := mc.cpu(tt.args.cpu, tt.args.json)

			assert.Equal(t, tt.want, got.LimitCores)
		})
	}
}

func TestMetricsFetcher_CPU_CounterReset(t *testing.T) {
	now := time.Now().Truncate(time.Second) // the store keeps the previous time in whole seconds
	mc := &MetricsFetcher{
		store:            persist.NewInMemoryStore(),
		getRuntimeNumCPU: func() int { return 2 },
	}
	json := &container.InspectResponse{HostConfig: &container.HostConfig{}}
	sample := func(at time.Time, total, system uint64) raw.Metrics {
		return raw.Metrics{
			ContainerID: "test-container",
			Time:        at,
			CPU: raw.CPU{
				TotalUsage:        total,
				SystemUsage:       system,
				OnlineCPUs:        2,
				UsageInUsermode:   utils.ToPointer(total),
				UsageInKernelmode: utils.ToPointer(uint64(0)),
			},
		}
	}

	// first sample, then a regular one
	mc.cpu(sample(now, 1_000_000_000_000, 100), json)
	got := mc.cpu(sample(now.Add(time.Second), 1_500_000_000_000, 200), json)
	assert.Greater(t, got.UsedCores, 0.0)

	// container restarted: counters are lower than the stored ones
	logs := captureLogs(t)
	got = mc.cpu(sample(now.Add(2*time.Second), 1_000, 300), json)
	assert.Contains(t, logs.String(), "total CPU usage went backwards")
	assert.Equal(t, 0.0, got.UsedCores)
	assert.Equal(t, 0.0, got.UsedCoresPercent)
	assert.Equal(t, 0.0, got.CPUPercent)
	assert.Equal(t, 0.0, got.UserPercent)
	assert.Equal(t, 0.0, got.KernelPercent)
	assert.Equal(t, 2.0, got.LimitCores)

	// the next sample uses the post-restart sample as baseline and is sane again
	got = mc.cpu(sample(now.Add(3*time.Second), 1_000_000_000+1_000, 400), json)
	assert.InDelta(t, 1.0, got.UsedCores, 0.001)
	assert.InDelta(t, 50.0, got.UsedCoresPercent, 0.1)
}

// After a restart not every counter lands below its old value
// The sample must still be all-zero rather than mixing baselines from different process lifetimes.
func TestMetricsFetcher_CPU_CounterResetKernelRises(t *testing.T) {
	now := time.Now().Truncate(time.Second) // the store keeps the previous time in whole seconds
	mc := &MetricsFetcher{
		store:            persist.NewInMemoryStore(),
		getRuntimeNumCPU: func() int { return 2 },
	}
	json := &container.InspectResponse{HostConfig: &container.HostConfig{}}
	sample := func(at time.Time, user, kernel, system uint64) raw.Metrics {
		return raw.Metrics{
			ContainerID: "test-container",
			Time:        at,
			CPU: raw.CPU{
				TotalUsage:        user + kernel,
				SystemUsage:       system,
				OnlineCPUs:        2,
				UsageInUsermode:   utils.ToPointer(user),
				UsageInKernelmode: utils.ToPointer(kernel),
			},
		}
	}

	mc.cpu(sample(now, 3_000_000_000, 1_000_000_000, 100), json)

	got := mc.cpu(sample(now.Add(time.Second), 1_500_000_000, 1_200_000_000, 200), json)
	assert.Equal(t, 0.0, got.UsedCores)
	assert.Equal(t, 0.0, got.UsedCoresPercent)
	assert.Equal(t, 0.0, got.CPUPercent)
	assert.Equal(t, 0.0, got.UserPercent)
	assert.Equal(t, 0.0, got.KernelPercent)
	assert.Equal(t, 2.0, got.LimitCores)

	got = mc.cpu(sample(now.Add(2*time.Second), 2_500_000_000, 1_700_000_000, 300), json)
	assert.InDelta(t, 1.5, got.UsedCores, 0.001)
	assert.InDelta(t, 100.0, got.UserPercent, 0.001)
	assert.InDelta(t, 50.0, got.KernelPercent, 0.001)
}

// swap tests are not applicable for windows
// nolint: funlen
func TestMetricsFetcher_memory(t *testing.T) {
	type fields struct {
		store              persist.Storer
		fetcher            raw.Fetcher
		inspector          raw.DockerInspector
		exitedContainerTTL time.Duration
	}
	type args struct {
		mem raw.Memory
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   Memory
	}{
		{
			// docker run --memory-swap=400m --memory=300m --memory-reservation=250m stress stress-ng --vm 1 --vm-bytes 350m
			name: "swap metrics",
			args: args{
				raw.Memory{
					UsageLimit:        314572800,
					Cache:             339968,
					RSS:               312569856,
					SwapUsage:         utils.ToPointer(uint64(371634176)),
					FuzzUsage:         314449920,
					KernelMemoryUsage: 1626112,
					SwapLimit:         419430400,
					SoftLimit:         262144000,
				},
			},
			want: Memory{
				UsageBytes:            312569856,
				CacheUsageBytes:       339968,
				RSSUsageBytes:         312569856,
				MemLimitBytes:         314572800,
				UsagePercent:          99.36328125,
				KernelUsageBytes:      1626112,
				SwapUsageBytes:        utils.ToPointer(uint64(369754112)),
				SwapOnlyUsageBytes:    utils.ToPointer(uint64(57184256)),
				SwapLimitBytes:        419430400,
				SwapLimitUsagePercent: utils.ToPointer(float64(88.15625)),
				SoftLimitBytes:        262144000,
			},
		},
		{
			// docker run stress stress-ng --vm 1 --vm-bytes 100m
			name: "no swap",
			args: args{
				raw.Memory{
					UsageLimit:        9223372036854771712,
					Cache:             7839744,
					RSS:               104759296,
					SwapUsage:         utils.ToPointer(uint64(115326976)),
					FuzzUsage:         115326976,
					KernelMemoryUsage: 1830912,
					SwapLimit:         9223372036854771712,
					SoftLimit:         9223372036854771712,
				},
			},
			want: Memory{
				UsageBytes:            104759296,
				CacheUsageBytes:       7839744,
				RSSUsageBytes:         104759296,
				MemLimitBytes:         0,
				UsagePercent:          0.0,
				KernelUsageBytes:      1830912,
				SwapUsageBytes:        utils.ToPointer(uint64(104759296)),
				SwapOnlyUsageBytes:    utils.ToPointer(uint64(0)),
				SwapLimitBytes:        0,
				SwapLimitUsagePercent: utils.ToPointer(float64(0.0)),
				SoftLimitBytes:        0,
			},
		},
		{
			// docker run stress stress-ng --vm 1 --vm-bytes 100m
			name: "swap smaller then Fuzz",
			args: args{
				raw.Memory{
					UsageLimit:        9223372036854771712,
					Cache:             7839744,
					RSS:               104759296,
					SwapUsage:         utils.ToPointer(uint64(16)),
					FuzzUsage:         115326976,
					KernelMemoryUsage: 1830912,
					SwapLimit:         9223372036854771712,
					SoftLimit:         9223372036854771712,
				},
			},
			want: Memory{
				UsageBytes:            104759296,
				CacheUsageBytes:       7839744,
				RSSUsageBytes:         104759296,
				MemLimitBytes:         0,
				UsagePercent:          0.0,
				KernelUsageBytes:      1830912,
				SwapUsageBytes:        nil,
				SwapOnlyUsageBytes:    nil,
				SwapLimitBytes:        0,
				SwapLimitUsagePercent: nil,
				SoftLimitBytes:        0,
			},
		},
		{
			name: "no swap reported",
			args: args{
				raw.Memory{
					UsageLimit:        9223372036854771712,
					Cache:             7839744,
					RSS:               104759296,
					SwapUsage:         nil, // we assume this is not reported
					FuzzUsage:         115326976,
					KernelMemoryUsage: 1830912,
					SwapLimit:         9223372036854771712,
					SoftLimit:         9223372036854771712,
				},
			},
			want: Memory{
				UsageBytes:       104759296,
				CacheUsageBytes:  7839744,
				RSSUsageBytes:    104759296,
				MemLimitBytes:    0,
				UsagePercent:     0.0,
				KernelUsageBytes: 1830912,
				SwapLimitBytes:   0,
				SoftLimitBytes:   0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := &MetricsFetcher{
				store:              tt.fields.store,
				fetcher:            tt.fields.fetcher,
				inspector:          tt.fields.inspector,
				exitedContainerTTL: tt.fields.exitedContainerTTL,
			}
			if got := mc.memory(tt.args.mem, &container.InspectResponse{}); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MetricsFetcher.memory() = %v, want %v", got, tt.want)
			}
		})
	}
}

// nolint: stylecheck
func TestCpuPercent(t *testing.T) {
	var (
		previousTotalUsage  uint64 = 10
		previousSystemUsage uint64 = 20
		currentTotalUsage   uint64 = previousTotalUsage + 1
		currentSystemUsage  uint64 = previousSystemUsage + 10
		onlineCPUs          uint   = 2
	)

	cases := []struct {
		Name              string
		Previous, Current raw.CPU
		Expected          float64
	}{
		{
			Name:     "No system usage changes",
			Previous: raw.CPU{TotalUsage: previousTotalUsage, SystemUsage: previousSystemUsage},
			Current:  raw.CPU{TotalUsage: currentTotalUsage, SystemUsage: previousSystemUsage, OnlineCPUs: onlineCPUs},
			Expected: 0,
		},
		{
			Name:     "No total usage changes",
			Previous: raw.CPU{TotalUsage: previousTotalUsage, SystemUsage: previousSystemUsage},
			Current:  raw.CPU{TotalUsage: previousTotalUsage, SystemUsage: currentSystemUsage, OnlineCPUs: onlineCPUs},
			Expected: 0,
		},
		{
			Name:     "System and total usage changes",
			Previous: raw.CPU{TotalUsage: previousTotalUsage, SystemUsage: previousSystemUsage},
			Current:  raw.CPU{TotalUsage: currentTotalUsage, SystemUsage: currentSystemUsage, OnlineCPUs: onlineCPUs},
			Expected: 20,
		},
		{
			Name:     "Fallback to PercpuUsage having 2 used CPUs but 5 positions in the array and onlineCPUs not defined",
			Previous: raw.CPU{TotalUsage: previousTotalUsage, SystemUsage: previousSystemUsage},
			Current:  raw.CPU{TotalUsage: currentTotalUsage, SystemUsage: currentSystemUsage, PercpuUsage: []uint64{10, 10, 0, 0, 0}},
			Expected: 20,
		},
		{
			Name:     "Total usage counter reset (container restart)",
			Previous: raw.CPU{TotalUsage: 1000, SystemUsage: previousSystemUsage},
			Current:  raw.CPU{TotalUsage: 5, SystemUsage: currentSystemUsage, OnlineCPUs: onlineCPUs},
			Expected: 0,
		},
		{
			Name:     "System usage counter went backwards",
			Previous: raw.CPU{TotalUsage: previousTotalUsage, SystemUsage: 1000},
			Current:  raw.CPU{TotalUsage: currentTotalUsage, SystemUsage: 5, OnlineCPUs: onlineCPUs},
			Expected: 0,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			v := cpuPercent(c.Previous, c.Current)
			assert.Equal(t, c.Expected, v)
		})
	}
}
