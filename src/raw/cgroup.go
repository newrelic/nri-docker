package raw

// NewCgroupsFetcher creates the proper metrics fetcher for the used cgroups version.
func NewCgroupsFetcher(hostRoot string, systemCPUReader SystemCPUReader) (Fetcher, error) {
	// TODO: check cgroups version and create the corresponding fetcher.
	return &CgroupsV1Fetcher{
		hostRoot:        hostRoot,
		systemCPUReader: systemCPUReader,
	}, nil
}
