package user

import (
	"time"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/infra-integrations-sdk/persist"
	"github.com/newrelic/nri-docker/src/system"
)

type Network system.Network

type BlockingIO struct {
	TotalReadCount  float64
	TotalWriteCount float64
	TotalReadBytes  float64
	TotalWriteBytes float64
}

type MetricsCollector struct {
	store persist.Storer
}

func NewCollector() (*MetricsCollector, error) {
	store, err := persist.NewFileStore( // TODO: make the following options configurable
		persist.DefaultPath("container_cpus"),
		log.NewStdErr(true),
		60*time.Second)

	if err != nil {
		return nil, err
	}
	return &MetricsCollector{store: store}, nil
}
