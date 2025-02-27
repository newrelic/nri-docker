// Package biz provides business-value metrics from system raw metrics
package biz

import (
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/newrelic/infra-integrations-sdk/v3/persist"
	"github.com/newrelic/nri-docker/src/raw"
	"github.com/stretchr/testify/assert"
)

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
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			v := cpuPercent(c.Previous, c.Current)
			assert.Equal(t, c.Expected, v)
		})
	}

}

func TestMetricsFetcher_CPU_LimitCores(t *testing.T) {
	type args struct {
		cpu  raw.Metrics
		json *types.ContainerJSON
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
				json: &types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						HostConfig: &container.HostConfig{
							Resources: container.Resources{
								NanoCPUs: 500000000,
							},
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
				json: &types.ContainerJSON{
					ContainerJSONBase: &types.ContainerJSONBase{
						HostConfig: &container.HostConfig{},
					},
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

func uint64ToPointer(u uint64) *uint64 {
	return &u
}

func float64ToPointer(f float64) *float64 {
	return &f
}
