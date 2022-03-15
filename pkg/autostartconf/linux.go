package autostartconf

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type LinuxSystemdConfigurer struct {
	Store           AutoStartStore
	ValueConfigFile string
	DestConfigFile  string
	ServiceName     string
}

func (lsc LinuxSystemdConfigurer) UnInstall() error {
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

	workspaceID, err := lsc.Store.GetCurrentWorkspaceID()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if workspaceID == "" {
		err := lsc.Enable()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = lsc.Start()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else {
		lsc.CreateForcedSymlink()
	}
	return nil
}

func (lsc LinuxSystemdConfigurer) Enable() error {
	//nolint //this is never defined by a user
	out, err := exec.Command("systemctl", "enable", lsc.ServiceName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running systemctl enable %s: %v, %s", lsc.DestConfigFile, err, out)
	}
	return nil
}

func (lsc LinuxSystemdConfigurer) Start() error {
	//nolint //this is never defined by a user
	out, err := exec.Command("systemctl", "start", lsc.ServiceName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running systemctl start %s: %v, %s", lsc.DestConfigFile, err, out)
	}
	return nil
}

// CreateForcedSymlink aims to be the equivalent operation as running
// ln -sf /lib/systemd/system/huproxy.service /etc/systemd/system/default.target.wants/huproxy.service
// which overwrite's an existing symbolic link to point to a different file
// which we need to do in the workspace docker image because systemd is running
// at build time.
func (lsc LinuxSystemdConfigurer) CreateForcedSymlink() error {
	serviceType := strings.Split(lsc.DestConfigFile, "/")[2]
	symlinkTarget := ""
	switch serviceType {
	case "system":
		symlinkTarget = path.Join("/etc/systemd/system/default.target.wants/", lsc.ServiceName+".service")
	case "user":
		symlinkTarget = path.Join("/etc/systemd/user/default.target.wants/", lsc.ServiceName+".service")
	}
	_, err := os.Stat(symlinkTarget)
	if err == nil { // file doesn't exist
		errother := os.Remove(symlinkTarget)
		if errother != nil {
			return breverrors.WrapAndTrace(errother)
		}
	}
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = os.Symlink(lsc.DestConfigFile, symlinkTarget)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func NewVPNConfig(store AutoStartStore) LinuxSystemdConfigurer {
	return LinuxSystemdConfigurer{
		Store: store,
		ValueConfigFile: `
[Install]
WantedBy=multi-user.target

[Unit]
Description=Brev vpn daemon
After=systemd-user-sessions.service

[Service]
Type=simple
ExecStart=/usr/local/bin/brev meshd
Restart=always
`,
		DestConfigFile: "/etc/systemd/system/brevvpnd.service",
		ServiceName:    "brevvpnd",
	}
}

func NewRPCConfig(store AutoStartStore) LinuxSystemdConfigurer {
	return LinuxSystemdConfigurer{
		Store: store,
		ValueConfigFile: `
[Install]
WantedBy=multi-user.target

[Unit]
Description=Brev rpc daemon
After=systemd-user-sessions.service

[Service]
Type=simple
ExecStart=/usr/local/bin/brev task run rpcd --user ` + store.GetOSUser() + `
Restart=always
`,
		DestConfigFile: "/etc/systemd/system/brevrpcd.service",
		ServiceName:    "brevrpcd",
	}
}
