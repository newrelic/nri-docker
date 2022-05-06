package raw

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCountCpusetCPUs(t *testing.T) {
	cases := []struct {
		input         string
		expectedCount uint
	}{
		{
			input:         "0",
			expectedCount: 1,
		},
		{
			input:         "0-4",
			expectedCount: 4,
		},
		{
			input:         "1,4",
			expectedCount: 2,
		},
		{
			input:         "0-4,8,10",
			expectedCount: 6,
		},
		{
			input:         "0-4,8,10,12-16",
			expectedCount: 10,
		},
	}

	for _, c := range cases {
		t.Run("Input "+c.input, func(t *testing.T) {
			count, err := countCpusetCPUs(c.input)
			require.NoError(t, err)
			assert.Equal(t, c.expectedCount, count)
		})
	}
}

func TestCountCpusetCPUsErrors(t *testing.T) {
	invalidInputs := []string{
		"",
		"-3",
		"7-3",
		"0-4-3",
		"1,",
		"a-1",
		"a",
		"1-a",
	}
	for _, input := range invalidInputs {
		t.Run("Input "+input, func(t *testing.T) {
			_, err := countCpusetCPUs(input)
			assert.Error(t, err)
		})
	}
}
