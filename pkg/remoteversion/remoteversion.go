package remoteversion

import (
	"fmt"

	"github.com/fatih/color"

	"github.com/brevdev/brev-cli/pkg/cmd/version"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

var green = color.New(color.FgGreen).SprintfFunc()

var (
	upToDateString  = "Current Version: %s\n"
	outOfDateString = green("A new version of brev has been released!\n") +
		`Current version: %s
New Version: %s
To update to latest version, use:
	brew upgrade brevdev/homebrew-brev/brev`
)

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
		t.Errprint(err, "Failed to retrieve latest version")
		return "", breverrors.WrapAndTrace(err)
	}

	versionString := ""
	if githubRelease.TagName != version.Version {
		versionString = fmt.Sprintf(
			outOfDateString,
			version.Version,
			githubRelease.TagName,
		) + "\n\n"
	}
	return versionString, nil
}
