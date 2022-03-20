package autostartconf

import (
	"os"
	"path"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type LinuxSystemdConfigurer struct {
	Store           AutoStartStore
	ValueConfigFile string
	DestConfigFile  string
	ServiceName     string
	ServiceType     string
}

func (lsc LinuxSystemdConfigurer) UnInstall() error {
	exists, err := lsc.Store.FileExists(lsc.DestConfigFile)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if exists {
		errother := lsc.Store.Remove(lsc.DestConfigFile)
		if errother != nil {
			return breverrors.WrapAndTrace(errother)
		}
	}
	err = execCommands([][]string{
		{"systemctl", "disable", lsc.ServiceName},
		{"systemctl", "stop", lsc.ServiceName},
	})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (lsc LinuxSystemdConfigurer) Install() error {
	_ = lsc.UnInstall() // best effort
	err := lsc.Store.CopyBin(targetBin)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = lsc.Store.WriteString(lsc.DestConfigFile, lsc.ValueConfigFile)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if ShouldSymlink() {
		errother := lsc.CreateForcedSymlink()
		if errother != nil {
			return breverrors.WrapAndTrace(errother)
		}
	} else {
		errother := execCommands([][]string{
			{"systemctl", "enable", lsc.ServiceName},
			{"systemctl", "start", lsc.ServiceName},
			{"systemctl", "daemon-reload"},
		})
		if errother != nil {
			return breverrors.WrapAndTrace(errother)
		}
	}
	return nil
}

// CreateForcedSymlink aims to be the equivalent operation as running
// ln -sf /lib/systemd/system/huproxy.service /etc/systemd/system/default.target.wants/huproxy.service
// which overwrite's an existing symbolic link to point to a different file
// which we need to do in the workspace docker image because systemd isn't running
// at build time.
func (lsc LinuxSystemdConfigurer) CreateForcedSymlink() error {
	symlinkTarget := path.Join("/etc/systemd/system/default.target.wants/", lsc.ServiceName)
	err := os.Symlink(lsc.DestConfigFile, symlinkTarget)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func ShouldSymlink() bool {
	if os.Getenv("SHOULD_SYMLINK") != "" {
		return os.Getenv("SHOULD_SYMLINK") == "1"
	}
	return false
}
