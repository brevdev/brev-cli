package register

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

func tempUser(t *testing.T) *user.User {
	t.Helper()
	return &user.User{HomeDir: t.TempDir()}
}

func seedKeys(t *testing.T, u *user.User, content string) {
	t.Helper()
	sshDir := filepath.Join(u.HomeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "authorized_keys"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readKeys(t *testing.T, u *user.User) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(u.HomeDir, ".ssh", "authorized_keys"))
	if err != nil {
		t.Fatalf("reading authorized_keys: %v", err)
	}
	return string(data)
}

// --- BrevKeyTag ---

func TestBrevKeyTag_WithUserID(t *testing.T) {
	tag := BrevKeyTag("user_abc123")
	expected := "# brev-cli user_id=user_abc123"
	if tag != expected {
		t.Errorf("expected %q, got %q", expected, tag)
	}
}

func TestBrevKeyTag_EmptyUserID(t *testing.T) {
	tag := BrevKeyTag("")
	if tag != BrevKeyPrefix {
		t.Errorf("expected %q, got %q", BrevKeyPrefix, tag)
	}
}

// --- ListBrevAuthorizedKeys ---

func TestListBrevAuthorizedKeys_ParsesNewFormat(t *testing.T) {
	u := tempUser(t)
	seedKeys(t, u, strings.Join([]string{
		"ssh-rsa EXISTING user@host",
		"ssh-ed25519 AAAA_ALICE # brev-cli user_id=user_1",
		"ssh-rsa AAAA_BOB # brev-cli user_id=user_2",
		"",
	}, "\n"))

	keys, err := ListBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("ListBrevAuthorizedKeys: %v", err)
	}

	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}

	if keys[0].KeyContent != "ssh-ed25519 AAAA_ALICE" {
		t.Errorf("expected key content 'ssh-ed25519 AAAA_ALICE', got %q", keys[0].KeyContent)
	}
	if keys[0].UserID != "user_1" {
		t.Errorf("expected user_id 'user_1', got %q", keys[0].UserID)
	}

	if keys[1].KeyContent != "ssh-rsa AAAA_BOB" {
		t.Errorf("expected key content 'ssh-rsa AAAA_BOB', got %q", keys[1].KeyContent)
	}
	if keys[1].UserID != "user_2" {
		t.Errorf("expected user_id 'user_2', got %q", keys[1].UserID)
	}
}

func TestListBrevAuthorizedKeys_ParsesOldFormat(t *testing.T) {
	u := tempUser(t)
	seedKeys(t, u, "ssh-ed25519 AAAA_OLD # brev-cli\n")

	keys, err := ListBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("ListBrevAuthorizedKeys: %v", err)
	}

	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
	if keys[0].KeyContent != "ssh-ed25519 AAAA_OLD" {
		t.Errorf("expected key content 'ssh-ed25519 AAAA_OLD', got %q", keys[0].KeyContent)
	}
	if keys[0].UserID != "" {
		t.Errorf("expected empty user_id for old format, got %q", keys[0].UserID)
	}
}

func TestListBrevAuthorizedKeys_MixedFormats(t *testing.T) {
	u := tempUser(t)
	seedKeys(t, u, strings.Join([]string{
		"ssh-rsa AAAA_OLD # brev-cli",
		"ssh-rsa NONBREV user@host",
		"ssh-ed25519 AAAA_NEW # brev-cli user_id=uid_42",
		"",
	}, "\n"))

	keys, err := ListBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("ListBrevAuthorizedKeys: %v", err)
	}

	if len(keys) != 2 {
		t.Fatalf("expected 2 brev keys, got %d", len(keys))
	}

	// Old format
	if keys[0].UserID != "" {
		t.Errorf("expected empty user_id for old format, got %q", keys[0].UserID)
	}
	// New format
	if keys[1].UserID != "uid_42" {
		t.Errorf("expected user_id 'uid_42', got %q", keys[1].UserID)
	}
}

func TestListBrevAuthorizedKeys_NoFile(t *testing.T) {
	u := tempUser(t)

	keys, err := ListBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(keys))
	}
}

func TestListBrevAuthorizedKeys_NoBrevKeys(t *testing.T) {
	u := tempUser(t)
	seedKeys(t, u, "ssh-rsa NONBREV user@host\n")

	keys, err := ListBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("ListBrevAuthorizedKeys: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected 0 brev keys, got %d", len(keys))
	}
}

// --- RemoveAuthorizedKeyLine ---

func TestRemoveAuthorizedKeyLine_RemovesExactLine(t *testing.T) {
	u := tempUser(t)
	seedKeys(t, u, strings.Join([]string{
		"ssh-rsa KEEP user@host",
		"ssh-ed25519 REMOVE # brev-cli user_id=user_1",
		"ssh-rsa KEEP2 admin@server",
		"",
	}, "\n"))

	if err := RemoveAuthorizedKeyLine(u, "ssh-ed25519 REMOVE # brev-cli user_id=user_1"); err != nil {
		t.Fatalf("RemoveAuthorizedKeyLine: %v", err)
	}

	result := readKeys(t, u)
	if strings.Contains(result, "REMOVE") {
		t.Errorf("line was not removed:\n%s", result)
	}
	if !strings.Contains(result, "ssh-rsa KEEP user@host") {
		t.Errorf("other key was removed:\n%s", result)
	}
	if !strings.Contains(result, "ssh-rsa KEEP2 admin@server") {
		t.Errorf("other key was removed:\n%s", result)
	}
}

func TestRemoveAuthorizedKeyLine_NoopForEmptyLine(t *testing.T) {
	u := tempUser(t)
	if err := RemoveAuthorizedKeyLine(u, ""); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRemoveAuthorizedKeyLine_NoopForMissingFile(t *testing.T) {
	u := tempUser(t)
	if err := RemoveAuthorizedKeyLine(u, "ssh-rsa SOMETHING"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

// --- InstallAuthorizedKey with user ID ---

func TestInstallAuthorizedKey_IncludesUserID(t *testing.T) {
	u := tempUser(t)

	if _, err := InstallAuthorizedKey(u, "ssh-rsa AAAA testkey", "user_abc"); err != nil {
		t.Fatalf("InstallAuthorizedKey: %v", err)
	}

	content := readKeys(t, u)
	expected := "ssh-rsa AAAA testkey # brev-cli user_id=user_abc"
	if !strings.Contains(content, expected) {
		t.Errorf("expected %q in authorized_keys, got:\n%s", expected, content)
	}
}

func TestInstallAuthorizedKey_EmptyUserID_UsesPrefix(t *testing.T) {
	u := tempUser(t)

	if _, err := InstallAuthorizedKey(u, "ssh-rsa AAAA testkey", ""); err != nil {
		t.Fatalf("InstallAuthorizedKey: %v", err)
	}

	content := readKeys(t, u)
	if !strings.Contains(content, "ssh-rsa AAAA testkey "+BrevKeyPrefix) {
		t.Errorf("expected key tagged with prefix, got:\n%s", content)
	}
	if strings.Contains(content, "user_id=") {
		t.Errorf("should not contain user_id when empty, got:\n%s", content)
	}
}
