package autostartconf

import (
	"runtime"
)

const targetBin = "/usr/local/bin/brev"

type AutoStartStore interface {
	CopyBin(targetBin string) error
	WriteString(path, data string) error
	GetOSUser() string
	UserHomeDir() (string, error)
	Remove(target string) error
}

type DaemonConfigurer interface {
	Install() error
	UnInstall() error
}

func NewVPNConfig(store AutoStartStore) DaemonConfigurer {
	switch runtime.GOOS {
	case "linux":
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
ExecStart=brev tasks run vpnd
Restart=always
`,
			DestConfigFile: "/etc/systemd/system/brevvpnd.service",
			ServiceName:    "brevvpnd",
			ServiceType:    "system",
		}
	case "darwin":
		return DarwinPlistConfigurer{
			Store: store,
			ValueConfigFile: `
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>

  <key>Label</key>
  <string>com.brev.vpnd</string>

  <key>ProgramArguments</key>
  <array>
    <string>brev</string>
	<string>tasks</string>
	<string>run</string>
	<string>vpnd</string>
  </array>

  <key>RunAtLoad</key>
  <true/>

</dict>
</plist>
			`,
			ServiceName: "com.brev.vpnd",
			ServiceType: System,
		}
	}
	return nil
}

func NewRPCConfig(store AutoStartStore) DaemonConfigurer {
	switch runtime.GOOS {
	case "linux":
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
ExecStart=brev tasks run rpcd --user ` + store.GetOSUser() + `
Restart=always
`,
			DestConfigFile: "/etc/systemd/system/brevrpcd.service",
			ServiceName:    "brevrpcd",
			ServiceType:    "system",
		}
	case "darwin":
		return DarwinPlistConfigurer{ // todo add user to rpcd
			Store: store,
			ValueConfigFile: `
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>

  <key>Label</key>
  <string>com.brev.rpcd</string>

  <key>ProgramArguments</key>
  <array>
    <string>brev</string>
	<string>tasks</string>
	<string>run</string>
	<string>rpcd</string>
	<string>--user</string>
	<string>` + store.GetOSUser() + `</string>
  </array>

  <key>RunAtLoad</key>
  <true/>

</dict>
</plist>
			`,
			ServiceName: "com.brev.rpcd",
			ServiceType: System,
		}
	}
	return nil
}
