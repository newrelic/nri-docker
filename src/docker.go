package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"strings"

	"github.com/docker/docker/client"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"

	"github.com/newrelic/nri-docker/src/config"
	"github.com/newrelic/nri-docker/src/nri"
	"github.com/newrelic/nri-docker/src/raw"
	"github.com/newrelic/nri-docker/src/raw/aws"
)

const (
	integrationName = "com.newrelic.docker"
)

var (
	integrationVersion = "0.0.0"
	gitCommit          = ""
	buildDate          = ""
)

func main() {
	args := config.ArgumentList{}
	i, err := integration.New(integrationName, integrationVersion, integration.Args(&args))
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	if args.ShowVersion {
		fmt.Printf(
			"New Relic %s integration Version: %s, Platform: %s, GoVersion: %s, GitCommit: %s, BuildDate: %s\n",
			strings.Title(strings.Replace(integrationName, "com.newrelic.", "", 1)),
			integrationVersion,
			fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
			runtime.Version(),
			gitCommit,
			buildDate)
		os.Exit(0)
	}

	log.SetupLogging(args.Verbose)

	var fetcher raw.Fetcher
	var docker raw.DockerClient
	if args.Fargate {
		var metadataBaseURL *url.URL
		if metadataBaseURL, err = aws.MetadataV4BaseURL(); err != nil {
			log.Debug("The Metadata endpoint V4 is not available, falling back to V3: %s", err.Error())
			// If we do not find V4 we fall back to V3
			metadataBaseURL, err = aws.MetadataV3BaseURL()
		}
		exitOnErr(err)

		fetcher, err = aws.NewFargateFetcher(metadataBaseURL)
		exitOnErr(err)

		docker, err = aws.NewFargateInspector(metadataBaseURL)
		exitOnErr(err)
	} else {
		var tmpDocker *client.Client
		tmpDocker, err = client.NewClientWithOpts(client.FromEnv, client.WithVersion(args.DockerClientVersion))
		exitOnErr(err)
		defer tmpDocker.Close()
		docker = raw.NewCachedInfoDockerClient(tmpDocker)

		detectedHostRoot, hostRootErr := raw.DetectHostRoot(args.HostRoot, raw.CanAccessDir)
		exitOnErr(hostRootErr)

		cgroupInfo, cgroupInfoErr := raw.GetCgroupInfo(context.Background(), docker)
		exitOnErr(cgroupInfoErr)

		fetcher, err = raw.NewCgroupsFetcher(
			detectedHostRoot,
			cgroupInfo,
			raw.NewPosixSystemCPUReader(),
			raw.NewNetDevNetworkStatsGetter(),
		)
		exitOnErr(err)
	}
	sampler, err := nri.NewSampler(fetcher, docker, args)
	exitOnErr(err)
	exitOnErr(sampler.SampleAll(context.Background(), i))
	exitOnErr(i.Publish())
}

func exitOnErr(err error) {
	if err != nil {
		log.Error(err.Error())
		os.Exit(-1)
	}
}
