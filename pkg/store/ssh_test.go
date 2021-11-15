package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSSHConfig(t *testing.T) {
	fs := MakeMockFileStore()
	_, err := fs.GetSSHConfig()
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
	err := fs.CreateNewSSHConfigBackup()
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
