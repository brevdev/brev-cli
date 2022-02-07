package store

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/spf13/afero"
)

type FileStore struct {
	BasicStore
	fs afero.Fs
}

func (b *BasicStore) WithFileSystem(fs afero.Fs) *FileStore {
	return &FileStore{*b, fs}
}

func (f FileStore) GetOrCreateFile(path string) (afero.File, error) {
	fileExists, err := afero.Exists(f.fs, path)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	var file afero.File
	if fileExists {
		file, err = f.fs.OpenFile(path, os.O_RDWR, 0o644)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
	} else {
		if err = f.fs.MkdirAll(filepath.Dir(path), 0o775); err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		file, err = f.fs.Create(path)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
	}
	return file, nil
}

func (f FileStore) FileExists(filepath string) (bool, error) {
	fileExists, err := afero.Exists(f.fs, filepath)
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	return fileExists, nil
}

func (f FileStore) GetDotGitConfigFile() (string, error) {
	dotGitConfigFile := filepath.Join(".git", "config")

	dotGitConfigExists, err := afero.Exists(f.fs, dotGitConfigFile)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	var file afero.File
	if dotGitConfigExists {
		file, err = f.fs.OpenFile(dotGitConfigFile, os.O_RDWR, 0o644)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
	} else {
		return "", breverrors.WrapAndTrace(errors.New("couldn't verify if "+ dotGitConfigFile +" is an active git directory"))
	}

	buf := new(strings.Builder)
	_, err = io.Copy(buf, file)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return buf.String(), nil

}

func (f FileStore) IsRustProject() (bool, error) {
	filePath := filepath.Join("", "Cargo.lock")
	doesCargoLockExist, err := afero.Exists(f.fs, filePath)
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}

	filePath = filepath.Join("", "Cargo.toml")
	doesCargoTomlExist, err := afero.Exists(f.fs, filePath)
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}

	return doesCargoLockExist || doesCargoTomlExist, nil
}
