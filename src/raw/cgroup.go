package raw

// NewCgroupsFetcher creates the proper metrics fetcher for the used cgroups version.
func NewCgroupsFetcher(hostRoot string, cgroupInfo *CgroupInfo, systemCPUReader SystemCPUReader) (Fetcher, error) {
	if cgroupInfo.Version == CgroupV2 {
		return NewCgroupsV2Fetcher(hostRoot, systemCPUReader, cgroupInfo.Driver)
	}

	return NewCgroupsV1Fetcher(hostRoot, systemCPUReader)
}
