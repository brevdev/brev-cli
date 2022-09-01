package store

import (
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

var workspaceGroupBase = "api/workspace_groups"

func (s AuthHTTPStore) GetWorkspaceGroups(organizationID string) ([]entity.WorkspaceGroup, error) {
	var result []entity.WorkspaceGroup
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetQueryParam("organizationId", organizationID).
		SetResult(&result).
		Get(workspaceGroupBase)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}
	return result, nil
}
