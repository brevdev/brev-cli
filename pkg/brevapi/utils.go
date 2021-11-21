package brevapi

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/requests"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/manifoldco/promptui"
)

type Client struct {
	Key *OauthToken
}

func HandleNewClientErrors(err error) error {
	switch e := err.(type) {
	case *breverrors.CredentialsFileNotFound:
		// TODO prompt
		_, err = Login(true)
		return err
	case *requests.RESTResponseError:
		switch e.ResponseStatusCode {
		case 404: // happens when user signs in to the cli using github but does not have an account on brev
			return fmt.Errorf("create an account on https://console.brev.dev")
		case 403: // possibly malformed credentials.json, try logging in
			_, err = Login(true)
			return err
		}
	}
	return breverrors.WrapAndTrace(err)
}

func NewClient() (*Client, error) {
	// get token and use it to create a client
	token, err := GetToken()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	client := Client{
		Key: token,
	}
	return &client, nil
}

func NewCommandClient() (*Client, error) {
	var client *Client
	var err error
	client, err = NewClient()
	if err != nil {
		err = HandleNewClientErrors(err)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		} else {
			client, err = NewClient()
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
	err := exec.Command(cmd, args...).Start() //#nosec variables are hard coded
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
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

type NewWorkspace struct {
	Name    string `json:"name"`
	GitRepo string `json:"gitRepo"`
}

func ValidateGitURL(_ *terminal.Terminal, url string) NewWorkspace {
	// gitlab.com:mygitlaborg/mycoolrepo.git
	if strings.Contains(url, "http") {
		split := strings.Split(url, ".com/")
		provider := strings.Split(split[0], "://")[1]

		if strings.Contains(split[1], ".git") {
			return NewWorkspace{
				GitRepo: fmt.Sprintf("%s.com:%s", provider, split[1]),
				Name:    strings.Split(split[1], ".git")[0],
			}
		} else {
			return NewWorkspace{
				GitRepo: fmt.Sprintf("%s.com:%s.git", provider, split[1]),
				Name:    split[1],
			}
		}
	} else {
		split := strings.Split(url, ".com:")
		provider := strings.Split(split[0], "@")[1]
		return NewWorkspace{
			GitRepo: fmt.Sprintf("%s.com:%s", provider, split[1]),
			Name:    strings.Split(split[1], ".git")[0],
		}
	}
}

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

type PromptContent struct {
	ErrorMsg string
	Label    string
	Default  string
}

func PromptGetInput(pc PromptContent) string {
	validate := func(input string) error {
		if len(input) == 0 {
			return breverrors.WrapAndTrace(errors.New(pc.ErrorMsg))
		}
		return nil
	}

	templates := &promptui.PromptTemplates{
		Prompt:  "{{ . }} ",
		Valid:   "{{ . | green }} ",
		Invalid: "{{ . | yellow }} ",
		Success: "{{ . | bold }} ",
	}

	prompt := promptui.Prompt{
		Label:     pc.Label,
		Templates: templates,
		Validate:  validate,
		Default:   pc.Default,
		AllowEdit: true,
	}

	result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		os.Exit(1)
	}

	return result
}

type PromptSelectContent struct {
	ErrorMsg string
	Label    string
	Items    []string
}

func PromptSelectInput(pc PromptSelectContent) string {
	// templates := &promptui.SelectTemplates{
	// 	Label:  "{{ . }} ",
	// 	Selected:   "{{ . | green }} ",
	// 	Active: "{{ . | cyan }} ",
	// }

	prompt := promptui.Select{
		Label: pc.Label,
		Items: pc.Items,
		// Templates: templates,
	}

	_, result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		os.Exit(1)
	}

	return result
}
