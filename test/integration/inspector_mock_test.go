package biz

import (
	"context"
	"github.com/docker/docker/api/types"
)

type InspectorMock struct {
	containerID  string
	pid          int
	restartCount int
}

func NewInspectorMock(containerID string, pid, restartCount int) InspectorMock {
	return InspectorMock{
		containerID:  containerID,
		pid:          pid,
		restartCount: restartCount,
	}
}

func (i InspectorMock) ContainerInspect(_ context.Context, _ string) (types.ContainerJSON, error) {
	return types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID: i.containerID,
			State: &types.ContainerState{
				Pid: i.pid,
			},
			RestartCount: i.restartCount,
		},
	}, nil
}

func (i InspectorMock) Info(_ context.Context) (types.Info, error) {
	return types.Info{}, nil
}
