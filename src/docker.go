package main

import (
	"os"

	sdkArgs "github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-docker/src/docker"
	"github.com/newrelic/nri-docker/src/stats"
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

	log.SetupLogging(args.Verbose)
	provider, err := stats.NewCGroupsProvider()
	exitOnErr(err)
	defer provider.PersistStats()

	cs, err := docker.NewContainerSampler(provider)
	exitOnErr(err)

	exitOnErr(cs.SampleAll(i))

	exitOnErr(i.Publish())
}

func exitOnErr(err error) {
	if err != nil {
		log.Error(err.Error())
		os.Exit(-1)
	}
}
