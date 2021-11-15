package store

import (
	"testing"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func setupSSHConfigFile(fs afero.Fs) error {
	sshConfigPath, err := files.GetUserSSHConfigPath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	_, err = fs.Create(*sshConfigPath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func TestGetSSHConfig(t *testing.T) {
	fs := MakeMockFileStore()
	err := setupSSHConfigFile(fs.fs)
	if !assert.Nil(t, err) {
		return
	}

	_, err = fs.GetSSHConfig()
	if !assert.Nil(t, err) {
		return
	}
}

func TestWriteSSHConfig(t *testing.T) {
	fs := MakeMockFileStore()
	err := fs.WriteSSHConfig("")
	if !assert.Nil(t, err) {
		return
	}
}

func TestCreateNewSSHConfigBackup(t *testing.T) {
	fs := MakeMockFileStore()
	err := setupSSHConfigFile(fs.fs)
	if !assert.Nil(t, err) {
		return
	}

	err = fs.CreateNewSSHConfigBackup()
	if !assert.Nil(t, err) {
		return
	}
}

func TestWritePrivateKey(t *testing.T) {
	fs := MakeMockFileStore()
	privateKey := "pk"
	err := fs.WritePrivateKey(privateKey)
	if !assert.Nil(t, err) {
		return
	}
}
