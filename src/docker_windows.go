//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
	"github.com/newrelic/infra-integrations-sdk/v3/integration"
	"github.com/newrelic/infra-integrations-sdk/v3/log"
	"github.com/newrelic/nri-docker/src/config"
	"github.com/newrelic/nri-docker/src/nri"
	"github.com/newrelic/nri-docker/src/raw"
	"github.com/newrelic/nri-docker/src/raw/aws"
	"github.com/newrelic/nri-docker/src/raw/dockerapi"
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
	exitOnErr(err)
	if args.ShowVersion {
		printVersion()
		os.Exit(0)
	}
	log.SetupLogging(args.Verbose)
	if args.Fargate {
		populateFromFargate(i, args)
	} else {
		populateFromDocker(i, args)
	}
	exitOnErr(i.Publish())
}

func populateFromFargate(i *integration.Integration, args config.ArgumentList) {
	metadataBaseURL, err := aws.GetMetadataBaseURL()
	exitOnErr(err)
	fargateFetcher, err := aws.NewFargateFetcher(metadataBaseURL)
	exitOnErr(err)
	fargateDockerClient, err := aws.NewFargateInspector(metadataBaseURL)
	exitOnErr(err)
	sampler, err := nri.NewSampler(fargateFetcher, fargateDockerClient, args)
	exitOnErr(err)
	// Info is currently used to get the Storage Driver stats that is not present on Fargate.
	exitOnErr(sampler.SampleAll(context.Background(), i, system.Info{}))
}

func populateFromDocker(i *integration.Integration, args config.ArgumentList) {
	withVersionOpt := client.WithVersion(args.DockerClientVersion)
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, withVersionOpt)
	exitOnErr(err)
	defer dockerClient.Close()
	cgroupInfo, err := dockerClient.Info(context.Background())
	exitOnErr(err)
	var fetcher raw.Fetcher
	fetcher = dockerapi.NewFetcher(dockerClient)
	sampler, err := nri.NewSampler(fetcher, dockerClient, args)
	exitOnErr(err)
	exitOnErr(sampler.SampleAll(context.Background(), i, cgroupInfo))
}

func useDockerAPI(_ bool, _ string) bool {
	return true
}

func exitOnErr(err error) {
	if err != nil {
		log.Error(err.Error())
		os.Exit(-1)
	}
}

func printVersion() {
	fmt.Printf(
		"New Relic Docker integration Version: %s, Platform: %s, GoVersion: %s, GitCommit: %s, BuildDate: %s\n",
		integrationVersion,
		fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		runtime.Version(),
		gitCommit,
		buildDate)
}
