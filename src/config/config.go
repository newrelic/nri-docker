package config

import "github.com/newrelic/infra-integrations-sdk/args"

type ArgumentList struct {
	args.DefaultArgumentList
	HostRoot              string `default:"" help:"If the integration is running from a container, the mounted folder pointing to the host root folder"`
	Fargate               bool   `default:"false" help:"Enables Fargate container metrics fetching. If enabled no metrics are collected from cgroups or Docker. Defaults to false"`
	ExitedContainersTTL   string `default:"24h" help:"Enables to integration to stop reporting Exited containers that are older than the set TTL. Possible values are time-strings: 1s, 1m, 1h"`
	DockerClientVersion   string `default:"1.24" help:"Optional. Specify the version of the docker client. Used for compatibility."`
	DisableStorageMetrics bool   `default:"false" help:"Disables storage driver metrics collection."`
	ShowVersion           bool   `default:"false" help:"Print build information and exit"`
	// CgroupPath and CgroupDriver arguments are not used but are kept here for backwards compatibility reasons.
	CgroupPath   string `default:"" help:"Deprecated. cgroup_path argument is not used anymore."`
	CgroupDriver string `default:"" help:"Deprecated. cgroup_driver argument is not used anymore."`

	CacheTTL string `default:"1m" help:"Set the maximum cache that the integration is going to use to calculate rates and deltas. Possible values are time-strings: 1s, 1m, 1h"`
}
