package workspacemanagerv2

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"

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
	GetWorkspaceMeta(id string) (*store.WorkspaceMeta, error)
	GetWorkspaceSetupParams(id string) (*store.SetupParamsV0, error)
	GetWorkspaceSecretsConfig(id string) (string, error)
}

func NewWorkspaceManager(cm ContainerManager, store WorkspaceManagerStore) *WorkspaceManager {
	return &WorkspaceManager{ContainerManager: cm, Store: store}
}

func (w WorkspaceManager) MakeContainerWorkspace(workspaceID string) (*ContainerWorkspace, error) {
	workspace, err := w.Store.GetWorkspace(workspaceID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	setupParams, err := w.Store.GetWorkspaceSetupParams(workspaceID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	paramsData, err := json.MarshalIndent(setupParams, "", " ")
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	workspaceMeta, err := w.Store.GetWorkspaceMeta(workspaceID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	workspaceData, err := json.MarshalIndent(workspaceMeta, "", " ")
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	secretsConfig, err := w.Store.GetWorkspaceSecretsConfig(workspaceID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	basePath := "/tmp/brev/volumes"
	workspaceVolumesPath := filepath.Join(basePath, workspace.ID)

	localMeta := filepath.Join(workspaceVolumesPath, "etc/meta")
	metaVolumes := NewStaticFiles("/etc/meta", map[string]io.Reader{
		"setup_v0.json":  bytes.NewBuffer(paramsData),
		"workspace.json": bytes.NewBuffer(workspaceData),
	}).
		WithPathPrefix(localMeta) // TODO proper path

	secretsLocalConfig := filepath.Join(workspaceVolumesPath, "etc/config")
	secretsConfigVolumes := NewStaticFiles("/etc/config", map[string]io.Reader{
		"config.hcl": bytes.NewBuffer([]byte(secretsConfig)),
	}).
		WithPathPrefix(secretsLocalConfig) // TODO proper path

	// may need to make tmp executable
	// need to create volume for fuse

	workspaceVolLocalPath := filepath.Join(workspaceVolumesPath, "home/brev/workspace")
	workspaceVol := SimpleVolume{
		Identifier:  workspaceVolLocalPath,
		MountToPath: "/home/brev/workspace",
	}

	k8sTokenVolPath := filepath.Join(workspaceVolumesPath, "var/run/kubernetes")
	k8sTokenVol := NewSymLinkVolume("/var/run/kubernetes", k8sTokenVolPath, "/var/run/kubernetes")

	containerWorkspace := NewContainerWorkspace(w.ContainerManager, workspaceID, workspace.WorkspaceTemplate.Image,
		[]Volume{
			metaVolumes,
			secretsConfigVolumes,
			workspaceVol,
			k8sTokenVol,
		},
	)

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
	if err != nil && !strings.Contains(err.Error(), "No such container") {
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
