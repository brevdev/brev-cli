//go:build linux

package disablessh

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

type fakeGetentRunner struct {
	output []byte
	err    error
	name   string
	args   []string
}

func (f *fakeGetentRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	f.name = name
	f.args = append([]string(nil), args...)
	return append([]byte(nil), f.output...), f.err
}

func TestResolveGetent_UsesOnlyFixedCandidates(t *testing.T) {
	var checked []string
	got, err := resolveGetent(func(candidate string) bool {
		checked = append(checked, candidate)
		return candidate == "/bin/getent"
	})
	if err != nil {
		t.Fatalf("resolveGetent: %v", err)
	}
	if got != "/bin/getent" {
		t.Fatalf("resolveGetent = %q, want /bin/getent", got)
	}
	wantChecked := []string{"/usr/bin/getent", "/bin/getent"}
	if !reflect.DeepEqual(checked, wantChecked) {
		t.Fatalf("checked = %#v, want fixed candidates %#v", checked, wantChecked)
	}
}

func TestListLocalAccountsWith_RunsFixedGetentPasswdAndParsesOutput(t *testing.T) {
	data, err := os.ReadFile("testdata/passwd.txt")
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeGetentRunner{output: data}

	accounts, err := listLocalAccountsWith(context.Background(), "/usr/bin/getent", runner)
	if err != nil {
		t.Fatalf("listLocalAccountsWith: %v", err)
	}
	if runner.name != "/usr/bin/getent" || !reflect.DeepEqual(runner.args, []string{"passwd"}) {
		t.Fatalf("getent call = %q %#v, want /usr/bin/getent [passwd]", runner.name, runner.args)
	}
	if len(accounts) != 4 {
		t.Fatalf("accounts = %d, want 4 deduplicated homes", len(accounts))
	}
}

func TestListLocalAccountsWith_PropagatesGetentFailure(t *testing.T) {
	runner := &fakeGetentRunner{err: errors.New("exit 2")}
	_, err := listLocalAccountsWith(context.Background(), "/usr/bin/getent", runner)
	if err == nil || !strings.Contains(err.Error(), "getent passwd") || !strings.Contains(err.Error(), "exit 2") {
		t.Fatalf("listLocalAccountsWith() error = %v, want getent failure context", err)
	}
}

func TestListLocalAccountsWith_RejectsEmptyEnumeration(t *testing.T) {
	for _, output := range [][]byte{nil, []byte("\n\r\n")} {
		runner := &fakeGetentRunner{output: output}
		_, err := listLocalAccountsWith(context.Background(), "/usr/bin/getent", runner)
		if err == nil || !strings.Contains(err.Error(), "returned no local accounts") {
			t.Fatalf("listLocalAccountsWith(%q) error = %v, want empty-enumeration failure", output, err)
		}
	}
}

func TestSystemAuthorizedKeysCleaner_RemovesBothMarkersAndPreservesModeAndOwnership(t *testing.T) {
	account, authKeysPath := prepareAuthorizedKeys(t, true)
	before, err := os.ReadFile("testdata/authorized_keys.before")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(authKeysPath, before, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(authKeysPath, 0o2640); err != nil {
		t.Fatal(err)
	}
	var beforeStat unix.Stat_t
	if err := unix.Stat(authKeysPath, &beforeStat); err != nil {
		t.Fatal(err)
	}
	if got := beforeStat.Mode & 0o7777; got != 0o2640 {
		t.Skipf("filesystem cannot establish setgid test precondition: mode = %#o, want %#o", got, uint32(0o2640))
	}

	removed, err := cleanLocalAccount(account)
	if err != nil {
		t.Fatalf("cleanLocalAccount: %v", err)
	}
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	want, err := os.ReadFile("testdata/authorized_keys.after")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(authKeysPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("authorized_keys = %q, want %q", got, want)
	}
	var afterStat unix.Stat_t
	if err := unix.Stat(authKeysPath, &afterStat); err != nil {
		t.Fatal(err)
	}
	if afterStat.Uid != beforeStat.Uid || afterStat.Gid != beforeStat.Gid {
		t.Fatalf("ownership = %d:%d, want %d:%d", afterStat.Uid, afterStat.Gid, beforeStat.Uid, beforeStat.Gid)
	}
	if gotMode, wantMode := afterStat.Mode&0o7777, beforeStat.Mode&0o7777; gotMode != wantMode {
		t.Fatalf("mode = %#o, want full mode %#o", gotMode, wantMode)
	}
}

func TestReplaceAuthorizedKeys_RejectsSubstitutedTempSource(t *testing.T) {
	_, authKeysPath := prepareAuthorizedKeys(t, true)
	original := []byte("ssh-ed25519 KEEP keep@example.com #brev-portID:old\n")
	if err := os.WriteFile(authKeysPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	sshFD, originalFD, originalStat := openReplacementTestDescriptors(t, authKeysPath)
	defer closeDescriptor(sshFD)
	defer closeDescriptor(originalFD)

	err := replaceAuthorizedKeysWithHooks(
		sshFD,
		originalFD,
		original,
		[]byte("ssh-ed25519 KEEP keep@example.com\n"),
		originalStat,
		replaceAuthorizedKeysHooks{beforeExchange: func(sshFD int, tempName string) error {
			if err := unix.Unlinkat(sshFD, tempName, 0); err != nil {
				return err
			}
			attackerFD, err := unix.Openat(
				sshFD,
				tempName,
				unix.O_CREAT|unix.O_EXCL|unix.O_WRONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW,
				0o600,
			)
			if err != nil {
				return err
			}
			defer closeDescriptor(attackerFD)
			return writeAll(attackerFD, []byte("attacker-controlled source\n"))
		}},
	)
	if err == nil || !strings.Contains(err.Error(), "changed during commit") {
		t.Fatalf("replaceAuthorizedKeysWithHooks() error = %v, want source-substitution failure", err)
	}
	got, readErr := os.ReadFile(authKeysPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("authorized_keys = %q, want original destination preserved %q", got, original)
	}
}

func TestReplaceAuthorizedKeys_RejectsSubstitutedDestinationWithoutDestroyingIt(t *testing.T) {
	_, authKeysPath := prepareAuthorizedKeys(t, true)
	original := []byte("ssh-ed25519 OLD old@example.com #brev-portID:old\n")
	if err := os.WriteFile(authKeysPath, original, 0o600); err != nil {
		t.Fatal(err)
	}
	sshFD, originalFD, originalStat := openReplacementTestDescriptors(t, authKeysPath)
	defer closeDescriptor(sshFD)
	defer closeDescriptor(originalFD)
	replacement := []byte("ssh-ed25519 NEW concurrent@example.com\n")

	err := replaceAuthorizedKeysWithHooks(
		sshFD,
		originalFD,
		original,
		[]byte("ssh-ed25519 OLD old@example.com\n"),
		originalStat,
		replaceAuthorizedKeysHooks{beforeExchange: func(sshFD int, _ string) error {
			const replacementName = "authorized_keys.concurrent-replacement"
			replacementFD, err := unix.Openat(
				sshFD,
				replacementName,
				unix.O_CREAT|unix.O_EXCL|unix.O_WRONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW,
				0o600,
			)
			if err != nil {
				return err
			}
			if err := writeAll(replacementFD, replacement); err != nil {
				closeDescriptor(replacementFD)
				return err
			}
			if err := unix.Close(replacementFD); err != nil {
				return err
			}
			return unix.Renameat(sshFD, replacementName, sshFD, authorizedKeysName)
		}},
	)
	if err == nil || !strings.Contains(err.Error(), "changed during commit") {
		t.Fatalf("replaceAuthorizedKeysWithHooks() error = %v, want destination-substitution failure", err)
	}
	got, readErr := os.ReadFile(authKeysPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, replacement) {
		t.Fatalf("authorized_keys = %q, want concurrent replacement preserved %q", got, replacement)
	}
}

func TestSystemAuthorizedKeysCleaner_NoMarkersDoesNotRewrite(t *testing.T) {
	account, authKeysPath := prepareAuthorizedKeys(t, true)
	if err := os.WriteFile(authKeysPath, []byte("ssh-ed25519 AAAA_KEEP keep@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var before unix.Stat_t
	if err := unix.Stat(authKeysPath, &before); err != nil {
		t.Fatal(err)
	}

	removed, err := cleanLocalAccount(account)
	if err != nil {
		t.Fatalf("cleanLocalAccount: %v", err)
	}
	if removed != 0 {
		t.Fatalf("removed = %d, want 0", removed)
	}
	var after unix.Stat_t
	if err := unix.Stat(authKeysPath, &after); err != nil {
		t.Fatal(err)
	}
	if before.Dev != after.Dev || before.Ino != after.Ino {
		t.Fatalf("inode changed from %d:%d to %d:%d without markers", before.Dev, before.Ino, after.Dev, after.Ino)
	}
}

func TestSystemAuthorizedKeysCleaner_MissingSSHDirectoryIsSuccess(t *testing.T) {
	account, _ := prepareAuthorizedKeys(t, false)
	removed, err := cleanLocalAccount(account)
	if err != nil || removed != 0 {
		t.Fatalf("removed, err = %d, %v; want 0, nil", removed, err)
	}
}

func TestSystemAuthorizedKeysCleaner_MissingHomeComponentIsSuccess(t *testing.T) {
	account := localAccount{Username: "alice", HomeDir: filepath.Join(t.TempDir(), "missing", "alice")}
	removed, err := cleanLocalAccount(account)
	if err != nil || removed != 0 {
		t.Fatalf("removed, err = %d, %v; want 0, nil", removed, err)
	}
}

func TestSystemAuthorizedKeysCleaner_MissingAuthorizedKeysIsSuccess(t *testing.T) {
	account, authKeysPath := prepareAuthorizedKeys(t, true)
	if _, err := os.Stat(authKeysPath); !os.IsNotExist(err) {
		t.Fatalf("authorized_keys unexpectedly exists: %v", err)
	}
	removed, err := cleanLocalAccount(account)
	if err != nil || removed != 0 {
		t.Fatalf("removed, err = %d, %v; want 0, nil", removed, err)
	}
}

func TestSystemAuthorizedKeysCleaner_RejectsSSHDirectorySymlink(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(home, ".ssh")); err != nil {
		t.Fatal(err)
	}
	assertUnsafeAccountPath(t, localAccount{Username: "alice", HomeDir: home})
}

func TestSystemAuthorizedKeysCleaner_RejectsAuthorizedKeysSymlink(t *testing.T) {
	account, authKeysPath := prepareAuthorizedKeys(t, true)
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, []byte("ssh-rsa KEEP\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, authKeysPath); err != nil {
		t.Fatal(err)
	}
	assertUnsafeAccountPath(t, account)
}

func TestSystemAuthorizedKeysCleaner_RejectsIntermediateHomeSymlink(t *testing.T) {
	root := t.TempDir()
	realParent := filepath.Join(root, "real")
	home := filepath.Join(realParent, "alice")
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	linkParent := filepath.Join(root, "linked")
	if err := os.Symlink(realParent, linkParent); err != nil {
		t.Fatal(err)
	}
	assertUnsafeAccountPath(t, localAccount{Username: "alice", HomeDir: filepath.Join(linkParent, "alice")})
}

func TestSystemAuthorizedKeysCleaner_RejectsFinalHomeSymlink(t *testing.T) {
	root := t.TempDir()
	realHome := filepath.Join(root, "real-home")
	if err := os.MkdirAll(filepath.Join(realHome, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	linkedHome := filepath.Join(root, "linked-home")
	if err := os.Symlink(realHome, linkedHome); err != nil {
		t.Fatal(err)
	}
	assertUnsafeAccountPath(t, localAccount{Username: "alice", HomeDir: linkedHome})
}

func TestSystemAuthorizedKeysCleaner_RejectsParentTraversal(t *testing.T) {
	root := t.TempDir()
	account := localAccount{Username: "alice", HomeDir: root + "/missing/../alice"}
	assertUnsafeAccountPath(t, account)
}

func TestSystemAuthorizedKeysCleaner_RejectsFIFOWithoutBlocking(t *testing.T) {
	account, authKeysPath := prepareAuthorizedKeys(t, true)
	if err := unix.Mkfifo(authKeysPath, 0o600); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := cleanLocalAccount(account)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cleanLocalAccount() error = nil, want FIFO rejection")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cleanLocalAccount blocked while inspecting FIFO")
	}
}

func TestSystemAuthorizedKeysCleaner_RejectsNonRegularAuthorizedKeys(t *testing.T) {
	account, authKeysPath := prepareAuthorizedKeys(t, true)
	if err := os.Mkdir(authKeysPath, 0o700); err != nil {
		t.Fatal(err)
	}
	assertUnsafeAccountPath(t, account)
}

func prepareAuthorizedKeys(t *testing.T, createSSH bool) (localAccount, string) {
	t.Helper()
	home := filepath.Join(t.TempDir(), "home", "alice")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	sshDir := filepath.Join(home, ".ssh")
	if createSSH {
		if err := os.Mkdir(sshDir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	return localAccount{Username: "alice", HomeDir: home}, filepath.Join(sshDir, "authorized_keys")
}

func openReplacementTestDescriptors(t *testing.T, authorizedKeysPath string) (int, int, unix.Stat_t) {
	t.Helper()
	sshFD, err := unix.Open(filepath.Dir(authorizedKeysPath), directoryOpenFlags(), 0)
	if err != nil {
		t.Fatal(err)
	}
	originalFD, err := unix.Openat(
		sshFD,
		authorizedKeysName,
		unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK,
		0,
	)
	if err != nil {
		closeDescriptor(sshFD)
		t.Fatal(err)
	}
	var originalStat unix.Stat_t
	if err := unix.Fstat(originalFD, &originalStat); err != nil {
		closeDescriptor(originalFD)
		closeDescriptor(sshFD)
		t.Fatal(err)
	}
	return sshFD, originalFD, originalStat
}

func assertUnsafeAccountPath(t *testing.T, account localAccount) {
	t.Helper()
	if removed, err := cleanLocalAccount(account); err == nil {
		t.Fatalf("removed, err = %d, nil; want unsafe-path rejection", removed)
	}
}
