package autostartconf

import (
	"fmt"
	"os/exec"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

const linuxSystemdUnitFile = `
[Install]
WantedBy=multi-user.target

[Unit]
Description=Brev SSH Proxy Daemon
After=systend-user-sessions.service

[Service]
Type=simple
ExecStart=brev run-tasks
Restart=always
`

const (
	unitFileDest = "/etc/systemd/system/brev.service"
)

type LinuxSystemdConfigurer struct {
	AutoStartStore
	ValueConfigFile string
	DestConfigFile  string
}

func (lsc LinuxSystemdConfigurer) UnInstall() error {
	return nil
}

func (lsc LinuxSystemdConfigurer) Install() error {
	_ = lsc.UnInstall() // best effort
	err := lsc.CopyBin(targetBin)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = lsc.WriteString(lsc.DestConfigFile, lsc.ValueConfigFile)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	out, err := exec.Command("systemctl", "enable", "brev").CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running systemctl enable %s: %v, %s", lsc.DestConfigFile, err, out)
	}
	out, err = exec.Command("systemctl", "start", "brev").CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running systemctl start %s: %v, %s", lsc.DestConfigFile, err, out)
	}
	return nil
}
