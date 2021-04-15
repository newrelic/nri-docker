package biz

import (
	"fmt"

	"github.com/docker/docker/api/types"
	humanize "github.com/dustin/go-humanize"
)

// DeviceMapperStats contains the stats of devicemapper storages driver type
type DeviceMapperStats struct {
	DataAvailable     uint64
	DataUsed          uint64
	DataTotal         uint64
	MetadataAvailable uint64
	MetadataUsed      uint64
	MetadataTotal     uint64
}

// ParseDeviceMapperStats parses the output from the info DriverStatus info Array. This information is not consider stable by
// Docker and could change anytime according to API the docs.
func ParseDeviceMapperStats(info types.Info) (*DeviceMapperStats, error) {

	if info.Driver != "devicemapper" {
		return nil, fmt.Errorf("only devicemapper is supported")
	}

	if len(info.DriverStatus) == 0 {
		return nil, fmt.Errorf("DriverStatus not found")
	}

	stats := &DeviceMapperStats{}
	for _, status := range info.DriverStatus {
		value, err := humanize.ParseBytes(status[1])
		if err != nil {
			continue
		}
		switch status[0] {
		case "Data Space Used":
			stats.DataUsed = value
		case "Data Space Available":
			stats.DataAvailable = value
		case "Data Space Total":
			stats.DataTotal = value
		case "Metadata Space Used":
			stats.MetadataUsed = value
		case "Metadata Space Available":
			stats.MetadataAvailable = value
		case "Metadata Space Total":
			stats.MetadataTotal = value
		}
	}
	return stats, nil
}
