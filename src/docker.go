package main

import (
	"os"

	sdkArgs "github.com/newrelic/infra-integrations-sdk/args"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-docker/src/nri"
)

type argumentList struct {
	sdkArgs.DefaultArgumentList
	HostRoot string `default:"/host" help:"If the integration is running from a container, the mounted folder pointing to the host root folder"`
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

	cs, err := nri.NewContainerSampler(args.HostRoot)
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
