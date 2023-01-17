package autostartconf

import (
	"path/filepath"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type StaticBinaryConfigurer struct {
	LinuxSystemdConfigurer
	URL  string
	Name string
}

func (sbc StaticBinaryConfigurer) Install() error {
	_ = sbc.UnInstall() // best effort

	// download binary
	err := sbc.Store.DownloadBinary(
		sbc.URL,
		filepath.Join("/usr/local/bin", sbc.Name),
	)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = sbc.Store.WriteString(sbc.getDestConfigFile(), sbc.ValueConfigFile)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if ShouldSymlink() {
		errother := sbc.CreateForcedSymlink()
		if errother != nil {
			return breverrors.WrapAndTrace(errother)
		}
	} else {
		errother := ExecCommands([][]string{
			{"systemctl", "enable", sbc.ServiceName},
			{"systemctl", "start", sbc.ServiceName},
			{"systemctl", "daemon-reload"},
		})
		if errother != nil {
			return breverrors.WrapAndTrace(errother)
		}
	}
	return nil
}
