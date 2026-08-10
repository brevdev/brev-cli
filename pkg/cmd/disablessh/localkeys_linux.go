//go:build linux

package disablessh

import (
	"bytes"
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
	if len(accounts) == 0 {
		return nil, fmt.Errorf("getent passwd returned no local accounts")
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

	if err := replaceAuthorizedKeys(sshFD, authorizedKeysFD, data, cleaned, opened); err != nil {
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

type replaceAuthorizedKeysHooks struct {
	beforeExchange func(sshFD int, tempName string) error
}

func replaceAuthorizedKeys(
	sshFD int,
	originalFD int,
	originalData []byte,
	cleaned []byte,
	original unix.Stat_t,
) error {
	return replaceAuthorizedKeysWithHooks(
		sshFD,
		originalFD,
		originalData,
		cleaned,
		original,
		replaceAuthorizedKeysHooks{},
	)
}

func replaceAuthorizedKeysWithHooks(
	sshFD int,
	originalFD int,
	originalData []byte,
	cleaned []byte,
	original unix.Stat_t,
	hooks replaceAuthorizedKeysHooks,
) (retErr error) {
	tempFD, tempName, err := createRandomTempFile(sshFD)
	if err != nil {
		return err
	}
	var tempCleanupIdentity unix.Stat_t
	tempIdentityKnown := false
	if err := unix.Fstat(tempFD, &tempCleanupIdentity); err != nil {
		closeDescriptor(tempFD)
		return fmt.Errorf("inspect created temporary authorized_keys: %w", err)
	}
	tempIdentityKnown = true
	var tempStat unix.Stat_t
	defer func() {
		if tempIdentityKnown {
			if _, cleanupErr := unlinkNameIfMatches(sshFD, tempName, tempCleanupIdentity); cleanupErr != nil {
				retErr = errors.Join(retErr, fmt.Errorf("remove temporary authorized_keys: %w", cleanupErr))
			}
		}
		closeDescriptor(tempFD)
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
	if err := unix.Fstat(tempFD, &tempStat); err != nil {
		return fmt.Errorf("inspect temporary authorized_keys: %w", err)
	}
	if !isRegular(tempStat) {
		return fmt.Errorf("temporary authorized_keys is not a regular file")
	}

	if err := verifyDescriptorState(originalFD, original, originalData); err != nil {
		return fmt.Errorf("authorized_keys changed before commit: %w", err)
	}
	if err := verifyDescriptorState(tempFD, tempStat, cleaned); err != nil {
		return fmt.Errorf("temporary authorized_keys changed before commit: %w", err)
	}
	if err := verifyNameMatches(sshFD, authorizedKeysName, original); err != nil {
		return fmt.Errorf("authorized_keys changed before commit: %w", err)
	}
	if err := verifyNameMatches(sshFD, tempName, tempStat); err != nil {
		return fmt.Errorf("temporary authorized_keys changed before commit: %w", err)
	}
	if hooks.beforeExchange != nil {
		if err := hooks.beforeExchange(sshFD, tempName); err != nil {
			return fmt.Errorf("run authorized_keys commit hook: %w", err)
		}
	}

	if err := unix.Renameat2(
		sshFD,
		tempName,
		sshFD,
		authorizedKeysName,
		unix.RENAME_EXCHANGE,
	); err != nil {
		return fmt.Errorf("exchange temporary authorized_keys: %w", err)
	}

	postAuthorized, postTemp, verificationErr := verifyExchangedAuthorizedKeys(
		sshFD,
		originalFD,
		tempFD,
		tempName,
		original,
		tempStat,
		originalData,
		cleaned,
	)
	if verificationErr != nil {
		rollbackErr := rollbackAuthorizedKeysExchange(sshFD, tempName, postAuthorized, postTemp)
		if rollbackErr != nil {
			return errors.Join(
				fmt.Errorf("authorized_keys changed during commit: %w", verificationErr),
				fmt.Errorf("restore authorized_keys exchange: %w", rollbackErr),
			)
		}
		return fmt.Errorf("authorized_keys changed during commit: %w", verificationErr)
	}

	unlinked, err := unlinkNameIfMatches(sshFD, tempName, original)
	if err != nil {
		return fmt.Errorf("remove exchanged original authorized_keys: %w", err)
	}
	if !unlinked {
		return fmt.Errorf("authorized_keys changed during commit before removing exchanged original")
	}
	if err := unix.Fsync(sshFD); err != nil {
		return fmt.Errorf("sync .ssh directory: %w", err)
	}
	return nil
}

func verifyExchangedAuthorizedKeys(
	sshFD int,
	originalFD int,
	tempFD int,
	tempName string,
	original unix.Stat_t,
	temp unix.Stat_t,
	originalData []byte,
	cleaned []byte,
) (unix.Stat_t, unix.Stat_t, error) {
	postAuthorized, authorizedErr := statName(sshFD, authorizedKeysName)
	postTemp, tempErr := statName(sshFD, tempName)
	var verificationErrs []error
	if authorizedErr != nil {
		verificationErrs = append(verificationErrs, fmt.Errorf("inspect exchanged authorized_keys: %w", authorizedErr))
	} else if !sameFileIdentity(postAuthorized, temp) {
		verificationErrs = append(verificationErrs, fmt.Errorf("exchanged authorized_keys does not match verified temporary file"))
	}
	if tempErr != nil {
		verificationErrs = append(verificationErrs, fmt.Errorf("inspect exchanged original authorized_keys: %w", tempErr))
	} else if !sameFileIdentity(postTemp, original) {
		verificationErrs = append(verificationErrs, fmt.Errorf("exchanged original authorized_keys does not match opened file"))
	}
	if err := verifyDescriptorState(originalFD, original, originalData); err != nil {
		verificationErrs = append(verificationErrs, fmt.Errorf("opened original authorized_keys changed: %w", err))
	}
	if err := verifyDescriptorState(tempFD, temp, cleaned); err != nil {
		verificationErrs = append(verificationErrs, fmt.Errorf("opened temporary authorized_keys changed: %w", err))
	}
	return postAuthorized, postTemp, errors.Join(verificationErrs...)
}

func rollbackAuthorizedKeysExchange(
	sshFD int,
	tempName string,
	postAuthorized unix.Stat_t,
	postTemp unix.Stat_t,
) error {
	currentAuthorized, authorizedErr := statName(sshFD, authorizedKeysName)
	currentTemp, tempErr := statName(sshFD, tempName)
	if authorizedErr != nil || tempErr != nil {
		var inspectErrs []error
		if authorizedErr != nil {
			inspectErrs = append(inspectErrs, fmt.Errorf("inspect current authorized_keys before rollback: %w", authorizedErr))
		}
		if tempErr != nil {
			inspectErrs = append(inspectErrs, fmt.Errorf("inspect current temporary name before rollback: %w", tempErr))
		}
		return errors.Join(inspectErrs...)
	}
	if !sameFileIdentity(currentAuthorized, postAuthorized) || !sameFileIdentity(currentTemp, postTemp) {
		return fmt.Errorf("directory entries changed again before rollback")
	}
	if err := unix.Renameat2(
		sshFD,
		tempName,
		sshFD,
		authorizedKeysName,
		unix.RENAME_EXCHANGE,
	); err != nil {
		return fmt.Errorf("exchange directory entries back: %w", err)
	}
	if err := verifyNameMatches(sshFD, authorizedKeysName, postTemp); err != nil {
		return fmt.Errorf("verify restored authorized_keys: %w", err)
	}
	if err := verifyNameMatches(sshFD, tempName, postAuthorized); err != nil {
		return fmt.Errorf("verify restored temporary name: %w", err)
	}
	if err := unix.Fsync(sshFD); err != nil {
		return fmt.Errorf("sync restored .ssh directory: %w", err)
	}
	return nil
}

func verifyDescriptorState(fd int, expected unix.Stat_t, expectedData []byte) error {
	if err := verifyDescriptorMetadata(fd, expected); err != nil {
		return err
	}
	data, err := readAllAt(fd)
	if err != nil {
		return fmt.Errorf("read opened file: %w", err)
	}
	if !bytes.Equal(data, expectedData) {
		return fmt.Errorf("opened file contents changed")
	}
	return nil
}

func verifyDescriptorMetadata(fd int, expected unix.Stat_t) error {
	var current unix.Stat_t
	if err := unix.Fstat(fd, &current); err != nil {
		return fmt.Errorf("inspect opened file: %w", err)
	}
	if !isRegular(current) || !sameFileIdentity(current, expected) {
		return fmt.Errorf("opened file identity changed")
	}
	if current.Uid != expected.Uid || current.Gid != expected.Gid || current.Mode&0o7777 != expected.Mode&0o7777 {
		return fmt.Errorf("opened file ownership or mode changed")
	}
	return nil
}

func verifyNameMatches(dirFD int, name string, expected unix.Stat_t) error {
	current, err := statName(dirFD, name)
	if err != nil {
		return err
	}
	if !isRegular(current) || !sameFileIdentity(current, expected) {
		return fmt.Errorf("%q no longer identifies the verified regular file", name)
	}
	return nil
}

func statName(dirFD int, name string) (unix.Stat_t, error) {
	var stat unix.Stat_t
	if err := unix.Fstatat(dirFD, name, &stat, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return unix.Stat_t{}, err
	}
	return stat, nil
}

func unlinkNameIfMatches(dirFD int, name string, expected unix.Stat_t) (bool, error) {
	current, err := statName(dirFD, name)
	if errors.Is(err, unix.ENOENT) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !sameFileIdentity(current, expected) {
		return false, nil
	}
	if err := unix.Unlinkat(dirFD, name, 0); err != nil {
		return false, err
	}
	return true, nil
}

func readAllAt(fd int) ([]byte, error) {
	const chunkSize = 32 * 1024
	data := make([]byte, 0, chunkSize)
	buffer := make([]byte, chunkSize)
	for {
		n, err := unix.Pread(fd, buffer, int64(len(data)))
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return data, nil
		}
		data = append(data, buffer[:n]...)
	}
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
			unix.O_CREAT|unix.O_EXCL|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW,
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
