package raw

// NewCgroupsFetcher creates the proper metrics fetcher for the used cgroups version.
func NewCgroupsFetcher(hostRoot string, cgroupInfo *CgroupInfo, systemCPUReader SystemCPUReader) (Fetcher, error) {
	// TODO: use cgroup version from cgroupInfo and create the corresponding fetcher.
	return NewCgroupsV1Fetcher(hostRoot, systemCPUReader)
}
