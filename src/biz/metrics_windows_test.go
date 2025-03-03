package biz

import (
	"testing"
	"time"

	"github.com/newrelic/nri-docker/src/raw"
	"github.com/stretchr/testify/assert"
)

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
