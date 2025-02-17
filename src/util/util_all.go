// Copyright 2025 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package util

import (
	"context"
	"runtime"

	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/nri-docker/src/config"
	"github.com/newrelic/nri-docker/src/nri"
	"github.com/newrelic/nri-docker/src/raw/dockerapi"
)

// ForceTrueForOSOtherThanLinux returns true always for OSes other than Linux
func ForceTrueForOSOtherThanLinux(dockerAPIArg bool) bool {
	return true
}

func PopulateFromDocker(i *integration.Integration, args config.ArgumentList) {
	withVersionOpt := client.WithVersion(args.DockerClientVersion)
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, withVersionOpt)
	ExitOnErr(err)
	defer dockerClient.Close()

	fetcher := dockerapi.NewFetcher(dockerClient, runtime.GOOS)

	sampler, err := nri.NewSampler(fetcher, dockerClient, args)
	ExitOnErr(err)
	// always use dockerAPI if not on Linux
	ExitOnErr(sampler.SampleAll(context.Background(), i, system.Info{}))
}
