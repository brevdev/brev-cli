package brevapi

import (
	"errors"
	"fmt"

	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/deprecatedauth"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/requests"
)

type DeprecatedClient struct {
	Key *deprecatedauth.OauthToken
}

func HandleNewClientErrors(err error) error {
	switch e := err.(type) {
	case *breverrors.CredentialsFileNotFound:
		_, err = deprecatedauth.Login(true)
		return breverrors.WrapAndTrace(err)
	case *requests.RESTResponseError:
		switch e.ResponseStatusCode {
		case 404: // happens when user signs in to the cli using github but does not have an account on brev
			return fmt.Errorf("create an account on https://console.brev.dev")
		case 403: // possibly malformed credentials.json, try logging in
			_, err = deprecatedauth.Login(true)
			return breverrors.WrapAndTrace(err)
		}
	}
	return breverrors.WrapAndTrace(err)
}

func NewDeprecatedClient() (*DeprecatedClient, error) {
	// get token and use it to create a client
	token, err := deprecatedauth.GetToken()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	client := DeprecatedClient{
		Key: token,
	}
	return &client, nil
}

func NewCommandClient() (*DeprecatedClient, error) {
	var client *DeprecatedClient
	var err error
	client, err = NewDeprecatedClient()
	if err != nil {
		err = HandleNewClientErrors(err)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		} else {
			client, err = NewDeprecatedClient()
			if err != nil {
				return nil, breverrors.WrapAndTrace(err)
			}
		}
	}
	return client, breverrors.WrapAndTrace(err)
}

func buildBrevEndpoint(resource string) string {
	baseEndpoint := config.GlobalConfig.GetBrevAPIURl()
	return baseEndpoint + resource
}

func IsInProjectDirectory() (bool, error) {
	return false, nil
}

func StringInList(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func GetOrgFromName(name string) (*Organization, error) {
	client, err := NewCommandClient()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	// Get all orgs
	orgs, err := client.GetOrgs()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	for _, o := range orgs {
		if o.Name == name {
			return &o, nil
		}
	}

	return nil, breverrors.WrapAndTrace(errors.New("no organization with that name"))
}

func GetWorkspaceFromName(name string) (*Workspace, error) {
	client, err := NewCommandClient()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	activeOrg, err := GetActiveOrgContext(files.AppFs)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	if err != nil {
		// Get all orgs
		orgs, err2 := client.GetOrgs()
		if err2 != nil {
			return nil, err2
		}
		for _, o := range orgs {
			wss, err3 := client.GetWorkspaces(o.ID)
			if err3 != nil {
				return nil, err3
			}
			for _, w := range wss {
				if w.Name == name {
					return &w, nil
				}
			}
		}
	}
	// If active org, get all ActiveOrg workspaces
	wss, err := client.GetMyWorkspaces(activeOrg.ID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	for _, v := range wss {
		if v.Name == name {
			return &v, nil
		}
	}

	return nil, breverrors.WrapAndTrace(errors.New("no workspace with that name"))
}

func GetCachedWorkspaceNames() []string {
	activeOrg, err := GetActiveOrgContext(files.AppFs)
	if err != nil {
		return nil
	}

	cachedWorkspaces, err := GetWsCacheData()
	if err != nil {
		return nil
	}

	var wsNames []string
	for _, cw := range cachedWorkspaces {
		if cw.OrgID == activeOrg.ID {
			for _, w := range cw.Workspaces {
				wsNames = append(wsNames, w.Name)
			}
			return wsNames
		}
	}

	return nil
}

func GetOrgNames() []string {
	cachedOrgs, err := GetOrgCacheData()
	if err != nil {
		return nil
	}

	// orgs  := getOrgs()
	var orgNames []string
	for _, v := range cachedOrgs {
		orgNames = append(orgNames, v.Name)
	}

	return orgNames
}

func GetWorkspaceNames() []string {
	activeOrg, err := GetActiveOrgContext(files.AppFs)
	if err != nil {
		return nil
	}

	client, err := NewCommandClient()
	if err != nil {
		return nil
	}
	wss, err := client.GetMyWorkspaces(activeOrg.ID)
	if err != nil {
		return nil
	}

	var wsNames []string
	for _, w := range wss {
		wsNames = append(wsNames, w.Name)
	}

	return wsNames
}

// func PollWorkspaceForReadyState()

func GetMe() (*User, error) {
	client, err := NewCommandClient()
	if err != nil {
		return nil, err
	}
	user, err := client.GetMe()
	if err != nil {
		return nil, err
	}
	return user, nil
}
