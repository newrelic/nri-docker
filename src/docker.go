package main

import (
	"os"

	sdkArgs "github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-docker/src/docker"
)

type argumentList struct {
	sdkArgs.DefaultArgumentList
}

const (
	integrationName    = "com.newrelic.docker"
	integrationVersion = "0.1.0"
)

var (
	args argumentList
)

func main() {
	// Create Integration
	i, err := integration.New(integrationName, integrationVersion, integration.Args(&args))
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	sampler := docker.NewContainerSampler()

	entity := i.LocalEntity()
	if err := sampler.Populate(entity.NewMetricSet(docker.ContainerSampleName)); err != nil {
		log.Error("error populating %q: %s", docker.ContainerSampleName, err.Error())
		os.Exit(-1)
	}
	if err = i.Publish(); err != nil {
		log.Error(err.Error())
		os.Exit(-1)
	}
}
