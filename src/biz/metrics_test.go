// Package biz provides business-value metrics from system raw metrics
package biz

import (
	"reflect"
	"testing"
	"time"

	"github.com/newrelic/infra-integrations-sdk/persist"
	"github.com/newrelic/nri-docker/src/raw"
)

func TestMetricsFetcher_memory(t *testing.T) {
	type fields struct {
		store              persist.Storer
		fetcher            raw.Fetcher
		inspector          Inspector
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
			//docker run --memory-swap=400m --memory=300m --memory-reservation=250m stress stress-ng --vm 1 --vm-bytes 350m
			name: "swap metrics",
			args: args{
				raw.Memory{

					UsageLimit:        314572800,
					Cache:             339968,
					RSS:               312569856,
					SwapUsage:         371634176,
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
				SwapUsageBytes:        369754112,
				SwapOnlyUsageBytes:    57184256,
				SwapLimitBytes:        419430400,
				SwapLimitUsagePercent: 88.15625,
				SoftLimitBytes:        262144000,
			},
		},
		{
			//docker run stress stress-ng --vm 1 --vm-bytes 100m
			name: "no swap",
			args: args{
				raw.Memory{
					UsageLimit:        9223372036854771712,
					Cache:             7839744,
					RSS:               104759296,
					SwapUsage:         115326976,
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
				SwapUsageBytes:        104759296,
				SwapOnlyUsageBytes:    0,
				SwapLimitBytes:        0,
				SwapLimitUsagePercent: 0.0,
				SoftLimitBytes:        0,
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
			if got := mc.memory(tt.args.mem); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MetricsFetcher.memory() = %v, want %v", got, tt.want)
			}
		})
	}
}
