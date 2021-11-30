package version

import (
	"fmt"

	"github.com/fatih/color"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
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

type VersionStore interface {
	GetLatestReleaseMetadata() (*store.GithubReleaseMetadata, error)
}

func BuildVersionString(t *terminal.Terminal, versionStore VersionStore) (string, error) {
	githubRelease, err := versionStore.GetLatestReleaseMetadata()
	if err != nil {
		t.Errprint(err, "Failed to retrieve latest version")
		return "", breverrors.WrapAndTrace(err)
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
