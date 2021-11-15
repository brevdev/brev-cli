package store

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestWithFileSystem(t *testing.T) {
	fs := MakeMockFileStore()
	if !assert.NotNil(t, fs) {
		return
	}
}

func MakeMockFileStore() *FileStore {
	bs := MakeMockBasicStore()
	fs := bs.WithFileSystem(afero.NewMemMapFs())
	return fs
}
