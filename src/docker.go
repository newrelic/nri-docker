// Copyright 2025 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	"github.com/newrelic/nri-docker/src/config"
	"github.com/newrelic/nri-docker/src/driver"
)

var (
	integrationVersion = "0.0.0"
	gitCommit          = ""
	buildDate          = ""
)

func main() {
	args := config.ArgumentList{}
	// we always use the docker API for OSes other than Linux
	args.UseDockerAPI = driver.ForceTrueForOSOtherThanLinux(args.UseDockerAPI)
	i, err := integration.New(driver.IntegrationName, integrationVersion, integration.Args(&args))
	driver.ExitOnErr(err)

	if args.ShowVersion {
		driver.PrintVersion(integrationVersion, gitCommit, buildDate)
		os.Exit(0)
	}

	log.SetupLogging(args.Verbose)

	if args.Fargate {
		driver.PopulateFromFargate(i, args)
	} else {
		driver.PopulateFromDocker(i, args)
	}

	driver.ExitOnErr(i.Publish())
}
