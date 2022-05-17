package workspacemanagerv2

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func GetAllContainerManagers() []ContainerManager {
	return []ContainerManager{DockerContainerManager{}}
}

// TIP: use docker inspect to get information about container like volume mounted, command, ports etc.

func Test_GetContainerDNE(t *testing.T) {
	dcms := GetAllContainerManagers()
	for _, cm := range dcms {
		res, err := cm.GetContainer(context.TODO(), "dne")
		assert.Error(t, err)
		assert.Empty(t, res)
	}
}

func Test_CreateThenGetContainer(t *testing.T) {
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

func Test_CreateThenStartThenStopContainer(t *testing.T) {
	dcms := GetAllContainerManagers()
	for _, cm := range dcms {
		ctx := context.Background()
		containerID, err := cm.CreateContainer(ctx, CreateContainerOptions{}, "nginx")
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

		err = cm.DeleteContainer(ctx, containerID)
		assert.Nil(t, err)

		gr4, err := cm.GetContainer(ctx, containerID)
		assert.Error(t, err)
		assert.Empty(t, gr4)
	}
}

func Test_PortMapping(t *testing.T) {
	dcms := GetAllContainerManagers()
	for _, cm := range dcms {
		ctx := context.Background()
		containerID, err := cm.CreateContainer(ctx, CreateContainerOptions{
			Ports: []string{"3456:80"},
		}, "nginx")
		if !assert.Nil(t, err) {
			return
		}

		err = cm.StartContainer(ctx, containerID)
		assert.Nil(t, err)

		time.Sleep(time.Second * 1)
		cmd := exec.CommandContext(ctx, "curl", "http://localhost:3456")
		out, err := cmd.CombinedOutput()
		fmt.Println(string(out))
		assert.Nil(t, err)

		err = cm.StopContainer(ctx, containerID)
		assert.Nil(t, err)
	}
}

func Test_Volumes(t *testing.T) {
	dcms := GetAllContainerManagers()
	for _, cm := range dcms {
		ctx := context.Background()
		localPath := fmt.Sprintf("/tmp/brevcli-test-volume/%s", uuid.New().String())
		fmt.Println(localPath)

		err := os.MkdirAll(localPath, os.ModePerm)
		assert.Nil(t, err)

		_, err = os.OpenFile(filepath.Join(localPath, "original"), os.O_CREATE, 0o600) //nolint:gosec // test
		assert.Nil(t, err)

		containerID, err := cm.CreateContainer(ctx, CreateContainerOptions{
			Volumes: []Volume{
				SimpleVolume{
					Identifier:  localPath,
					MountToPath: "/volume",
				},
			},
			Command:     "cp",
			CommandArgs: []string{"/volume/original", "/volume/new"},
		}, "nginx")
		if !assert.Nil(t, err) {
			return
		}

		err = cm.StartContainer(ctx, containerID)
		assert.Nil(t, err)

		info, err := ioutil.ReadDir(localPath)
		assert.Nil(t, err)
		assert.Len(t, info, 2)
	}
}
