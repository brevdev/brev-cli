package workspacemanagerv2

import (
	"context"
	"io"
	"os"
	"path/filepath"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type StaticFiles struct {
	FromMountPathPrefix string
	ToMountPath         string
	FileMap             map[string]io.Reader
}

var _ Volume = StaticFiles{}

func NewStaticFiles(path string, fileMap map[string]io.Reader) StaticFiles {
	return StaticFiles{ToMountPath: path, FileMap: fileMap}
}

func (s StaticFiles) WithPathPrefix(prefix string) StaticFiles {
	s.FromMountPathPrefix = prefix
	return s
}

func (s StaticFiles) GetIdentifier() string {
	return s.GetMountFromPath()
}

func (s StaticFiles) GetMountToPath() string {
	return s.ToMountPath
}

func (s StaticFiles) GetMountFromPath() string {
	return s.FromMountPathPrefix
}

func (s StaticFiles) Setup(_ context.Context) error {
	path := s.GetMountFromPath()
	err := os.MkdirAll(path, os.ModePerm)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	for k, v := range s.FileMap {
		filePath := filepath.Join(s.FromMountPathPrefix, k)
		f, err := os.Create(filePath) //nolint:gosec // executed in safe space
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		defer f.Close() //nolint:errcheck,gosec // defer

		_, err = io.Copy(f, v)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = f.Sync()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	return nil
}

func (s StaticFiles) Teardown(_ context.Context) error {
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

type SymLinkVolume struct {
	FromSymLinkPath string
	LocalVolumePath string
	MountToPath     string
}

var _ Volume = SymLinkVolume{}

func NewSymLinkVolume(fromSymLinkPath string, localVolumePath string, mountToPath string) *SymLinkVolume {
	return &SymLinkVolume{
		FromSymLinkPath: fromSymLinkPath,
		LocalVolumePath: localVolumePath,
		MountToPath:     mountToPath,
	}
}

func (s SymLinkVolume) GetIdentifier() string {
	return s.LocalVolumePath
}

func (s SymLinkVolume) GetMountToPath() string {
	return s.MountToPath
}

func (s SymLinkVolume) Setup(_ context.Context) error {
	err := os.Symlink(s.FromSymLinkPath, s.LocalVolumePath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (s SymLinkVolume) Teardown(_ context.Context) error {
	err := os.RemoveAll(s.LocalVolumePath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

type DynamicVolume struct {
	FromMountPathPrefix string
	ToMountPath         string
	FileMap             map[string]func(string)
}

// can be push or polled based?

var _ Volume = DynamicVolume{}

func NewDynamicVolume(path string, fileMap map[string]func(string)) *DynamicVolume {
	return &DynamicVolume{ToMountPath: path, FileMap: fileMap}
}

func (s DynamicVolume) WithPathPrefix(prefix string) DynamicVolume {
	s.FromMountPathPrefix = prefix
	return s
}

func (s DynamicVolume) GetIdentifier() string {
	return ""
}

func (s DynamicVolume) GetMountToPath() string {
	return ""
}

func (s DynamicVolume) GetMountFromPath() string {
	return s.FromMountPathPrefix
}

func (s DynamicVolume) Setup(_ context.Context) error {
	for f, d := range s.FileMap {
		d(filepath.Join(s.GetMountFromPath(), f))
	}
	return nil
}

func (s DynamicVolume) Teardown(_ context.Context) error {
	return nil
}
