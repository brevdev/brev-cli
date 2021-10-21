package brev_api

import (
	"os/exec"
	"runtime"

	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/config"
)

type Client struct {
	Key *auth.OauthToken
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
