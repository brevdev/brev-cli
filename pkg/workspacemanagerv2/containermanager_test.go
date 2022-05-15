package workspacemanagerv2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func GetAllContainerManagers() []ContainerManager {
	return []ContainerManager{DockerContainerManager{}}
}

func Test_DockerGetContainerDNE(t *testing.T) {
	dcms := GetAllContainerManagers()
	for _, cm := range dcms {
		res, err := cm.GetContainer(context.TODO(), "dne")
		assert.Error(t, err)
		assert.Empty(t, res)
	}
}

func Test_DockerCreateThenGetContainer(t *testing.T) {
	dcms := GetAllContainerManagers()
	for _, cm := range dcms {
		ctx := context.Background()
		containerID, err := cm.CreateContainer(ctx, CreateContainerOptions{}, "hello-world")
		if !assert.Nil(t, err) {
			return
		}
		res, err := cm.GetContainer(ctx, containerID)
		assert.Nil(t, err)
		assert.Equal(t, containerID, res.ID)
	}
}

func Test_DockerCreateThenStartThenStopContainer(t *testing.T) {
	dcms := GetAllContainerManagers()
	for _, cm := range dcms {
		ctx := context.Background()
		containerID, err := cm.CreateContainer(ctx, CreateContainerOptions{}, "hello-world")
		if !assert.Nil(t, err) {
			return
		}
		gr1, err := cm.GetContainer(ctx, containerID)
		assert.Nil(t, err)
		assert.Equal(t, ContainerStopped, gr1.Status)

		err = cm.StartContainer(ctx, containerID)
		assert.Nil(t, err)

		gr2, err := cm.GetContainer(ctx, containerID)
		assert.Nil(t, err)
		assert.Equal(t, ContainerRunning, gr2.Status)

		err = cm.StopContainer(ctx, containerID)
		assert.Nil(t, err)

		gr3, err := cm.GetContainer(ctx, containerID)
		assert.Nil(t, err)
		assert.Equal(t, ContainerStopped, gr3.Status)
	}
}
