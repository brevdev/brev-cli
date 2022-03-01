package autostartconf

import "github.com/brevdev/brev-cli/pkg/store"

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
	unitFileDest = "/etc/systemd/system/"
)

type LinuxSystemdConfigurer struct {
	store.FileStore
	ValueConfigFile string
	DestConfigFile  string
}

func (lsc LinuxSystemdConfigurer) UnInstall() error {
	return nil
}

func (lsc LinuxSystemdConfigurer) Install() error {
	_ = lsc.UnInstall() // best effort
	return nil
}
