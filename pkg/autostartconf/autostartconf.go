package autostartconf

import (
	"fmt"
	"os/exec"
	"runtime"
)

const (
	targetBin = "/usr/local/bin/brev"
	osLinux   = "linux"
	osDarwin  = "darwin"
)

type AutoStartStore interface {
	CopyBin(targetBin string) error
	WriteString(path, data string) error
	GetOSUser() string
	UserHomeDir() (string, error)
	Remove(target string) error
	FileExists(target string) (bool, error)
	DownloadBinary(url string, target string) error
}

type DaemonConfigurer interface {
	Install() error
	UnInstall() error
}

func ExecCommands(commands [][]string) error {
	for _, command := range commands {
		first, rest := firstAndRest(command)
		out, err := exec.Command(first, rest...).CombinedOutput() // #nosec G204
		if err != nil {
			return fmt.Errorf("error running %s %s: %v, %s", first, fmt.Sprint(command), err, out)
		}
	}
	return nil
}

func firstAndRest(commandstring []string) (string, []string) {
	first := commandstring[0]
	rest := commandstring[1:]
	return first, rest
}

func NewVPNConfig(store AutoStartStore) DaemonConfigurer {
	switch runtime.GOOS {
	case osLinux:
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
			ServiceName: "brevvpnd.service",
			ServiceType: "system",
			TargetBin:   targetBin,
		}
	case osDarwin:
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
    <string>/usr/local/bin/brev</string>
	<string>tasks</string>
	<string>run</string>
	<string>vpnd</string>
  </array>

  <key>RunAtLoad</key>
  <true/>

  <key>StandardOutPath</key>
  <string>/var/log/brevvpnd.log</string>
  <key>StandardErrorPath</key>
  <string>/var/log/brevvpnd.log</string>

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
	case osLinux:
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
ExecStart=brev tasks run rpcd --user ` + store.GetOSUser() + `
Restart=always
`,
			ServiceName: "brevrpcd.service",
			ServiceType: "system",
		}
	case osDarwin:
		return DarwinPlistConfigurer{
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
    <string>/usr/local/bin/brev</string>
	<string>tasks</string>
	<string>run</string>
	<string>rpcd</string>
	<string>--user</string>
	<string>` + store.GetOSUser() + `</string>
  </array>

  <key>RunAtLoad</key>
  <true/>

  <key>StandardOutPath</key>
  <string>/var/log/brevrpcd.log</string>
  <key>StandardErrorPath</key>
  <string>/var/log/brevrpcd.log</string>

</dict>
</plist>
			`,
			ServiceName: "com.brev.rpcd",
			ServiceType: System,
		}
	}
	return nil
}

// func NewDaemonConfiguration(user string, command string, serviceName string, serviceType string) DaemonConfigurer {
// 	switch runtime.GOOS {
// 	case osLinux:
// 		return LinuxSystemdConfigurer{
// 		}
// 	case osDarwin:
// 	}
// }

func NewSSHConfigurer(store AutoStartStore) DaemonConfigurer {
	switch runtime.GOOS {
	case osLinux:
		return LinuxSystemdConfigurer{
			Store: store,
			ValueConfigFile: `
[Install]
WantedBy=multi-user.target

[Unit]
Description=Brev ssh configurer daemon
After=systemd-user-sessions.service

[Service]
Type=simple
ExecStart=brev tasks run sshcd --user ` + store.GetOSUser() + `
Restart=always
User=` + store.GetOSUser() + `
`,
			ServiceName: "brevsshcd.service",
			ServiceType: "user",
			TargetBin:   targetBin,
		}
	case osDarwin:
		return DarwinPlistConfigurer{
			Store: store,
			ValueConfigFile: `
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>

  <key>Label</key>
  <string>com.brev.sshcd</string>

  <key>ProgramArguments</key>
  <array>
    <string>/usr/local/bin/brev</string>
	<string>tasks</string>
	<string>run</string>
	<string>sshcd</string>
	<string>--user</string>
	<string>` + store.GetOSUser() + `</string>
  </array>

  <key>RunAtLoad</key>
  <true/>

  <key>StandardOutPath</key>
  <string>/var/log/brevsshcd.log</string>
  <key>StandardErrorPath</key>
  <string>/var/log/brevsshcd.log</string>

</dict>
</plist>
			`,
			ServiceName: "com.brev.sshcd",
			ServiceType: SingleUser,
		}
	}
	return nil
}

func NewBrevMonConfigure(
	store AutoStartStore,
	disableAutostop bool,
	reportInterval string,
	portToCheckTrafficOn string,
) DaemonConfigurer {
	configFile := fmt.Sprintf(`[Unit]
	Description=brevmon
	After=network.target
	
	[Service]
	User=root
	Type=exec
	ExecStart=/usr/local/bin/brevmon %s
	ExecReload=/usr/local/bin/brevmon %s
	Restart=always
	
	[Install]
	WantedBy=default.target
	`, portToCheckTrafficOn, portToCheckTrafficOn)
	if disableAutostop {
		configFile = fmt.Sprintf(`[Unit]
Description=brevmon
After=network.target

[Service]
User=root
Type=exec
ExecStart=/usr/local/bin/brevmon %s --disable-autostop --report-interval `+reportInterval+`
ExecReload=/usr/local/bin/brevmon %s --disable-autostop --report-interval `+reportInterval+`
Restart=always

[Install]
WantedBy=default.target
`, portToCheckTrafficOn, portToCheckTrafficOn)
	}
	return AptBinaryConfigurer{
		LinuxSystemdConfigurer: LinuxSystemdConfigurer{
			Store:           store,
			ValueConfigFile: configFile,
			ServiceName:     "brevmon.service",
			ServiceType:     "system",
		},

		URL:  "https://s3.amazonaws.com/brevmon.brev.dev/brevmon.tar.gz",
		Name: "brevmon",
		aptDependencies: []string{
			"libpcap-dev",
		},
	}
}
