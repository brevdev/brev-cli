//go:build windows

package setupworkspace

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

// CmdAsUser is a no-op on windows: setupworkspace targets the Linux workspace
// VM, so a windows build only needs to compile. Credential-based process
// impersonation is not portable; if windows execution as another user is ever
// needed, use CreateProcessWithLogonW via golang.org/x/sys/windows.
func CmdAsUser(cmd *exec.Cmd, user *user.User) error {
	cmd.Env = append(os.Environ(), "USER="+user.Username, "HOME="+user.HomeDir)
	return nil
}

func ChownFileToUser(*os.File, *user.User) error {
	return breverrors.New("chown is not supported on windows")
}

func ChownFilePathToUser(string, *user.User) error {
	return breverrors.New("chown is not supported on windows")
}

var _ = fmt.Sprintf // keep fmt import if unused after edits
