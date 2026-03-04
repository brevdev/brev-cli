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

	if _, err := register.InstallAuthorizedKey(u, "ssh-rsa AAAA testkey", ""); err != nil {
		t.Fatalf("InstallAuthorizedKey: %v", err)
	}

	content := readAuthorizedKeys(t, u)
	if !strings.Contains(content, "ssh-rsa AAAA testkey "+register.BrevKeyPrefix) {
		t.Errorf("expected key tagged with %q, got:\n%s", register.BrevKeyPrefix, content)
	}
}

func Test_InstallAuthorizedKey_SkipsDuplicate(t *testing.T) {
	u := tempUser(t)

	if _, err := register.InstallAuthorizedKey(u, "ssh-rsa AAAA testkey", ""); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if _, err := register.InstallAuthorizedKey(u, "ssh-rsa AAAA testkey", ""); err != nil {
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
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte("ssh-rsa AAAA testkey "+register.BrevKeyPrefix+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := register.InstallAuthorizedKey(u, "ssh-rsa AAAA testkey", ""); err != nil {
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

	if _, err := register.InstallAuthorizedKey(u, "", ""); err != nil {
		t.Fatalf("InstallAuthorizedKey: %v", err)
	}
	if _, err := register.InstallAuthorizedKey(u, "   ", ""); err != nil {
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

	if _, err := register.InstallAuthorizedKey(u, "ssh-rsa AAAA testkey", ""); err != nil {
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

	if _, err := register.InstallAuthorizedKey(u, "ssh-rsa AAAA testkey", ""); err != nil {
		t.Fatalf("InstallAuthorizedKey: %v", err)
	}

	content := readAuthorizedKeys(t, u)
	if !strings.Contains(content, "ssh-rsa EXISTING user@host") {
		t.Errorf("existing key was lost:\n%s", content)
	}
	if !strings.Contains(content, "ssh-rsa AAAA testkey "+register.BrevKeyPrefix) {
		t.Errorf("new key not found:\n%s", content)
	}
}

func Test_InstallAuthorizedKey_TagsExistingUntaggedKey(t *testing.T) {
	u := tempUser(t)

	// Pre-seed a key without the brev-cli tag.
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte("ssh-rsa AAAA testkey\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// InstallAuthorizedKey should tag the existing key rather than adding a duplicate.
	if _, err := register.InstallAuthorizedKey(u, "ssh-rsa AAAA testkey", ""); err != nil {
		t.Fatalf("InstallAuthorizedKey: %v", err)
	}

	content := readAuthorizedKeys(t, u)
	if !strings.Contains(content, "ssh-rsa AAAA testkey "+register.BrevKeyPrefix) {
		t.Errorf("expected existing key to be tagged with %q, got:\n%s", register.BrevKeyPrefix, content)
	}
	count := strings.Count(content, "ssh-rsa AAAA testkey")
	if count != 1 {
		t.Errorf("expected key to appear once, appeared %d times:\n%s", count, content)
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
		"ssh-rsa BREVKEY1 " + register.BrevKeyPrefix,
		"ssh-ed25519 OTHERKEY admin@server",
		"ssh-rsa BREVKEY2 " + register.BrevKeyPrefix,
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	removed, err := register.RemoveBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("RemoveBrevAuthorizedKeys: %v", err)
	}

	if len(removed) != 2 {
		t.Errorf("expected 2 removed keys, got %d: %v", len(removed), removed)
	}

	result := readAuthorizedKeys(t, u)
	if strings.Contains(result, register.BrevKeyPrefix) {
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

	removed, err := register.RemoveBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("expected no removed keys, got %v", removed)
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

	removed, err := register.RemoveBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("RemoveBrevAuthorizedKeys: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("expected no removed keys, got %v", removed)
	}

	result := readAuthorizedKeys(t, u)
	if result != original {
		t.Errorf("file was modified when it shouldn't have been.\nwant:\n%s\ngot:\n%s", original, result)
	}
}

// --- RemoveAuthorizedKey (specific key removal) ---

func Test_RemoveAuthorizedKey_RemovesOnlyTargetKey(t *testing.T) {
	u := tempUser(t)
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	content := strings.Join([]string{
		"ssh-rsa KEEP1 user@host",
		"ssh-rsa TARGET " + register.BrevKeyPrefix,
		"ssh-rsa KEEP2 admin@server",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := register.RemoveAuthorizedKey(u, "ssh-rsa TARGET"); err != nil {
		t.Fatalf("RemoveAuthorizedKey: %v", err)
	}

	result := readAuthorizedKeys(t, u)
	if strings.Contains(result, "TARGET") {
		t.Errorf("target key still present:\n%s", result)
	}
	if !strings.Contains(result, "ssh-rsa KEEP1 user@host") {
		t.Errorf("unrelated key was removed:\n%s", result)
	}
	if !strings.Contains(result, "ssh-rsa KEEP2 admin@server") {
		t.Errorf("unrelated key was removed:\n%s", result)
	}
}

func Test_RemoveAuthorizedKey_NoopWhenKeyNotPresent(t *testing.T) {
	u := tempUser(t)
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	original := "ssh-rsa EXISTING user@host\n"
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := register.RemoveAuthorizedKey(u, "ssh-rsa NOTHERE"); err != nil {
		t.Fatalf("RemoveAuthorizedKey: %v", err)
	}

	result := readAuthorizedKeys(t, u)
	if !strings.Contains(result, "ssh-rsa EXISTING user@host") {
		t.Errorf("existing key was removed:\n%s", result)
	}
}

func Test_RemoveAuthorizedKey_NoopCases(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"MissingFile", "ssh-rsa SOMEKEY"},
		{"EmptyKey", ""},
		{"WhitespaceKey", "   "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := tempUser(t)
			if err := register.RemoveAuthorizedKey(u, tt.key); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}

func Test_RemoveAuthorizedKey_DoesNotRemoveOtherBrevKeys(t *testing.T) {
	u := tempUser(t)
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	content := strings.Join([]string{
		"ssh-rsa ALICE_KEY " + register.BrevKeyPrefix,
		"ssh-rsa BOB_KEY " + register.BrevKeyPrefix,
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	// Remove only Alice's key — Bob's should stay.
	if err := register.RemoveAuthorizedKey(u, "ssh-rsa ALICE_KEY"); err != nil {
		t.Fatalf("RemoveAuthorizedKey: %v", err)
	}

	result := readAuthorizedKeys(t, u)
	if strings.Contains(result, "ALICE_KEY") {
		t.Errorf("Alice's key still present:\n%s", result)
	}
	if !strings.Contains(result, "ssh-rsa BOB_KEY") {
		t.Errorf("Bob's key was removed:\n%s", result)
	}
}

// --- Round-trip: install then remove (all brev keys) ---

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
	if _, err := register.InstallAuthorizedKey(u, "ssh-rsa KEY1", "user_1"); err != nil {
		t.Fatal(err)
	}
	if _, err := register.InstallAuthorizedKey(u, "ssh-rsa KEY2", "user_2"); err != nil {
		t.Fatal(err)
	}

	// Remove all brev keys.
	if _, err := register.RemoveBrevAuthorizedKeys(u); err != nil {
		t.Fatal(err)
	}

	result := readAuthorizedKeys(t, u)
	if strings.Contains(result, register.BrevKeyPrefix) {
		t.Errorf("brev keys still present after removal:\n%s", result)
	}
	if !strings.Contains(result, "ssh-rsa EXISTING user@host") {
		t.Errorf("non-brev key was removed:\n%s", result)
	}
}

// --- Round-trip: install then rollback specific key ---

func Test_InstallThenRemoveSpecificKey_RollbackScenario(t *testing.T) {
	u := tempUser(t)

	// Install two brev keys (simulating two users granted access).
	if _, err := register.InstallAuthorizedKey(u, "ssh-rsa ALICE", "user_a"); err != nil {
		t.Fatal(err)
	}
	if _, err := register.InstallAuthorizedKey(u, "ssh-rsa BOB", "user_b"); err != nil {
		t.Fatal(err)
	}

	// Simulate rollback: remove only Bob's key (e.g. his grant RPC failed).
	if err := register.RemoveAuthorizedKey(u, "ssh-rsa BOB"); err != nil {
		t.Fatal(err)
	}

	result := readAuthorizedKeys(t, u)
	if strings.Contains(result, "BOB") {
		t.Errorf("Bob's key still present after rollback:\n%s", result)
	}
	if !strings.Contains(result, "ssh-rsa ALICE") {
		t.Errorf("Alice's key was removed during Bob's rollback:\n%s", result)
	}
}
