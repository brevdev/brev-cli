package store

import (
	"fmt"
	"strconv"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

func (s AuthHTTPStore) RegisterNode(publicKey string) error {
	workspaceID, err := s.GetCurrentWorkspaceID()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if workspaceID != "" {
		err := s.registerWorkspaceNodeToNetwork(registerWorkspaceNodeRequest{
			WorkspaceID: workspaceID,
			PublicKey:   publicKey,
		})
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else {
		currentUser, err := s.GetCurrentUser()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		currentOrg, err := s.GetActiveOrganizationOrDefault()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = s.registerUserNodeToNetwork(registerUserNodeRequest{
			UserID:         currentUser.ID,
			OrganizationID: currentOrg.ID,
			PublicKey:      publicKey,
		})
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

var (
	networkBasePath = "/api/network"
	networkNodePath = fmt.Sprintf("%s/node", networkBasePath)
)

var networkWorkspacePath = fmt.Sprintf("%s/workspace/register", networkNodePath)

type registerWorkspaceNodeRequest struct {
	WorkspaceID string `json:"workspaceId"`
	PublicKey   string `json:"nodePublicKey"`
}

func (s AuthHTTPStore) registerWorkspaceNodeToNetwork(req registerWorkspaceNodeRequest) error {
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetBody(req).
		Post(networkWorkspacePath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return NewHTTPResponseError(res)
	}

	return nil
}

var networkUserPath = fmt.Sprintf("%s/user/register", networkNodePath)

type registerUserNodeRequest struct {
	UserID         string `json:"userId"`
	OrganizationID string `json:"organizationId"`
	PublicKey      string `json:"nodePublicKey"`
}

func (s AuthHTTPStore) registerUserNodeToNetwork(req registerUserNodeRequest) error {
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetBody(req).
		Post(networkUserPath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return NewHTTPResponseError(res)
	}

	return nil
}

type GetAuthKeyResponse struct {
	CoordServerURL string `json:"coordServerUrl"`
	AuthKey        string `json:"authKey"`
}

func (s AuthHTTPStore) GetNetworkAuthKey() (*GetAuthKeyResponse, error) {
	workspaceID, err := s.GetCurrentWorkspaceID()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if workspaceID != "" {
		var workspace *entity.Workspace
		workspace, err = s.GetWorkspace(workspaceID)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		var key *GetAuthKeyResponse
		key, err = s.GetNetworkAuthKeyByNetworkID(workspace.NetworkID, true)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		return key, nil
	}
	org, err := s.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	key, err := s.GetNetworkAuthKeyByNetworkID(org.UserNetworkID, false)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return key, nil
}

var (
	networkdIDParamName   = "networkID"
	networkPathPattern    = fmt.Sprintf("%s/%s", networkBasePath, "%s")
	networkPath           = fmt.Sprintf(networkPathPattern, fmt.Sprintf("{%s}", networkdIDParamName))
	networkKeyPathPattern = fmt.Sprintf("%s/key", networkPathPattern)
	networkKeyPath        = fmt.Sprintf(networkKeyPathPattern, fmt.Sprintf("{%s}", networkdIDParamName))
)

func (s AuthHTTPStore) GetNetworkAuthKeyByNetworkID(networkID string, ephemeral bool) (*GetAuthKeyResponse, error) {
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetQueryParam("ephemeral", strconv.FormatBool(ephemeral)).
		SetPathParam(networkdIDParamName, networkID).
		Get(networkKeyPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return nil, nil
}
