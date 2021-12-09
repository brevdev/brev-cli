package store

import (
	"os"

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

func (f FileStore) GetOrCreateFile(filepath string) (afero.File, error) {
	fileExists, err := afero.Exists(f.fs, filepath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	var file afero.File
	if fileExists {
		file, err = f.fs.OpenFile(filepath, os.O_RDWR, 0o644)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
	} else {
		file, err = f.fs.Create(filepath)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
	}
	return file, nil
}
