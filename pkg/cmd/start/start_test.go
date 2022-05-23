package start

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/stretchr/testify/assert"
)

func TestMakeNewWorkspaceFromURL(t *testing.T) {
	gitTruth := "github.com:brevdev/brev-cli.git"
	nameTruth := "brev-cli"

	wksTruth := NewWorkspace{
		Name:    nameTruth,
		GitRepo: gitTruth,
	}

	naked := "https://github.com/brevdev/brev-cli"
	res := MakeNewWorkspaceFromURL(naked)
	if !assert.Equal(t, wksTruth, res) {
		return
	}

	http := "http://github.com/brevdev/brev-cli.git"
	res = MakeNewWorkspaceFromURL(http)
	if !assert.Equal(t, wksTruth, res) {
		return
	}

	https := "https://github.com/brevdev/brev-cli.git"
	res = MakeNewWorkspaceFromURL(https)
	if !assert.Equal(t, wksTruth, res) {
		return
	}

	ssh := "git@github.com:brevdev/brev-cli.git"
	res = MakeNewWorkspaceFromURL(ssh)
	if !assert.Equal(t, wksTruth, res) {
		return
	}
}

func Test_DisplayBC(t *testing.T) {
	term := terminal.New()
	displayConnectBreadCrumb(term, &entity.Workspace{
		ID:                "123456789",
		Name:              "my-name",
		WorkspaceGroupID:  "",
		OrganizationID:    "",
		WorkspaceClassID:  "",
		CreatedByUserID:   "",
		DNS:               "",
		Status:            "",
		Password:          "",
		GitRepo:           "",
		Version:           "",
		WorkspaceTemplate: entity.WorkspaceTemplate{},
		NetworkID:         "",
	})
}
