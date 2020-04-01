package biz

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/persist"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/nri-docker/src/raw/aws"
)

const fargateContainerID = "2bd0cbf916a2745ef9377277c33fd7e6582455d73feb0d22a15f421496d5f916"
const fargateTaskID = "f05a5672397746638bc201a252c5bb75"

func TestFargateMetrics(t *testing.T) {
	ts, cleanup := startMetadataEndpointStub(t, fargateTaskID)
	defer cleanup()
	baseURL, err := url.Parse(fmt.Sprintf("%s/v3/%s", ts.URL, fargateTaskID))
	require.NoError(t, err)

	fetcher, err := aws.NewFargateFetcher(baseURL)
	require.NoError(t, err)

	inspector, err := aws.NewFargateInspector(baseURL)
	require.NoError(t, err)

	metrics := NewProcessor(
		persist.NewInMemoryStore(),
		fetcher,
		inspector,
	)
	samples, err := metrics.Process(fargateContainerID)
	require.NoError(t, err)

	assert.Equal(t, float64(2), samples.CPU.LimitCores)
	assert.Equal(t, uint64(200704), samples.Memory.CacheUsageBytes)
	assert.Equal(t, uint64(268435456), samples.Memory.MemLimitBytes)
	assert.Equal(t, uint64(11292672), samples.Memory.RSSUsageBytes)
	assert.Equal(t, uint64(11292672), samples.Memory.UsageBytes)
	assert.Equal(t, 4.20684814453125, samples.Memory.UsagePercent)

	assert.Equal(t, uint64(11), samples.Pids.Current)
	assert.Equal(t, uint64(0), samples.Pids.Limit)

	assert.Equal(t, float64(0), samples.BlkIO.TotalReadBytes)
	assert.Equal(t, float64(0), samples.BlkIO.TotalReadCount)
	assert.Equal(t, float64(7839744), samples.BlkIO.TotalWriteBytes)
	assert.Equal(t, float64(957), samples.BlkIO.TotalWriteCount)

	assert.Empty(t, samples.Network)
}

func startMetadataEndpointStub(t *testing.T, taskID string) (server *httptest.Server, cleanup func()) {
	t.Helper()

	ts := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			var response []byte
			var err error

			switch r.RequestURI {
			case fmt.Sprintf("/v3/%s/task", taskID):
				response, err = ioutil.ReadFile("testdata/task_metadata_response.json")
			case fmt.Sprintf("/v3/%s/task/stats", taskID):
				response, err = ioutil.ReadFile("testdata/task_container_stats_response.json")
			}
			require.NoError(t, err)

			_, err = w.Write(response)
			require.NoError(t, err)
		},
	))
	return ts, ts.Close
}
