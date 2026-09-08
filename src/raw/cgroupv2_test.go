//go:build linux

package raw

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCgroupV2WeightToShares(t *testing.T) {
	// These expected values are derived from the formula: (weight-1)*262142/9999 + 2
	cases := []struct {
		name           string
		weight         uint64
		expectedShares uint64
	}{
		{
			name:           "zero weight returns zero (unset)",
			weight:         0,
			expectedShares: 0,
		},
		{
			name:           "minimum weight (1) maps to minimum v1 shares (2)",
			weight:         1,
			expectedShares: 2, // (1-1)*262142/9999 + 2 = 0 + 2
		},
		{
			name: "weight 40 is Docker/Moby encoding of 1024 shares (1 vCPU default)",
			// Docker formula: weight = 1 + ((1024-2)*9999)/262142 = 1+39 = 40
			weight:         40,
			expectedShares: 1024, // 39*262142/9999 + 2 = 1022 + 2
		},
		{
			name: "weight 79 is Docker/Moby encoding of ~2048 shares (2.0 vCPU)",
			// Docker formula: weight = 1 + ((2048-2)*9999)/262142 = 1+78 = 79
			weight:         79,
			expectedShares: 2046, // 78*262142/9999 + 2 = 2044 + 2
		},
		{
			name: "weight 10 is Docker/Moby encoding of ~256 shares (0.25 vCPU)",
			// Docker formula: weight = 1 + ((256-2)*9999)/262142 = 1+9 = 10
			weight:         10,
			expectedShares: 237, // 9*262142/9999 + 2 = 2359278/9999 + 2 = 235 + 2
		},
		{
			name: "default cgroup v2 weight (100) converts correctly",
			// The cgroup v2 default weight of 100 is NOT the same as the cgroup v1
			// default of 1024 shares. Docker maps cpu.shares=1024 -> cpu.weight=40.
			weight:         100,
			expectedShares: 2597, // 99*262142/9999 + 2 = 2595 + 2
		},
		{
			name:           "maximum weight (10000) maps to maximum v1 shares",
			weight:         10000,
			expectedShares: 262144, // 9999*262142/9999 + 2 = 262142 + 2
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cgroupV2WeightToShares(tc.weight)
			assert.Equal(t, tc.expectedShares, got)
		})
	}
}

// TestCgroupV2WeightToSharesRoundTrip verifies that converting Docker's cpu.shares → cpu.weight
// and back with cgroupV2WeightToShares produces a value within ±2 of the original.
// Integer division makes the round-trip slightly lossy; this bounds the drift.
func TestCgroupV2WeightToSharesRoundTrip(t *testing.T) {
	// Docker/Moby's forward conversion: shares -> weight
	dockerSharesToWeight := func(shares uint64) uint64 {
		if shares <= 2 {
			return 1
		}
		return 1 + (shares-2)*9999/262142
	}

	cases := []struct {
		name   string
		shares uint64
	}{
		{"0.25 vCPU (256 shares)", 256},
		{"0.5 vCPU (512 shares)", 512},
		{"1.0 vCPU (1024 shares)", 1024},
		{"2.0 vCPU (2048 shares)", 2048},
		{"4.0 vCPU (4096 shares)", 4096},
		{"8.0 vCPU (8192 shares)", 8192},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			weight := dockerSharesToWeight(tc.shares)
			got := cgroupV2WeightToShares(weight)
			diff := int64(got) - int64(tc.shares)
			if diff < 0 {
				diff = -diff
			}
			assert.LessOrEqualf(t, diff, int64(2),
				"round-trip drift for shares=%d (via weight=%d) got=%d, drift=%d should be ≤2",
				tc.shares, weight, got, diff)
		})
	}
}
