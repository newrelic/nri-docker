package stats

import (
	"context"
	"encoding/json"
	"io/ioutil"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type Provider interface {
	Fetch(containerID string) (Cooked, error)
}

// TODO: this is really slow. Fetch the data directly from disk
type APIProvider struct {
	docker *client.Client
}

func NewAPIProvider(docker *client.Client) *APIProvider {
	return &APIProvider{docker: docker}
}

func (asp *APIProvider) Fetch(containerID string) (Cooked, error) {
	stats := types.Stats{}

	apiStats, err := asp.docker.ContainerStats(context.Background(), containerID, false)
	if err != nil {
		return Cooked(stats), err
	}

	jsonStats, err := ioutil.ReadAll(apiStats.Body)
	_ = apiStats.Body.Close()
	if err != nil {
		return Cooked(stats), err
	}

	err = json.Unmarshal(jsonStats, &stats)
	if err != nil {
		return Cooked(stats), err
	}

	return Cooked(stats), nil
}
