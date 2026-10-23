// Copyright 2025 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package biz

import (
	"bytes"
	"os"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/v3/log"
	"github.com/stretchr/testify/assert"
)

// captureLogs returns everything logged until the test ends.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestCounterDelta(t *testing.T) {
	t.Run("counter that grew returns the difference", func(t *testing.T) {
		logs := captureLogs(t)
		assert.Equal(t, uint64(7), counterDelta("total CPU usage", uint64(10), uint64(3)))
		assert.Empty(t, logs.String())
	})

	t.Run("unchanged counter is a valid zero and is not logged", func(t *testing.T) {
		logs := captureLogs(t)
		assert.Equal(t, uint64(0), counterDelta("total CPU usage", uint64(10), uint64(10)))
		assert.Empty(t, logs.String())
	})

	t.Run("counter that went backwards resets to zero and warns", func(t *testing.T) {
		logs := captureLogs(t)
		assert.Equal(t, uint64(0), counterDelta("total CPU usage", uint64(5), uint64(1000)))
		assert.Contains(t, logs.String(), "total CPU usage went backwards (1000 -> 5)")
		assert.Contains(t, logs.String(), "resetting to 0")
		assert.Contains(t, logs.String(), "first sample")
	})

	t.Run("works for other unsigned types", func(t *testing.T) {
		captureLogs(t)
		assert.Equal(t, uint32(0), counterDelta("x", uint32(1), uint32(2)))
		assert.Equal(t, uint32(1), counterDelta("x", uint32(2), uint32(1)))
	})
}
