package raw

import (
	"context"
)

const (
	CgroupV1 = "1"
	CgroupV2 = "2"
)

const (
	CgroupSystemd = "systemd"
	CgroupGroupfs = "cgroupfs"
)

type CgroupInfo struct {
	Version string
	Driver  string
}

func GetCgroupInfo(ctx context.Context, informer DockerInformer) (*CgroupInfo, error) {
	info, err := informer.Info(ctx)
	if err != nil {
		return nil, err
	}
	return &CgroupInfo{Version: info.CgroupVersion, Driver: info.CgroupDriver}, nil
}
