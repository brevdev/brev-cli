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
	fileMap             map[string]io.Reader
}

var _ Volume = StaticFiles{}

func NewStaticFiles(path string, fileMap map[string]io.Reader) StaticFiles {
	return StaticFiles{ToMountPath: path, fileMap: fileMap}
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

	for k, v := range s.fileMap {
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
