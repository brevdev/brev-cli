package start

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

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
