package workspacemanagerv2

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
)

// Step one design for cloud
// Assume k8s secret exists
// Copy current workspace style

type WorkspaceManager struct {
	ContainerManager ContainerManager
	Store            WorkspaceManagerStore
}

type Container struct {
	ID     string
	Status string // running stopped
}

// ContainerManager Interface for docker, podman etc.
type ContainerManager interface {
	GetContainer(ctx context.Context, containerID string) (*Container, error)
	StopContainer(ctx context.Context, containerID string) error
	DeleteContainer(ctx context.Context, containerID string) error
	StartContainer(ctx context.Context, containerID string) error
	DeleteVolume(ctx context.Context, volumeName string) error
}

type WorkspaceManagerStore interface {
	GetWorkspace(id string) (*entity.Workspace, error)
	GetWorkspaceSetupParams(id string) (*store.SetupParamsV0, error)
	GetWorkspaceSecretsConfig(id string) (interface{}, error)
}

func NewWorkspaceManager(cm ContainerManager, store WorkspaceManagerStore) *WorkspaceManager {
	return &WorkspaceManager{ContainerManager: cm, Store: store}
}

func (w WorkspaceManager) Start(ctx context.Context, workspaceID string) error {
	workspace, err := w.Store.GetWorkspace(workspaceID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	params, err := w.Store.GetWorkspaceSetupParams(workspaceID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	paramsData, err := json.MarshalIndent(params, "", " ")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// secretsConfig, err := w.Store.GetWorkspaceSecretsConfig(workspaceID)
	// if err != nil {
	// 	return breverrors.WrapAndTrace(err)
	// }

	setupParamsVolume := NewStaticVolume("setup_paramsV0", "/etc/meta", bytes.NewBuffer(paramsData))

	containerWorkspace := NewContainerWorkspace(w.ContainerManager, workspaceID, workspace.WorkspaceTemplate.Image,
		[]Volume{
			setupParamsVolume,
		})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = containerWorkspace.Start(ctx)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w WorkspaceManager) Reset(ctx context.Context, workspaceID string) error {
	return nil
}

func (w WorkspaceManager) Stop(ctx context.Context, workspaceID string) error {
	return nil
}

type ContainerWorkspace struct {
	ContainerManager ContainerManager
	ID               string
	Image            string
	Volumes          []Volume
}

func NewContainerWorkspace(cm ContainerManager, id string, image string, volumes []Volume) *ContainerWorkspace {
	return &ContainerWorkspace{ContainerManager: cm, ID: id, Image: image, Volumes: volumes}
}

func (c ContainerWorkspace) Start(ctx context.Context) error {
	container, err := c.ContainerManager.GetContainer(ctx, c.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if container == nil {
		err := c.CreateNew(ctx)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	if container.Status == "stopped" {
		err := c.StartFromStopped(ctx)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	if container.Status == "running" {
		_ = 0
		// do nothing
	}
	return nil
}

func (c ContainerWorkspace) CreateNew(ctx context.Context) error {
	err := c.CreateVolumes(ctx)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (c ContainerWorkspace) StartFromStopped(ctx context.Context) error {
	// update neccessary volumes // on second thoguht maybe not (not current behavior)
	// // params?
	// start
	err := c.ContainerManager.StartContainer(ctx, c.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (c ContainerWorkspace) Reset(ctx context.Context) error {
	err := c.Stop(ctx)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = c.Start(ctx)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (c ContainerWorkspace) Rebuild(ctx context.Context) error {
	err := c.Stop(ctx)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = c.ContainerManager.DeleteContainer(ctx, c.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = c.DeleteVolumes(ctx)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = c.Start(ctx)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (c ContainerWorkspace) CreateVolumes(ctx context.Context) error {
	// two kinds of updates depending on env
	// // start volume processes (to support dynamic volumes like k8s token, hcl config, etc)
	// // could be static init, sym link, callback, or poll base
	for _, v := range c.Volumes {
		err := v.Setup() // should this be a goroutine?
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

func (c ContainerWorkspace) DeleteVolumes(ctx context.Context) error {
	for _, v := range c.Volumes {
		err := v.Teardown() // should this be a goroutine?
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

func (c ContainerWorkspace) Stop(ctx context.Context) error {
	err := c.ContainerManager.StopContainer(ctx, c.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

type Volume interface {
	GetIdentifier() string // this may be a name or path to external mount
	GetMountToPath() string
	Setup() error
	Teardown() error // should also clear/delete
}

type StaticVolume struct {
	Name                string
	FromMountPathPrefix string
	ToMountPath         string
	ToWrite             io.Reader
}

func NewStaticVolume(name string, path string, toWrite io.Reader) StaticVolume {
	return StaticVolume{Name: name, ToMountPath: path, ToWrite: toWrite}
}

func (s StaticVolume) WithPathPrefix(prefix string) StaticVolume {
	s.FromMountPathPrefix = prefix
	return s
}

func (s StaticVolume) GetIdentifier() string {
	return s.GetMountFromPath()
}

func (s StaticVolume) GetMountToPath() string {
	return s.ToMountPath
}

func (s StaticVolume) GetMountFromPath() string {
	return s.FromMountPathPrefix
}

func (s StaticVolume) Setup() error {
	path := s.GetMountFromPath()
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	filePath := filepath.Join(s.FromMountPathPrefix, s.Name)
	f, err := os.Create(filePath) //nolint:gosec // executed in safe space
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	defer f.Close() //nolint:errcheck,gosec // defer

	_, err = io.Copy(f, s.ToWrite)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = f.Sync()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (s StaticVolume) Teardown() error {
	err := os.RemoveAll(s.FromMountPathPrefix)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

var _ Volume = StaticVolume{}
