// Copyright 2025 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	"github.com/newrelic/nri-docker/src/config"
	"github.com/newrelic/nri-docker/src/util"
)

var (
	integrationVersion = "0.0.0"
	gitCommit          = ""
	buildDate          = ""
)

func main() {
	args := config.ArgumentList{}
	// we always use the docker API for OSes other than Linux
	args.UseDockerAPI = util.UpdateDockerAPIArg(args.UseDockerAPI)
	i, err := integration.New(util.IntegrationName, integrationVersion, integration.Args(&args))
	util.ExitOnErr(err)

	if args.ShowVersion {
		util.PrintVersion(integrationVersion, gitCommit, buildDate)
		os.Exit(0)
	}

	log.SetupLogging(args.Verbose)

	if args.Fargate {
		util.PopulateFromFargate(i, args)
	} else {
		util.PopulateFromDocker(i, args)
	}

	util.ExitOnErr(i.Publish())
}
