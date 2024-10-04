package aws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	docker "github.com/docker/docker/api/types"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	"github.com/newrelic/infra-integrations-sdk/v3/persist"

	"github.com/newrelic/nri-docker/src/raw"
)

const fargateClientTimeout = 30 * time.Second
const fargateTaskStatsCacheKey = "fargate-task-stats"

var fargateHTTPClient = &http.Client{Timeout: fargateClientTimeout}

type timedDockerStats struct {
	//StatsJSON inherits all the fields from docker.Stats adding Network info
	docker.StatsJSON
	time time.Time
}

// FargateStats holds a map of Fargate container IDs as key and their Docker metrics
// as values.
type FargateStats map[string]*timedDockerStats

// FargateFetcher fetches metrics from Fargate endpoints in AWS ECS.
type FargateFetcher struct {
	baseURL        *url.URL
	http           *http.Client
	containerStore persist.Storer
	latestFetch    time.Time
}

// NewFargateFetcher creates a new FargateFetcher with the given HTTP client.
func NewFargateFetcher(baseURL *url.URL) (*FargateFetcher, error) {
	containerStore := persist.NewInMemoryStore()

	return &FargateFetcher{
		baseURL:        baseURL,
		http:           fargateHTTPClient,
		containerStore: containerStore,
	}, nil
}

// Fetch fetches raw metrics from a given Fargate container.
func (e *FargateFetcher) Fetch(container docker.ContainerJSON) (raw.Metrics, error) {
	stats, err := e.fargateStatsFromCacheOrNew()
	if err != nil {
		return raw.Metrics{}, err
	}
	rawMetrics := fargateRawMetrics(stats)
	if rawMetrics[container.ID] == nil {
		return raw.Metrics{}, fmt.Errorf("the raw metric map did nont contain the container with ID: %s, %d", container.ID, len(rawMetrics))
	}
	return *rawMetrics[container.ID], nil
}

// fargateStatsFromCacheOrNew wraps the access to Fargate task stats with a caching layer.
func (e *FargateFetcher) fargateStatsFromCacheOrNew() (FargateStats, error) {
	defer func() {
		if err := e.containerStore.Save(); err != nil {
			log.Warn("error persisting Fargate task metadata: %s", err)
		}
	}()

	var response FargateStats
	_, err := e.containerStore.Get(fargateTaskStatsCacheKey, &response)
	if err == persist.ErrNotFound {
		response, err = e.getFargateContainerMetrics()
	}
	if err != nil {
		return nil, fmt.Errorf("cannot fetch task stats response: %s", err)
	}
	e.containerStore.Set(fargateTaskMetadataCacheKey, response)
	return response, nil
}

// getFargateContainerMetrics returns Docker metrics from inside a Fargate container.
// It captures the ECS container metadata endpoint from the environment variable defined by
// `containerMetadataEnvVar`.
// Note that the endpoint doesn't follow strictly the same schema as Docker's: it returns a list of containers,
// instead of only one. They are not compatible in terms of the requests that they accept, but they share
// part of the response's schema.
func (e *FargateFetcher) getFargateContainerMetrics() (FargateStats, error) {
	endpoint := TaskStatsEndpoint(e.baseURL.String())

	response, err := metadataResponse(e.http, endpoint)
	if err != nil {
		return nil, fmt.Errorf(
			"error when sending request to ECS container metadata endpoint (%s): %v",
			endpoint,
			err,
		)
	}
	log.Debug("fargate task stats response from endpoint %s: %s", endpoint, string(response))
	e.latestFetch = time.Now()

	var stats FargateStats
	err = json.Unmarshal(response, &stats)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling ECS container: %v", err)
	}

	now := time.Now()
	for i, k := range stats {
		if k == nil {
			log.Warn("getting stats from fargate returned a nil at index %d.", i)
			log.Warn("raise the log level to debug to have more information")
			continue
		}
		k.time = now
	}

	return stats, nil
}
