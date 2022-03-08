package store

import (
	"fmt"
	"strconv"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

func (s AuthHTTPStore) RegisterNode(publicKey string) error {
	workspaceID := s.GetCurrentWorkspaceID()
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
	networkPath     = "/api/network"
	networkNodePath = fmt.Sprintf("%s/node", networkPath)
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
	CoordServerURL string
	AuthKey        string
}

func (s AuthHTTPStore) GetNetworkAuthKey() (*GetAuthKeyResponse, error) {
	return nil, nil
}

func (s AuthHTTPStore) GetNetworkAuthKeyReq(networkID string, ephemeral bool) (*GetAuthKeyResponse, error) {
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetQueryParam("ephemeral", strconv.FormatBool(ephemeral)).
		Get(networkPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return nil, nil
}
