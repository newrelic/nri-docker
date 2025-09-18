package biz

import (
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/newrelic/infra-integrations-sdk/v3/persist"
	"github.com/newrelic/nri-docker/src/raw"
	"github.com/newrelic/nri-docker/src/utils"
	"github.com/stretchr/testify/assert"
)

func TestCPU(t *testing.T) {
	var (
		readTime    = time.Time(time.Now())
		preReadTime = readTime.Add(-time.Second)
	)
	tests := []struct {
		name          string
		metrics       raw.Metrics
		storedMetrics StoredCPUSample
		want          CPU
	}{
		{
			name: "Test CPU for the first sample",
			metrics: raw.Metrics{
				Time: readTime,
				CPU: raw.CPU{
					TotalUsage: 10000000,
					Read:       readTime,
					PreRead:    preReadTime,
					NumProcs:   2,
				},
			},
			want: CPU{
				CPUPercent:    0,
				NumProcs:      2,
				LimitCores:    2,
				UserPercent:   0,
				KernelPercent: 0,
			},
		},
		{
			name: "Test CPU container only using CPU in user mode",
			metrics: raw.Metrics{
				Time:        readTime,
				ContainerID: "container123",
				CPU: raw.CPU{
					TotalUsage:        20000000,
					UsageInUsermode:   utils.ToPointer(uint64(20000000)),
					UsageInKernelmode: utils.ToPointer(uint64(0)),
					Read:              readTime,
					PreRead:           preReadTime,
					NumProcs:          2,
				},
			},
			storedMetrics: StoredCPUSample{
				Time: readTime.Add(-time.Second).Unix(),
				CPU: raw.CPU{
					TotalUsage:        10000000,
					UsageInUsermode:   utils.ToPointer(uint64(10000000)),
					UsageInKernelmode: utils.ToPointer(uint64(0)),
					Read:              readTime.Add(-time.Second),
					PreRead:           preReadTime.Add(-time.Second),
					NumProcs:          2,
				},
			},
			want: CPU{
				CPUPercent:    50,
				NumProcs:      2,
				LimitCores:    2,
				UserPercent:   100,
				KernelPercent: 0,
			},
		},
		{
			name: "Test CPU container only using CPU in kernel mode",
			metrics: raw.Metrics{
				Time:        readTime,
				ContainerID: "container123",
				CPU: raw.CPU{
					TotalUsage:        20000000,
					UsageInUsermode:   utils.ToPointer(uint64(0)),
					UsageInKernelmode: utils.ToPointer(uint64(20000000)),
					Read:              readTime,
					PreRead:           preReadTime,
					NumProcs:          2,
				},
			},
			storedMetrics: StoredCPUSample{
				Time: readTime.Add(-time.Second).Unix(),
				CPU: raw.CPU{
					TotalUsage:        10000000,
					UsageInUsermode:   utils.ToPointer(uint64(0)),
					UsageInKernelmode: utils.ToPointer(uint64(10000000)),
					Read:              readTime.Add(-time.Second),
					PreRead:           preReadTime.Add(-time.Second),
					NumProcs:          2,
				},
			},
			want: CPU{
				CPUPercent:    50,
				NumProcs:      2,
				LimitCores:    2,
				UserPercent:   0,
				KernelPercent: 100,
			},
		},
		{
			name: "Test CPU container using both user and kernel CPU",
			metrics: raw.Metrics{
				Time:        readTime,
				ContainerID: "container123",
				CPU: raw.CPU{
					TotalUsage:        20000000,
					UsageInUsermode:   utils.ToPointer(uint64(10000000)),
					UsageInKernelmode: utils.ToPointer(uint64(10000000)),
					Read:              readTime,
					PreRead:           preReadTime,
					NumProcs:          2,
				},
			},
			storedMetrics: StoredCPUSample{
				Time: readTime.Add(-time.Second).Unix(),
				CPU: raw.CPU{
					TotalUsage:        10000000,
					UsageInUsermode:   utils.ToPointer(uint64(5000000)),
					UsageInKernelmode: utils.ToPointer(uint64(5000000)),
					Read:              readTime.Add(-time.Second),
					PreRead:           preReadTime.Add(-time.Second),
					NumProcs:          2,
				},
			},
			want: CPU{
				CPUPercent:    50,
				NumProcs:      2,
				LimitCores:    2,
				UserPercent:   50,
				KernelPercent: 50,
			},
		},
		{
			name: "Test CPU running in Hyper-V isolation",
			metrics: raw.Metrics{
				Time:        readTime,
				ContainerID: "container123",
				CPU: raw.CPU{
					TotalUsage: 20000000,
					Read:       readTime,
					PreRead:    preReadTime,
					NumProcs:   2,
				},
			},
			storedMetrics: StoredCPUSample{
				Time: readTime.Add(-time.Second).Unix(),
				CPU: raw.CPU{
					TotalUsage: 10000000,
					Read:       readTime.Add(-time.Second),
					PreRead:    preReadTime.Add(-time.Second),
					NumProcs:   2,
				},
			},
			want: CPU{
				CPUPercent:    50,
				NumProcs:      2,
				LimitCores:    2,
				UserPercent:   0,
				KernelPercent: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := &MetricsFetcher{
				store: persist.NewInMemoryStore(),
			}
			mc.store.Set("container123", tt.storedMetrics)
			got := mc.cpu(tt.metrics, nil)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCpuPercent(t *testing.T) {
	var (
		readTime                  = time.Time(time.Now())
		preReadTime               = readTime.Add(-time.Second)
		previousTotalUsage uint64 = 10000000
		currentTotalUsage  uint64 = previousTotalUsage + 10000000
		numProcs           uint32 = 2
	)

	cases := []struct {
		Name              string
		Previous, Current raw.CPU
		Expected          float64
	}{
		{
			Name:     "No total usage changes",
			Previous: raw.CPU{TotalUsage: previousTotalUsage, Read: preReadTime, PreRead: preReadTime.Add(-time.Second), NumProcs: numProcs},
			Current:  raw.CPU{TotalUsage: previousTotalUsage, Read: readTime, PreRead: preReadTime, NumProcs: numProcs},
			Expected: 0,
		},
		{
			Name:     "using one core 100%",
			Previous: raw.CPU{TotalUsage: previousTotalUsage, Read: preReadTime, PreRead: preReadTime.Add(-time.Second), NumProcs: numProcs},
			Current:  raw.CPU{TotalUsage: currentTotalUsage, Read: readTime, PreRead: preReadTime, NumProcs: numProcs},
			Expected: 50,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			v := cpuPercent(c.Previous, c.Current)
			assert.Equal(t, c.Expected, v)
		})
	}
}

func TestGetNumOfLimitCores(t *testing.T) {
	tests := []struct {
		name          string
		containerJSON *container.InspectResponse
		numProcs      uint32
		want          float64
	}{
		{
			name: "Test with CPUCount set",
			containerJSON: &container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					HostConfig: &container.HostConfig{
						Resources: container.Resources{
							CPUCount: 4,
						},
					},
				},
			},
			numProcs: 8,
			want:     4,
		},
		{
			name: "Test with NanoCPUs set",
			containerJSON: &container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					HostConfig: &container.HostConfig{
						Resources: container.Resources{
							NanoCPUs: 2000000000,
						},
					},
				},
			},
			numProcs: 8,
			want:     2,
		},
		{
			name: "Test with no limits set",
			containerJSON: &container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					HostConfig: &container.HostConfig{},
				},
			},
			numProcs: 8,
			want:     8,
		},
		{
			name: "Test with nil HostConfig",
			containerJSON: &container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					HostConfig: nil,
				},
			},
			numProcs: 8,
			want:     8,
		},
		{
			name:          "Test with nil ContainerJSON",
			containerJSON: nil,
			numProcs:      8,
			want:          8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getNumOfLimitCores(tt.containerJSON, tt.numProcs)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetTotalMemory(t *testing.T) {
	tests := []struct {
		name          string
		containerJSON *container.InspectResponse
		want          uint64
	}{
		{
			name: "Test with Memory set",
			containerJSON: &container.InspectResponse{
				ContainerJSONBase: &container.ContainerJSONBase{
					HostConfig: &container.HostConfig{
						Resources: container.Resources{
							Memory: 2147483648, // 2 GiB
						},
					},
				},
			},
			want: 2147483648,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getTotalMemory(tt.containerJSON)
			assert.Equal(t, tt.want, got)
		})
	}
}
