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

type ContainerStatus string

const (
	ContainerRunning ContainerStatus = "running"
	ContainerStopped ContainerStatus = "stopped"
)

type Container struct {
	ID     string
	Status ContainerStatus
}

type CreateContainerOptions struct {
	Name    string
	Volumes []Volume
	Ports   []string

	Command     string
	CommandArgs []string
}

// ContainerManager Interface for docker, podman etc.
type ContainerManager interface {
	GetContainer(ctx context.Context, containerID string) (*Container, error)
	StopContainer(ctx context.Context, containerID string) error
	DeleteContainer(ctx context.Context, containerID string) error
	CreateContainer(ctx context.Context, options CreateContainerOptions, image string) (string, error)
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

func (w WorkspaceManager) MakeContainerWorkspace(workspaceID string) (*ContainerWorkspace, error) {
	workspace, err := w.Store.GetWorkspace(workspaceID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	params, err := w.Store.GetWorkspaceSetupParams(workspaceID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	paramsData, err := json.MarshalIndent(params, "", " ")
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
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

	return containerWorkspace, nil
}

func (w WorkspaceManager) Start(ctx context.Context, workspaceID string) error {
	containerWorkspace, err := w.MakeContainerWorkspace(workspaceID)
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
	containerWorkspace, err := w.MakeContainerWorkspace(workspaceID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = containerWorkspace.Reset(ctx)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w WorkspaceManager) Stop(ctx context.Context, workspaceID string) error {
	containerWorkspace, err := w.MakeContainerWorkspace(workspaceID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = containerWorkspace.Stop(ctx)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

type ContainerWorkspace struct {
	ContainerManager ContainerManager
	Identifier       string
	Image            string
	Volumes          []Volume
}

func NewContainerWorkspace(cm ContainerManager, identifier string, image string, volumes []Volume) *ContainerWorkspace {
	return &ContainerWorkspace{ContainerManager: cm, Identifier: identifier, Image: image, Volumes: volumes}
}

func (c ContainerWorkspace) Start(ctx context.Context) error {
	container, err := c.ContainerManager.GetContainer(ctx, c.Identifier)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if container == nil { //nolint:gocritic // I like the else statement here
		err := c.CreateNew(ctx)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else if container.Status == ContainerStopped {
		err := c.StartFromStopped(ctx)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else if container.Status == ContainerRunning {
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
	containerID, err := c.ContainerManager.CreateContainer(ctx, CreateContainerOptions{
		Name:    c.Identifier,
		Volumes: c.Volumes,
	}, c.Image)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = c.ContainerManager.StartContainer(ctx, containerID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (c ContainerWorkspace) StartFromStopped(ctx context.Context) error {
	// update necessary volumes // on second thoguht maybe not (not current behavior)
	// // params?
	// start
	err := c.ContainerManager.StartContainer(ctx, c.Identifier)
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
	err = c.ContainerManager.DeleteContainer(ctx, c.Identifier)
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
		err := v.Setup(ctx) // should this be a goroutine?
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

func (c ContainerWorkspace) DeleteVolumes(ctx context.Context) error {
	for _, v := range c.Volumes {
		err := v.Teardown(ctx) // should this be a goroutine?
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

func (c ContainerWorkspace) Stop(ctx context.Context) error {
	err := c.ContainerManager.StopContainer(ctx, c.Identifier)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

type Volume interface {
	GetIdentifier() string // this may be a name or path to external mount
	GetMountToPath() string
	Setup(ctx context.Context) error
	Teardown(ctx context.Context) error // should also clear/delete
}

type StaticVolume struct {
	Name                string
	FromMountPathPrefix string
	ToMountPath         string
	ToWrite             io.Reader
}

var _ Volume = StaticVolume{}

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

func (s StaticVolume) Setup(_ context.Context) error {
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

func (s StaticVolume) Teardown(_ context.Context) error {
	err := os.RemoveAll(s.FromMountPathPrefix)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

type SimpleVolume struct {
	Identifier  string
	MountToPath string
}

var _ Volume = SimpleVolume{}

func (s SimpleVolume) GetIdentifier() string {
	return s.Identifier
}

func (s SimpleVolume) GetMountToPath() string {
	return s.MountToPath
}

func (s SimpleVolume) Setup(_ context.Context) error {
	return nil
}

func (s SimpleVolume) Teardown(_ context.Context) error {
	return nil
}
