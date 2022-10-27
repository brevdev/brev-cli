package autostartconf

import (
	"os"
	"path"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type LinuxSystemdConfigurer struct {
	Store       AutoStartStore
	ExecString  string
	WantedBy    string
	After       string
	ServiceName string
	Type        string
	User        string
	ServiceType string
}

const (
	systemDConfigDir = "/etc/systemd/system/"
)

func (lsc LinuxSystemdConfigurer) ValueConfigFile() string {
	// [Unit]
	// Description=brevmon
	// After=network.target

	// [Service]
	// User=root
	// Type=exec
	// ExecStart=/usr/local/bin/brevmon
	// ExecReload=/usr/local/bin/brevmon
	// Restart=always

	// [Install]
	// WantedBy=default.target
	unit := `[Unit]
Description=` + lsc.ServiceName + `
After=` + lsc.After + `

[Service]`
	if lsc.User != "" {
		unit += `
User=` + lsc.User
	}
	unit += `Type=` + lsc.Type + `
ExecStart=` + lsc.ExecString + `
ExecReload=` + lsc.ExecString + `
Restart=always

[Install]
WantedBy=` + lsc.WantedBy
	return unit
}

func (lsc *LinuxSystemdConfigurer) WithFlags(flags []string) *LinuxSystemdConfigurer {
	for _, flag := range flags {
		lsc.ExecString += " " + flag
	}
	return lsc
}

func (lsc LinuxSystemdConfigurer) getDestConfigFile() string {
	return path.Join(systemDConfigDir, lsc.ServiceName)
}

func (lsc LinuxSystemdConfigurer) UnInstall() error {
	exists, err := lsc.Store.FileExists(lsc.getDestConfigFile())
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if exists {
		errother := lsc.Store.Remove(lsc.getDestConfigFile())
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
	err = lsc.Store.WriteString(lsc.getDestConfigFile(), lsc.ValueConfigFile())
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
	err := os.Symlink(lsc.getDestConfigFile(), symlinkTarget)
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
