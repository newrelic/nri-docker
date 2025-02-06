// Copyright 2025 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"context"

	"github.com/docker/docker/client"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	"github.com/newrelic/nri-docker/src/config"
	"github.com/newrelic/nri-docker/src/nri"
	"github.com/newrelic/nri-docker/src/raw"
	"github.com/newrelic/nri-docker/src/raw/dockerapi"
)

// ForceTrueForOSOtherThanLinux returns the value of the dockerAPIArg without any modification
func ForceTrueForOSOtherThanLinux(dockerAPIArg bool) bool {
	return dockerAPIArg
}

func UseDockerAPI(dockerAPIRequested bool, version string) bool {
	if dockerAPIRequested {
		if version == raw.CgroupV2 {
			return true
		}
		log.Debug("UseDockerAPI config is not available on CgroupV1")
		return false
	}
	return false
}

func PopulateFromDocker(i *integration.Integration, args config.ArgumentList) {
	withVersionOpt := client.WithVersion(args.DockerClientVersion)
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, withVersionOpt)
	ExitOnErr(err)
	defer dockerClient.Close()

	cgroupInfo, err := dockerClient.Info(context.Background())
	ExitOnErr(err)

	var fetcher raw.Fetcher
	if UseDockerAPI(args.UseDockerAPI, cgroupInfo.CgroupVersion) {
		fetcher = dockerapi.NewFetcher(dockerClient)
	} else { // use cgroups as source of data
		fetcher, err = raw.NewCgroupFetcher(args.HostRoot, cgroupInfo)
		ExitOnErr(err)
	}

	sampler, err := nri.NewSampler(fetcher, dockerClient, args)
	ExitOnErr(err)
	ExitOnErr(sampler.SampleAll(context.Background(), i, cgroupInfo))
}
