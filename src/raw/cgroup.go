package raw

// NewCgroupsFetcher creates the proper metrics fetcher for the used cgroups version.
func NewCgroupsFetcher(
	hostRoot string,
	cgroupInfo *CgroupInfo,
	systemCPUReader SystemCPUReader,
	networkStatsGetter NetworkStatsGetter,
) (Fetcher, error) {
	if cgroupInfo.Version == CgroupV2 {
		return NewCgroupsV2Fetcher(hostRoot, cgroupInfo.Driver, systemCPUReader, networkStatsGetter)
	}

	return NewCgroupsV1Fetcher(hostRoot, systemCPUReader, networkStatsGetter)
}
