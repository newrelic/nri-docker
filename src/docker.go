package main

import (
	"context"
	"os"
	"time"

	"github.com/docker/docker/client"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"

	"github.com/newrelic/nri-docker/src/nri"
	"github.com/newrelic/nri-docker/src/raw"
	"github.com/newrelic/nri-docker/src/raw/aws"
)

type argumentList struct {
	Verbose             bool   `default:"false" help:"Print more information to logs."`
	Pretty              bool   `default:"false" help:"Print pretty formatted JSON."`
	NriCluster          string `default:"" help:"Optional. Cluster name"`
	HostRoot            string `default:"" help:"If the integration is running from a container, the mounted folder pointing to the host root folder"`
	CgroupPath          string `default:"" help:"Optional. The path where cgroup is mounted."`
	Fargate             bool   `default:"false" help:"Enables Fargate container metrics fetching. If enabled no metrics are collected from cgroups or Docker. Defaults to false"`
	ExitedContainersTTL string `default:"24h" help:"Enables to integration to stop reporting Exited containers that are older than the set TTL. Possible values are time-strings: 1s, 1m, 1h"`
	CgroupDriver        string `default:"" help:"Optional. Specify the cgroup driver."`
	DockerClientVersion string `default:"1.24" help:"Optional. Specify the version of the docker client. Used for compatibility."`
}

const (
	integrationName    = "com.newrelic.docker"
	integrationVersion = "1.4.0"
)

var (
	args argumentList
)

func main() {
	i, err := integration.New(integrationName, integrationVersion, integration.Args(&args))
	if err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}

	log.SetupLogging(args.Verbose)

	exitedContainerTTL, err := time.ParseDuration(args.ExitedContainersTTL)
	exitOnErr(err)

	var fetcher raw.Fetcher
	var docker nri.DockerClient
	if args.Fargate {
		metadataV3BaseURL, err := aws.MetadataV3BaseURL()
		exitOnErr(err)

		fetcher, err = aws.NewFargateFetcher(metadataV3BaseURL)
		exitOnErr(err)

		docker, err = aws.NewFargateInspector(metadataV3BaseURL)
		exitOnErr(err)
	} else {
		detectedHostRoot, err := raw.DetectHostRoot(args.HostRoot, raw.CanAccessDir)
		exitOnErr(err)
		fetcher, err = raw.NewCgroupsFetcher(
			detectedHostRoot,
			args.CgroupDriver,
			args.CgroupPath,
		)
		exitOnErr(err)
		var tmpDocker *client.Client
		tmpDocker, err = client.NewEnvClient()
		exitOnErr(err)
		defer tmpDocker.Close()
		tmpDocker.UpdateClientVersion(args.DockerClientVersion)
		docker = tmpDocker
	}
	sampler, err := nri.NewSampler(fetcher, docker, exitedContainerTTL)
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
