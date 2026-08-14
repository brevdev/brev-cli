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
