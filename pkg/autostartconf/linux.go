package autostartconf

import (
	"fmt"
	"os/exec"

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
	//nolint //this is never defined by a user
	out, err := exec.Command("systemctl", "enable", lsc.ServiceName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running systemctl enable %s: %v, %s", lsc.DestConfigFile, err, out)
	}
	//nolint //this is never defined by a user
	out, err = exec.Command("systemctl", "start", lsc.ServiceName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running systemctl start %s: %v, %s", lsc.DestConfigFile, err, out)
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
Description=Brev SSH Proxy Daemon
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

// todo user
func NewRPCConfig(store AutoStartStore) LinuxSystemdConfigurer {
	return LinuxSystemdConfigurer{
		Store: store,
		ValueConfigFile: `
[Install]
WantedBy=multi-user.target

[Unit]
Description=Brev SSH Proxy Daemon
After=systemd-user-sessions.service

[Service]
Type=simple
ExecStart=/usr/local/bin/brev task run rpcd --user` + store.GetOSUser() + `
Restart=always
`,
		DestConfigFile: "/etc/systemd/system/brevrpcd.service",
		ServiceName:    "brevrpcd",
	}
}
