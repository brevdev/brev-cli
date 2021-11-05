package brev_api

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/brevdev/brev-cli/pkg/brev_errors"
	"github.com/brevdev/brev-cli/pkg/config"
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
