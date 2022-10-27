package autostartconf

import (
	"path/filepath"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type AptBinaryConfigurer struct {
	LinuxSystemdConfigurer
	URL             string
	Name            string
	aptDependencies []string
}

func (abc AptBinaryConfigurer) Install() error {
	_ = abc.UnInstall() // best effort

	// install apt dependencies
	err := execCommands([][]string{
		{"apt-get", "update"},
		append([]string{"apt-get", "install", "-y"}, abc.aptDependencies...),
	})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	// download binary
	err = abc.Store.DownloadBinary(
		abc.URL,
		filepath.Join("/usr/local/bin", abc.Name),
	)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = abc.Store.WriteString(abc.getDestConfigFile(), abc.ValueConfigFile())
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if ShouldSymlink() {
		errother := abc.CreateForcedSymlink()
		if errother != nil {
			return breverrors.WrapAndTrace(errother)
		}
	} else {
		errother := execCommands([][]string{
			{"systemctl", "enable", abc.ServiceName},
			{"systemctl", "start", abc.ServiceName},
			{"systemctl", "daemon-reload"},
		})
		if errother != nil {
			return breverrors.WrapAndTrace(errother)
		}
	}
	return nil
}
