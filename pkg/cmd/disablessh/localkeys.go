package disablessh

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

const cleanupHelperArg = "__brev-disable-ssh-cleanup"

type KeyCleanupResult struct {
	AccountsScanned int `json:"accounts_scanned"`
	AccountsChanged int `json:"accounts_changed"`
	KeysRemoved     int `json:"keys_removed"`
}

type localAccount struct {
	Username string
	HomeDir  string
}

type localKeyCleaner interface {
	RemoveBrevKeys(context.Context) (KeyCleanupResult, error)
}

func parsePasswd(data []byte) ([]localAccount, error) {
	lines := bytes.Split(data, []byte("\n"))
	accounts := make([]localAccount, 0, len(lines))
	seenHomes := make(map[string]struct{}, len(lines))
	for i, line := range lines {
		line = bytes.TrimSuffix(line, []byte("\r"))
		if len(line) == 0 {
			continue
		}
		fields := bytes.Split(line, []byte(":"))
		if len(fields) != 7 {
			return nil, fmt.Errorf("parse passwd line %d: expected seven fields, got %d", i+1, len(fields))
		}
		home := string(fields[5])
		if !path.IsAbs(home) {
			return nil, fmt.Errorf("parse passwd line %d: home directory %q is not absolute", i+1, home)
		}
		if _, ok := seenHomes[home]; ok {
			continue
		}
		seenHomes[home] = struct{}{}
		accounts = append(accounts, localAccount{Username: string(fields[0]), HomeDir: home})
	}
	return accounts, nil
}

func stripBrevManagedAuthorizedKeyLines(data []byte) ([]byte, int) {
	segments := bytes.SplitAfter(data, []byte("\n"))
	cleaned := make([]byte, 0, len(data))
	removed := 0
	for _, segment := range segments {
		line := bytes.TrimSuffix(segment, []byte("\n"))
		line = bytes.TrimSuffix(line, []byte("\r"))
		if register.IsBrevManagedAuthorizedKeysLine(string(line)) {
			removed++
			continue
		}
		cleaned = append(cleaned, segment...)
	}
	return cleaned, removed
}

type systemLocalKeyCleaner struct {
	listAccounts func(context.Context) ([]localAccount, error)
	cleanAccount func(localAccount) (int, error)
}

func (c systemLocalKeyCleaner) RemoveBrevKeys(ctx context.Context) (KeyCleanupResult, error) {
	accounts, err := c.listAccounts(ctx)
	if err != nil {
		return KeyCleanupResult{}, fmt.Errorf("enumerate local accounts: %w", err)
	}

	var result KeyCleanupResult
	var accountErrs []error
	for _, account := range accounts {
		removed, err := c.cleanAccount(account)
		if err != nil {
			accountErrs = append(accountErrs, fmt.Errorf("clean Brev keys for account %q at %q: %w", account.Username, account.HomeDir, err))
			continue
		}
		result.AccountsScanned++
		if removed > 0 {
			result.AccountsChanged++
			result.KeysRemoved += removed
		}
	}
	if err := breverrors.Join(accountErrs...); err != nil {
		return result, fmt.Errorf("clean one or more local accounts: %w", err)
	}
	return result, nil
}

func newSystemLocalKeyCleaner() localKeyCleaner {
	return systemLocalKeyCleaner{
		listAccounts: listLocalAccounts,
		cleanAccount: cleanLocalAccount,
	}
}

type privilegedCommandRunner interface {
	Output(context.Context, string, ...string) ([]byte, error)
}

type execPrivilegedCommandRunner struct{}

func (execPrivilegedCommandRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	output, err := exec.CommandContext(ctx, name, args...).Output()
	if err == nil {
		return output, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
		return nil, fmt.Errorf("%w (stderr: %s)", err, strings.TrimSpace(string(exitErr.Stderr)))
	}
	return nil, fmt.Errorf("execute privileged command: %w", err)
}

type privilegedLocalKeyCleaner struct {
	geteuid    func() int
	executable func() (string, error)
	runner     privilegedCommandRunner
	direct     localKeyCleaner
}

func (c privilegedLocalKeyCleaner) RemoveBrevKeys(ctx context.Context) (KeyCleanupResult, error) {
	if c.geteuid() == 0 {
		result, err := c.direct.RemoveBrevKeys(ctx)
		if err != nil {
			return result, fmt.Errorf("run direct Brev key cleanup: %w", err)
		}
		return result, nil
	}
	executable, err := c.executable()
	if err != nil {
		return KeyCleanupResult{}, fmt.Errorf("locate Brev executable: %w", err)
	}
	output, err := c.runner.Output(ctx, "sudo", "-n", executable, cleanupHelperArg)
	if err != nil {
		return KeyCleanupResult{}, fmt.Errorf("run privileged Brev key cleanup: %w", err)
	}
	var result KeyCleanupResult
	if err := json.Unmarshal(output, &result); err != nil {
		return KeyCleanupResult{}, fmt.Errorf("decode privileged Brev key cleanup result: %w", err)
	}
	return result, nil
}

func newPrivilegedLocalKeyCleaner() localKeyCleaner { //nolint:unused // Used by the Task 5 disable-ssh command.
	return privilegedLocalKeyCleaner{
		geteuid:    os.Geteuid,
		executable: os.Executable,
		runner:     execPrivilegedCommandRunner{},
		direct:     newSystemLocalKeyCleaner(),
	}
}

// RunLocalKeyCleanupHelper runs the fixed privileged key-cleanup mode when
// selected by args. Normal CLI arguments are ignored.
func RunLocalKeyCleanupHelper(ctx context.Context, args []string, stdout io.Writer) (bool, error) {
	return runLocalKeyCleanupHelper(ctx, args, stdout, runtime.GOOS, os.Geteuid, newSystemLocalKeyCleaner())
}

func runLocalKeyCleanupHelper(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	goos string,
	geteuid func() int,
	cleaner localKeyCleaner,
) (bool, error) {
	if len(args) == 0 || args[0] != cleanupHelperArg {
		return false, nil
	}
	if len(args) != 1 {
		return true, fmt.Errorf("privileged Brev key cleanup requires exactly one fixed argument")
	}
	if goos != "linux" {
		return true, fmt.Errorf("brev disable-ssh local cleanup is only supported on Linux")
	}
	if geteuid() != 0 {
		return true, fmt.Errorf("privileged Brev key cleanup must run as root")
	}
	result, err := cleaner.RemoveBrevKeys(ctx)
	if err != nil {
		return true, fmt.Errorf("run local Brev key cleanup: %w", err)
	}
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		return true, fmt.Errorf("encode privileged Brev key cleanup result: %w", err)
	}
	return true, nil
}
