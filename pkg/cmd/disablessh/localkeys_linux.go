//go:build linux

package disablessh

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	authorizedKeysName = "authorized_keys"
	tempFilePrefix     = "authorized_keys.brev-cleanup-"
)

type getentCommandRunner interface {
	Output(context.Context, string, ...string) ([]byte, error)
}

type execGetentCommandRunner struct{}

func (execGetentCommandRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	output, err := exec.CommandContext(ctx, name, args...).Output()
	if err == nil {
		return output, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
		return nil, fmt.Errorf("%w (stderr: %s)", err, strings.TrimSpace(string(exitErr.Stderr)))
	}
	return nil, err
}

func resolveGetent(exists func(string) bool) (string, error) {
	for _, candidate := range [...]string{"/usr/bin/getent", "/bin/getent"} {
		if exists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("getent not found at /usr/bin/getent or /bin/getent")
}

func listLocalAccounts(ctx context.Context) ([]localAccount, error) {
	getentPath, err := resolveGetent(func(candidate string) bool {
		info, err := os.Stat(candidate)
		return err == nil && info.Mode().IsRegular()
	})
	if err != nil {
		return nil, err
	}
	return listLocalAccountsWith(ctx, getentPath, execGetentCommandRunner{})
}

func listLocalAccountsWith(ctx context.Context, getentPath string, runner getentCommandRunner) ([]localAccount, error) {
	output, err := runner.Output(ctx, getentPath, "passwd")
	if err != nil {
		return nil, fmt.Errorf("run getent passwd: %w", err)
	}
	accounts, err := parsePasswd(output)
	if err != nil {
		return nil, fmt.Errorf("parse getent passwd output: %w", err)
	}
	return accounts, nil
}

func cleanLocalAccount(account localAccount) (int, error) {
	homeFD, err := openAbsoluteDirectory(account.HomeDir)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return 0, nil
		}
		return 0, fmt.Errorf("open home directory %q: %w", account.HomeDir, err)
	}
	defer closeDescriptor(homeFD)

	sshFD, err := unix.Openat(homeFD, ".ssh", directoryOpenFlags(), 0)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return 0, nil
		}
		return 0, fmt.Errorf("open .ssh under home %q: %w", account.HomeDir, err)
	}
	defer closeDescriptor(sshFD)

	var beforeOpen unix.Stat_t
	if err := unix.Fstatat(sshFD, authorizedKeysName, &beforeOpen, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		if errors.Is(err, unix.ENOENT) {
			return 0, nil
		}
		return 0, fmt.Errorf("inspect authorized_keys under home %q: %w", account.HomeDir, err)
	}
	if !isRegular(beforeOpen) {
		return 0, fmt.Errorf("authorized_keys under home %q is not a regular file", account.HomeDir)
	}

	authorizedKeysFD, err := unix.Openat(
		sshFD,
		authorizedKeysName,
		unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK,
		0,
	)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return 0, nil
		}
		return 0, fmt.Errorf("open authorized_keys under home %q: %w", account.HomeDir, err)
	}
	authorizedKeysFile := os.NewFile(uintptr(authorizedKeysFD), authorizedKeysName)
	if authorizedKeysFile == nil {
		closeDescriptor(authorizedKeysFD)
		return 0, fmt.Errorf("open authorized_keys under home %q: invalid file descriptor", account.HomeDir)
	}
	defer func() { _ = authorizedKeysFile.Close() }()

	var opened unix.Stat_t
	if err := unix.Fstat(authorizedKeysFD, &opened); err != nil {
		return 0, fmt.Errorf("inspect opened authorized_keys under home %q: %w", account.HomeDir, err)
	}
	if !isRegular(opened) {
		return 0, fmt.Errorf("opened authorized_keys under home %q is not a regular file", account.HomeDir)
	}
	if !sameFileIdentity(beforeOpen, opened) {
		return 0, fmt.Errorf("authorized_keys under home %q changed while opening", account.HomeDir)
	}

	data, err := io.ReadAll(authorizedKeysFile)
	if err != nil {
		return 0, fmt.Errorf("read authorized_keys under home %q: %w", account.HomeDir, err)
	}
	cleaned, removed := stripBrevManagedAuthorizedKeyLines(data)
	if removed == 0 {
		return 0, nil
	}

	if err := replaceAuthorizedKeys(sshFD, cleaned, opened); err != nil {
		return 0, fmt.Errorf("replace authorized_keys under home %q: %w", account.HomeDir, err)
	}
	return removed, nil
}

func directoryOpenFlags() int {
	return unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW
}

func openAbsoluteDirectory(home string) (int, error) {
	if !path.IsAbs(home) {
		return -1, fmt.Errorf("path %q is not absolute", home)
	}
	for _, component := range strings.Split(home, "/") {
		if component == ".." {
			return -1, fmt.Errorf("path %q contains parent traversal", home)
		}
	}

	currentFD, err := unix.Open("/", directoryOpenFlags(), 0)
	if err != nil {
		return -1, fmt.Errorf("open root directory: %w", err)
	}
	cleaned := path.Clean(home)
	for _, component := range strings.Split(strings.TrimPrefix(cleaned, "/"), "/") {
		if component == "" || component == "." {
			continue
		}
		nextFD, err := unix.Openat(currentFD, component, directoryOpenFlags(), 0)
		if err != nil {
			closeDescriptor(currentFD)
			return -1, fmt.Errorf("open path component %q: %w", component, err)
		}
		if err := unix.Close(currentFD); err != nil {
			closeDescriptor(nextFD)
			return -1, fmt.Errorf("close parent directory before %q: %w", component, err)
		}
		currentFD = nextFD
	}
	return currentFD, nil
}

func isRegular(stat unix.Stat_t) bool {
	return stat.Mode&unix.S_IFMT == unix.S_IFREG
}

func sameFileIdentity(a, b unix.Stat_t) bool {
	return a.Dev == b.Dev && a.Ino == b.Ino && a.Mode&unix.S_IFMT == b.Mode&unix.S_IFMT
}

func replaceAuthorizedKeys(sshFD int, cleaned []byte, original unix.Stat_t) error {
	tempFD, tempName, err := createRandomTempFile(sshFD)
	if err != nil {
		return err
	}
	renamed := false
	defer func() {
		if tempFD >= 0 {
			closeDescriptor(tempFD)
		}
		if !renamed {
			_ = unix.Unlinkat(sshFD, tempName, 0)
		}
	}()

	if err := writeAll(tempFD, cleaned); err != nil {
		return fmt.Errorf("write temporary authorized_keys: %w", err)
	}
	if err := unix.Fchown(tempFD, int(original.Uid), int(original.Gid)); err != nil {
		return fmt.Errorf("restore temporary authorized_keys ownership: %w", err)
	}
	if err := unix.Fchmod(tempFD, original.Mode&0o7777); err != nil {
		return fmt.Errorf("restore temporary authorized_keys mode: %w", err)
	}
	if err := unix.Fsync(tempFD); err != nil {
		return fmt.Errorf("sync temporary authorized_keys: %w", err)
	}
	if err := unix.Close(tempFD); err != nil {
		tempFD = -1
		return fmt.Errorf("close temporary authorized_keys: %w", err)
	}
	tempFD = -1
	if err := unix.Renameat(sshFD, tempName, sshFD, authorizedKeysName); err != nil {
		return fmt.Errorf("rename temporary authorized_keys: %w", err)
	}
	renamed = true
	if err := unix.Fsync(sshFD); err != nil {
		return fmt.Errorf("sync .ssh directory: %w", err)
	}
	return nil
}

func createRandomTempFile(sshFD int) (int, string, error) {
	for range 128 {
		random := make([]byte, 16)
		if _, err := rand.Read(random); err != nil {
			return -1, "", fmt.Errorf("generate temporary authorized_keys name: %w", err)
		}
		name := tempFilePrefix + hex.EncodeToString(random)
		fd, err := unix.Openat(
			sshFD,
			name,
			unix.O_CREAT|unix.O_EXCL|unix.O_WRONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW,
			0o600,
		)
		if err == nil {
			return fd, name, nil
		}
		if !errors.Is(err, unix.EEXIST) {
			return -1, "", fmt.Errorf("create temporary authorized_keys: %w", err)
		}
	}
	return -1, "", fmt.Errorf("create temporary authorized_keys: exhausted random names")
}

func writeAll(fd int, data []byte) error {
	for len(data) > 0 {
		n, err := unix.Write(fd, data)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func closeDescriptor(fd int) {
	if fd >= 0 {
		_ = unix.Close(fd)
	}
}
