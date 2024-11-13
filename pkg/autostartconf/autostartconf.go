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
