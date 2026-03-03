package enablessh

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
)

// tempUser returns a *user.User whose HomeDir points to a temporary directory.
func tempUser(t *testing.T) *user.User {
	t.Helper()
	return &user.User{HomeDir: t.TempDir()}
}

// readAuthorizedKeys is a test helper that reads ~/.ssh/authorized_keys.
func readAuthorizedKeys(t *testing.T, u *user.User) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(u.HomeDir, ".ssh", "authorized_keys"))
	if err != nil {
		t.Fatalf("reading authorized_keys: %v", err)
	}
	return string(data)
}

// --- InstallAuthorizedKey ---

func Test_InstallAuthorizedKey_TagsKeyWithBrevComment(t *testing.T) {
	u := tempUser(t)

	if err := register.InstallAuthorizedKey(u, "ssh-rsa AAAA testkey"); err != nil {
		t.Fatalf("InstallAuthorizedKey: %v", err)
	}

	content := readAuthorizedKeys(t, u)
	if !strings.Contains(content, "ssh-rsa AAAA testkey "+register.BrevKeyComment) {
		t.Errorf("expected key tagged with %q, got:\n%s", register.BrevKeyComment, content)
	}
}

func Test_InstallAuthorizedKey_SkipsDuplicate(t *testing.T) {
	u := tempUser(t)

	if err := register.InstallAuthorizedKey(u, "ssh-rsa AAAA testkey"); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if err := register.InstallAuthorizedKey(u, "ssh-rsa AAAA testkey"); err != nil {
		t.Fatalf("second install: %v", err)
	}

	content := readAuthorizedKeys(t, u)
	count := strings.Count(content, "ssh-rsa AAAA testkey")
	if count != 1 {
		t.Errorf("expected key to appear once, appeared %d times:\n%s", count, content)
	}
}

func Test_InstallAuthorizedKey_SkipsDuplicateEvenIfAlreadyTagged(t *testing.T) {
	u := tempUser(t)

	// Pre-seed a tagged key (as if brev already installed it).
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte("ssh-rsa AAAA testkey "+register.BrevKeyComment+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := register.InstallAuthorizedKey(u, "ssh-rsa AAAA testkey"); err != nil {
		t.Fatalf("InstallAuthorizedKey: %v", err)
	}

	content := readAuthorizedKeys(t, u)
	count := strings.Count(content, "ssh-rsa AAAA testkey")
	if count != 1 {
		t.Errorf("expected key to appear once, appeared %d times:\n%s", count, content)
	}
}

func Test_InstallAuthorizedKey_EmptyKeyIsNoop(t *testing.T) {
	u := tempUser(t)

	if err := register.InstallAuthorizedKey(u, ""); err != nil {
		t.Fatalf("InstallAuthorizedKey: %v", err)
	}
	if err := register.InstallAuthorizedKey(u, "   "); err != nil {
		t.Fatalf("InstallAuthorizedKey (whitespace): %v", err)
	}

	// authorized_keys should not exist since nothing was written.
	path := filepath.Join(u.HomeDir, ".ssh", "authorized_keys")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected authorized_keys to not exist, but it does")
	}
}

func Test_InstallAuthorizedKey_CreatesSSHDir(t *testing.T) {
	u := tempUser(t)

	if err := register.InstallAuthorizedKey(u, "ssh-rsa AAAA testkey"); err != nil {
		t.Fatalf("InstallAuthorizedKey: %v", err)
	}

	info, err := os.Stat(filepath.Join(u.HomeDir, ".ssh"))
	if err != nil {
		t.Fatalf("stat .ssh: %v", err)
	}
	if !info.IsDir() {
		t.Error(".ssh is not a directory")
	}
}

func Test_InstallAuthorizedKey_PreservesExistingKeys(t *testing.T) {
	u := tempUser(t)

	// Pre-seed a non-brev key.
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte("ssh-rsa EXISTING user@host\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := register.InstallAuthorizedKey(u, "ssh-rsa AAAA testkey"); err != nil {
		t.Fatalf("InstallAuthorizedKey: %v", err)
	}

	content := readAuthorizedKeys(t, u)
	if !strings.Contains(content, "ssh-rsa EXISTING user@host") {
		t.Errorf("existing key was lost:\n%s", content)
	}
	if !strings.Contains(content, "ssh-rsa AAAA testkey "+register.BrevKeyComment) {
		t.Errorf("new key not found:\n%s", content)
	}
}

// --- RemoveBrevAuthorizedKeys ---

func Test_RemoveBrevAuthorizedKeys_RemovesTaggedKeys(t *testing.T) {
	u := tempUser(t)
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	content := strings.Join([]string{
		"ssh-rsa EXISTING user@host",
		"ssh-rsa BREVKEY1 " + register.BrevKeyComment,
		"ssh-ed25519 OTHERKEY admin@server",
		"ssh-rsa BREVKEY2 " + register.BrevKeyComment,
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := register.RemoveBrevAuthorizedKeys(u); err != nil {
		t.Fatalf("RemoveBrevAuthorizedKeys: %v", err)
	}

	result := readAuthorizedKeys(t, u)
	if strings.Contains(result, register.BrevKeyComment) {
		t.Errorf("brev keys still present:\n%s", result)
	}
	if !strings.Contains(result, "ssh-rsa EXISTING user@host") {
		t.Errorf("non-brev key was removed:\n%s", result)
	}
	if !strings.Contains(result, "ssh-ed25519 OTHERKEY admin@server") {
		t.Errorf("non-brev key was removed:\n%s", result)
	}
}

func Test_RemoveBrevAuthorizedKeys_NoopWhenFileDoesNotExist(t *testing.T) {
	u := tempUser(t)

	if err := register.RemoveBrevAuthorizedKeys(u); err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
}

func Test_RemoveBrevAuthorizedKeys_NoopWhenNoBrevKeys(t *testing.T) {
	u := tempUser(t)
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	original := "ssh-rsa EXISTING user@host\nssh-ed25519 OTHER admin@server\n"
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := register.RemoveBrevAuthorizedKeys(u); err != nil {
		t.Fatalf("RemoveBrevAuthorizedKeys: %v", err)
	}

	result := readAuthorizedKeys(t, u)
	if result != original {
		t.Errorf("file was modified when it shouldn't have been.\nwant:\n%s\ngot:\n%s", original, result)
	}
}

// --- Round-trip: install then remove ---

func Test_InstallThenRemove_RoundTrip(t *testing.T) {
	u := tempUser(t)

	// Pre-seed a non-brev key.
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte("ssh-rsa EXISTING user@host\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Install two brev keys.
	if err := register.InstallAuthorizedKey(u, "ssh-rsa KEY1"); err != nil {
		t.Fatal(err)
	}
	if err := register.InstallAuthorizedKey(u, "ssh-rsa KEY2"); err != nil {
		t.Fatal(err)
	}

	// Remove all brev keys.
	if err := register.RemoveBrevAuthorizedKeys(u); err != nil {
		t.Fatal(err)
	}

	result := readAuthorizedKeys(t, u)
	if strings.Contains(result, register.BrevKeyComment) {
		t.Errorf("brev keys still present after removal:\n%s", result)
	}
	if !strings.Contains(result, "ssh-rsa EXISTING user@host") {
		t.Errorf("non-brev key was removed:\n%s", result)
	}
}
