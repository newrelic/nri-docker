// Copyright 2025 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/docker/docker/api/types/system"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	"github.com/newrelic/nri-docker/src/config"
	"github.com/newrelic/nri-docker/src/nri"
	"github.com/newrelic/nri-docker/src/raw/aws"
)

const (
	IntegrationName = "com.newrelic.docker"
)

func ExitOnErr(err error) {
	if err != nil {
		log.Error(err.Error())
		os.Exit(-1)
	}
}

func PrintVersion(integrationVersion, gitCommit, buildDate string) {
	fmt.Printf(
		"New Relic Docker integration Version: %s, Platform: %s, GoVersion: %s, GitCommit: %s, BuildDate: %s\n",
		integrationVersion,
		fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		runtime.Version(),
		gitCommit,
		buildDate)
}

func PopulateFromFargate(i *integration.Integration, args config.ArgumentList) {
	metadataBaseURL, err := aws.GetMetadataBaseURL()
	ExitOnErr(err)

	fargateFetcher, err := aws.NewFargateFetcher(metadataBaseURL)
	ExitOnErr(err)

	fargateDockerClient, err := aws.NewFargateInspector(metadataBaseURL)
	ExitOnErr(err)

	sampler, err := nri.NewSampler(fargateFetcher, fargateDockerClient, args)
	ExitOnErr(err)
	// Info is currently used to get the Storage Driver stats that is not present on Fargate.
	ExitOnErr(sampler.SampleAll(context.Background(), i, system.Info{}))
}
