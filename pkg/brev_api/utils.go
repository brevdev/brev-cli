package brev_api

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/brevdev/brev-cli/pkg/brev_errors"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/requests"
)

type Client struct {
	Key *OauthToken
}

func HandleNewClientErrors(err error) error {
	switch e := err.(type) {
	case *brev_errors.CredentialsFileNotFound:
		// TODO prompt
		return Login(true)
	case *requests.RESTResponseError:
		switch e.ResponseStatusCode {
		case 404: // happens when user signs in to the cli using github but does not have an account on brev
			return fmt.Errorf("Create an account on https://console.brev.dev")
		case 403: // possibly malformed credentials.json, try logging in
			return Login(true)
		}
	}
	return err
}

func NewClient() (*Client, error) {
	// get token and use it to create a client
	token, err := GetToken()
	if err != nil {
		return nil, err
	}
	client := Client{
		Key: token,
	}
	// make sure the token we have is associated with a valid user
	user, err := client.GetMe()
	if err != nil {
		return nil, err
	}
	if user != nil {
		return &client, nil
	}
	return nil, fmt.Errorf("error creating client")
}

func NewCommandClient() (*Client, error) {
	var client *Client
	var err error
	client, err = NewClient()
	if err != nil {
		err = HandleNewClientErrors(err)
		if err != nil {
			return nil, err
		} else {
			client, err = NewClient()
			if err != nil {
				return nil, err
			}
		}
	}
	return client, err
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

// open opens the specified URL in the default browser of the user.
func OpenBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

func GetOrgFromName(name string) (*Organization, error) {
	client, err := NewCommandClient()
	if err != nil {
		return nil, err
	}

	// Get all orgs
	orgs, err := client.GetOrgs()
	if err != nil {
		return nil, err
	}

	for _, o := range orgs {
		if o.Name == name {
			return &o, nil
		}
	}

	return nil, errors.New("no organization with that name")
}

func GetWorkspaceFromName(name string) (*Workspace, error) {
	client, err := NewCommandClient()
	if err != nil {
		return nil, err
	}
	activeOrg, err := GetActiveOrgContext(files.AppFs)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	for _, v := range wss {
		if v.Name == name {
			return &v, nil
		}
	}

	return nil, errors.New("no workspace with that name")
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
