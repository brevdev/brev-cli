package brev_api

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/requests"
)

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
		switch err := err.(type) {
		case *requests.RESTResponseError:
			switch err.ResponseStatusCode {
			case 404: // happens when user signs in to the cli using github but does not have an account on brev
				return nil, fmt.Errorf("Create an account on https://console.brev.dev")
			case 403: // possibly malformed credentials.json, try logging in
				// TODO prompt
				err := Login()
				if err != nil {
					return nil, err
				}
				return NewClient()
			}
		}

		return nil, err
	}
	if user != nil {
		return &client, nil
	}
	return nil, fmt.Errorf("error creating client")
}

type Client struct {
	Key *OauthToken
}

func brevAlphaEndpoint(resource string) string {
	baseEndpoint := config.GetBrevALPHAAPIEndpoint()
	return baseEndpoint + "/_api/" + resource
}

func brevEndpoint(resource string) string {
	baseEndpoint := config.GetBrevAPIEndpoint()
	return baseEndpoint + resource
}

// Example usage
/*
	token, _ := auth.GetToken()
	brevAgent := brev_api.Agent{
		Key: token,
	}

	endpointsResponse, _ := brevAgent.GetEndpoints()
	fmt.Println(endpointsResponse)

	projectsResponse, _ := brevAgent.GetProjects()
	fmt.Println(projectsResponse)

	modulesResponse, _ := brevAgent.GetModules()
	fmt.Println(modulesResponse)
*/

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
