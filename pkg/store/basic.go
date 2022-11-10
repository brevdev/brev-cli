package store

import (
	"fmt"
	"os"
	otherpath "path"
	"runtime"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type BasicStore struct {
	envGetter func(string) string
}

func NewBasicStore() *BasicStore {
	return &BasicStore{
		envGetter: os.Getenv,
	}
}

// look in path on wsl to find out which user is the user on windows that is running wsl
// PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/games:/usr/local/games:/usr/lib/wsl/lib:/mnt/c/WINDOWS/system32:/mnt/c/WINDOWS:/mnt/c/WINDOWS/System32/Wbem:/mnt/c/WINDOWS/System32/WindowsPowerShell/v1.0/:/mnt/c/WINDOWS/System32/OpenSSH/:/mnt/c/Users/15854/AppData/Local/Microsoft/WindowsApps:/mnt/c/Users/15854/AppData/Local/Programs/Microsoft VS Code/bin:/snap/bin
func (b BasicStore) GetWSLHostHomeDir() (string, error) {
	if runtime.GOOS == "windows" {
		return "", breverrors.New("not supported on windows")
	}
	if runtime.GOOS == "linux" {
		path := b.envGetter("PATH")
		if path == "" {
			return "", breverrors.New("PATH is empty")
		}
		username := ""
		pathSplit := strings.Split(path, ":")
		for _, p := range pathSplit {
			if strings.Contains(p, "/mnt/c/Users/") {
				username = strings.Split(p, "/")[4]
				break
			}
		}
		if username == "" {
			return "", breverrors.New("could not find username")
		}
		windowsDir := "/mnt/c/Users/"
		windowsUserDir := otherpath.Join(windowsDir, username)
		return windowsUserDir, nil
	}
	return "", fmt.Errorf("not supported on %s", runtime.GOOS)
}
