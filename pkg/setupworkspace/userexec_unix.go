//go:build !windows

package setupworkspace

import (
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

func CmdAsUser(cmd *exec.Cmd, user *user.User) error {
	uid, err := strconv.ParseInt(user.Uid, 10, 32)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	gid, err := strconv.ParseInt(user.Gid, 10, 32)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	cmd.Env = append(os.Environ(), "USER="+user.Username, "HOME="+user.HomeDir, "SHELL=/bin/bash") // TODO get shell from user
	return nil
}

func ChownFileToUser(file *os.File, user *user.User) error {
	uid, err := strconv.ParseInt(user.Uid, 10, 32)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	gid, err := strconv.ParseInt(user.Gid, 10, 32)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = file.Chown(int(uid), int(gid))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func ChownFilePathToUser(filePath string, user *user.User) error {
	uid, err := strconv.ParseInt(user.Uid, 10, 32)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	gid, err := strconv.ParseInt(user.Gid, 10, 32)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = os.Chown(filePath, int(uid), int(gid))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
