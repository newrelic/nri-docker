package raw

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseUint(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		base     int
		bitSize  int
		expected uint64
		err      error
	}{
		{
			name:     "valid positive number",
			input:    "123",
			base:     10,
			bitSize:  64,
			expected: 123,
			err:      nil,
		},
		{
			name:     "valid negative number",
			input:    "-123",
			base:     10,
			bitSize:  64,
			expected: 0,
			err:      nil,
		},
		{
			name:     "invalid number",
			input:    "abc",
			base:     10,
			bitSize:  64,
			expected: 0,
			err:      &strconv.NumError{Func: "ParseUint", Num: "abc", Err: strconv.ErrSyntax},
		},
		{
			name:     "valid hex number",
			input:    "1a",
			base:     16,
			bitSize:  64,
			expected: 26,
			err:      nil,
		},
		{
			name:     "valid large number",
			input:    "18446744073709551615",
			base:     10,
			bitSize:  64,
			expected: 18446744073709551615,
			err:      nil,
		},
		{
			name:     "number out of range",
			input:    "18446744073709551616",
			base:     10,
			bitSize:  64,
			expected: 0,
			err:      &strconv.NumError{Func: "ParseUint", Num: "18446744073709551616", Err: strconv.ErrRange},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseUint(tt.input, tt.base, tt.bitSize)
			assert.Equal(t, tt.expected, result)
			if tt.err != nil {
				assert.EqualError(t, err, tt.err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
