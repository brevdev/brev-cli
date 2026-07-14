package remoteversion

import (
	"fmt"

	"github.com/fatih/color"

	"github.com/brevdev/brev-cli/pkg/cmd/version"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

var green = color.New(color.FgGreen).SprintfFunc()

var (
	upToDateString  = "Current Version: %s"
	outOfDateString = green("A new version of brev has been released!\n") +
		`Current version: %s
New Version: %s
To update to latest version run:

brev upgrade`
)

type VersionStore interface {
	GetLatestReleaseMetadata() (*store.GithubReleaseMetadata, error)
}

func BuildVersionString(t *terminal.Terminal, versionStore VersionStore) (string, error) {
	githubRelease, err := versionStore.GetLatestReleaseMetadata()
	if err != nil {
		return fmt.Sprintf(upToDateString, version.Version), nil
	}

	var versionString string
	if githubRelease.TagName == version.Version {
		versionString = fmt.Sprintf(
			upToDateString,
			version.Version,
		)
	} else {
		versionString = fmt.Sprintf(
			outOfDateString,
			version.Version,
			githubRelease.TagName,
		)
	}
	return versionString, nil
}

func BuildCheckLatestVersionString(t *terminal.Terminal, versionStore VersionStore) (string, error) {
	githubRelease, err := versionStore.GetLatestReleaseMetadata()
	if err != nil {
		return "", nil
	}

	versionString := ""
	if githubRelease.TagName != version.Version {
		versionString = fmt.Sprintf(
			outOfDateString,
			version.Version,
			githubRelease.TagName,
		) + "\n"
	}
	return versionString, nil
}
