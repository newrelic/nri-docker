// Copyright 2025 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package driver

import (
	"context"
	"runtime"

	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/client"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/nri-docker/src/config"
	"github.com/newrelic/nri-docker/src/nri"
	"github.com/newrelic/nri-docker/src/raw"
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

	docker := raw.NewDockerClientWrapper(dockerClient)
	defer docker.Close()

	fetcher := dockerapi.NewFetcher(docker, runtime.GOOS)

	sampler, err := nri.NewSampler(fetcher, docker, args)
	ExitOnErr(err)
	// always use dockerAPI if not on Linux
	ExitOnErr(sampler.SampleAll(context.Background(), i, system.Info{}))
}
