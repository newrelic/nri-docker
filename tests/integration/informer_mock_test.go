package biz

import (
	"context"
	"github.com/docker/docker/api/types"
)

const (
	containerID = "my-container"
	pid         = 666
)

type InspectorMock struct{}

func (i InspectorMock) ContainerInspect(_ context.Context, _ string) (types.ContainerJSON, error) {
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID: containerID,
			State: &types.ContainerState{
				Pid: pid,
			},
			RestartCount: 2,
		},
	}, nil
}

func (i InspectorMock) Info(_ context.Context) (types.Info, error) {
	return types.Info{}, nil
}
