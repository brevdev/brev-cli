package version

import (
	"fmt"

	"github.com/fatih/color"

	"github.com/brevdev/brev-cli/pkg/requests"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

const (
	cliReleaseURL = "https://api.github.com/repos/brevdev/brev-cli/releases/latest"
)

var Version = ""

var green = color.New(color.FgGreen).SprintfFunc()

var upToDateString = `
Current version: %s

` + green("You're up to date!")

var outOfDateString = `
Current version: %s

` + green("A new version of brev has been released!") + `

Version: %s

Details: %s

` + green("run 'brew upgrade brevdev/tap/brev' to upgrade") + `

%s
`

type githubReleaseMetadata struct {
	TagName      string `json:"tag_name"`
	IsDraft      bool   `json:"draft"`
	IsPrerelease bool   `json:"prerelease"`
	Name         string `json:"name"`
	Body         string `json:"body"`
}

func BuildVersionString(t *terminal.Terminal) (string, error) {
	githubRelease, err := getLatestGithubReleaseMetadata()
	if err != nil {
		t.Errprint(err, "Failed to retrieve latest version")
		return "", err
	}

	var versionString string
	if githubRelease.TagName == Version {
		versionString = fmt.Sprintf(
			upToDateString,
			Version,
		)
	} else {
		versionString = fmt.Sprintf(
			outOfDateString,
			Version,
			githubRelease.TagName,
			githubRelease.Name,
			githubRelease.Body,
		)
	}
	return versionString, nil
}

func getLatestGithubReleaseMetadata() (*githubReleaseMetadata, error) {
	request := &requests.RESTRequest{
		Method:   "GET",
		Endpoint: cliReleaseURL,
	}
	response, err := request.SubmitStrict()
	if err != nil {
		return nil, err
	}

	var payload githubReleaseMetadata
	err = response.UnmarshalPayload(&payload)
	if err != nil {
		return nil, err
	}

	return &payload, nil
}
