package register

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brevdev/brev-cli/pkg/terminal"
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

func TestDevplaneAuthorizedKeysComment(t *testing.T) {
	got := DevplaneAuthorizedKeysComment("nport_abc", "user_xyz")
	want := "#brev-portID:nport_abc,brev-userID:user_xyz"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestListBrevAuthorizedKeys_ParsesDevplaneFormat(t *testing.T) {
	u := tempUser(t)
	seedKeys(t, u, strings.Join([]string{
		"ssh-rsa EXISTING user@host",
		"ssh-ed25519 AAAA_ALICE user@a.com " + DevplaneAuthorizedKeysComment("port_1", "user_1"),
		"ssh-rsa AAAA_BOB " + DevplaneAuthorizedKeysComment("port_2", "user_2"),
		"",
	}, "\n"))

	keys, err := ListBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("ListBrevAuthorizedKeys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	if keys[0].PortID != "port_1" || keys[0].UserID != "user_1" {
		t.Errorf("key[0]: port=%q user=%q", keys[0].PortID, keys[0].UserID)
	}
	if keys[1].PortID != "port_2" || keys[1].UserID != "user_2" {
		t.Errorf("key[1]: port=%q user=%q", keys[1].PortID, keys[1].UserID)
	}
}

func TestListBrevAuthorizedKeys_ParsesLegacyFormat(t *testing.T) {
	u := tempUser(t)
	seedKeys(t, u, "ssh-ed25519 AAAA_OLD # brev-cli user_id=uid_42\n")

	keys, err := ListBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("ListBrevAuthorizedKeys: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
	if keys[0].UserID != "uid_42" {
		t.Errorf("expected user_id uid_42, got %q", keys[0].UserID)
	}
}

func TestListBrevAuthorizedKeys_MixedFormats(t *testing.T) {
	u := tempUser(t)
	seedKeys(t, u, strings.Join([]string{
		"ssh-rsa AAAA_LEGACY # brev-cli",
		"ssh-rsa NONBREV user@host",
		"ssh-ed25519 AAAA_NEW " + DevplaneAuthorizedKeysComment("p1", "uid_42"),
		"",
	}, "\n"))

	keys, err := ListBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("ListBrevAuthorizedKeys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 brev keys, got %d", len(keys))
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

func TestRemoveAuthorizedKeyLine_RemovesExactLine(t *testing.T) {
	u := tempUser(t)
	line := "ssh-ed25519 REMOVE " + DevplaneAuthorizedKeysComment("p1", "user_1")
	seedKeys(t, u, strings.Join([]string{
		"ssh-rsa KEEP user@host",
		line,
		"",
	}, "\n"))

	if err := RemoveAuthorizedKeyLine(u, line); err != nil {
		t.Fatalf("RemoveAuthorizedKeyLine: %v", err)
	}
	if strings.Contains(readKeys(t, u), "REMOVE") {
		t.Fatal("line was not removed")
	}
}

func TestRemoveBrevAuthorizedKeys_DevplaneLines(t *testing.T) {
	u := tempUser(t)
	seedKeys(t, u, strings.Join([]string{
		"ssh-rsa KEEP user@host",
		"ssh-rsa BREV1 " + DevplaneAuthorizedKeysComment("p1", "u1"),
		"ssh-rsa BREV2 " + DevplaneAuthorizedKeysComment("p2", "u2"),
		"",
	}, "\n"))

	removed, err := RemoveBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("RemoveBrevAuthorizedKeys: %v", err)
	}
	if len(removed) != 2 {
		t.Fatalf("expected 2 removed, got %d", len(removed))
	}
	result := readKeys(t, u)
	if strings.Contains(result, "#brev-portID:") {
		t.Errorf("brev keys remain:\n%s", result)
	}
	if !strings.Contains(result, "KEEP") {
		t.Error("non-brev key was removed")
	}
}

func TestRemoveBrevAuthorizedKeys_LegacyLines(t *testing.T) {
	u := tempUser(t)
	seedKeys(t, u, "ssh-rsa BREVKEY "+BrevKeyPrefixLegacy+"\n")
	removed, err := RemoveBrevAuthorizedKeys(u)
	if err != nil {
		t.Fatalf("RemoveBrevAuthorizedKeys: %v", err)
	}
	if len(removed) != 1 {
		t.Fatalf("expected 1 removed, got %d", len(removed))
	}
}

func TestInstallAuthorizedKey_AppendsDevplaneComment(t *testing.T) {
	u := tempUser(t)
	pub := "ssh-rsa AAAA testkey user@example.com"
	wantTag := DevplaneAuthorizedKeysComment("port_1", "user_1")
	if _, err := InstallAuthorizedKey(u, pub, "port_1", "user_1"); err != nil {
		t.Fatal(err)
	}
	content := readKeys(t, u)
	if !strings.Contains(content, pub+" "+wantTag) {
		t.Fatalf("expected devplane-tagged key, got:\n%s", content)
	}
}

func TestInstallAuthorizedKey_SkipsDuplicate(t *testing.T) {
	u := tempUser(t)
	pub := "ssh-rsa AAAA testkey"
	if _, err := InstallAuthorizedKey(u, pub, "port_1", "user_1"); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallAuthorizedKey(u, pub, "port_1", "user_1"); err != nil {
		t.Fatal(err)
	}
	if strings.Count(readKeys(t, u), "ssh-rsa AAAA testkey") != 1 {
		t.Fatal("expected single key line")
	}
}

func TestInstallAuthorizedKey_UpgradesUntaggedKey(t *testing.T) {
	u := tempUser(t)
	pub := "ssh-rsa AAAA testkey"
	seedKeys(t, u, pub+"\n")
	wantTag := DevplaneAuthorizedKeysComment("port_1", "user_1")
	if _, err := InstallAuthorizedKey(u, pub, "port_1", "user_1"); err != nil {
		t.Fatal(err)
	}
	content := readKeys(t, u)
	if !strings.Contains(content, pub+" "+wantTag) {
		t.Fatalf("expected upgraded tag, got:\n%s", content)
	}
}

func TestInstallAuthorizedKey_secondPortAppendsNewLine(t *testing.T) {
	u := tempUser(t)
	pub := "ssh-rsa AAAA testkey user@example.com"
	line1 := pub + " " + DevplaneAuthorizedKeysComment("port_a", "user_1")
	seedKeys(t, u, line1+"\n")

	if _, err := InstallAuthorizedKey(u, pub, "port_b", "user_1"); err != nil {
		t.Fatal(err)
	}
	content := readKeys(t, u)
	if strings.Count(content, "#brev-portID:") != 2 {
		t.Fatalf("expected two port-tagged lines, got:\n%s", content)
	}
	if !strings.Contains(content, DevplaneAuthorizedKeysComment("port_a", "user_1")) {
		t.Fatal("first port line should remain")
	}
	if !strings.Contains(content, DevplaneAuthorizedKeysComment("port_b", "user_1")) {
		t.Fatal("second port line should be added")
	}
	if strings.Count(content, "port_a") != 1 || strings.Count(content, "port_b") != 1 {
		t.Fatalf("merged comments on one line:\n%s", content)
	}
}

func TestRemoveAuthorizedKey_ByPublicKeyMaterial(t *testing.T) {
	u := tempUser(t)
	pub := "ssh-rsa AAAA testkey"
	seedKeys(t, u, pub+" user@host "+DevplaneAuthorizedKeysComment("p1", "u1")+"\n")
	if err := RemoveAuthorizedKey(u, pub); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(readKeys(t, u), "AAAA") {
		t.Fatal("key material should be removed")
	}
}

// --- PromptSSHPort ---

func TestPromptSSHPort(t *testing.T) {
	SetTestSSHPort(2222)
	defer ClearTestSSHPort()
	port, err := PromptSSHPort(terminal.New())
	if err != nil {
		t.Fatalf("PromptSSHPort: %v", err)
	}
	if port != 2222 {
		t.Errorf("expected 2222, got %d", port)
	}
}
