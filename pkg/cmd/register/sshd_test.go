package register

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test_sshdConfig_Content(t *testing.T) {
	checks := []struct {
		name    string
		want    string
	}{
		{"Port", "Port 2222"},
		{"HostKey", "HostKey /etc/brev-sshd/ssh_host_ed25519_key"},
		{"PubkeyAuth", "PubkeyAuthentication yes"},
		{"PasswordAuth", "PasswordAuthentication no"},
		{"UsePAM", "UsePAM no"},
		{"PermitRootLogin", "PermitRootLogin prohibit-password"},
		{"MaxAuthTries", "MaxAuthTries 3"},
		{"LoginGraceTime", "LoginGraceTime 30"},
		{"AuthorizedKeysFile", "AuthorizedKeysFile %h/.ssh/authorized_keys"},
		{"PidFile", "PidFile /run/brev-sshd.pid"},
		{"Chacha20Cipher", "chacha20-poly1305@openssh.com"},
		{"AES256GCM", "aes256-gcm@openssh.com"},
		{"AES128GCM", "aes128-gcm@openssh.com"},
		{"Sntrup761KEX", "sntrup761x25519-sha512@openssh.com"},
		{"Curve25519KEX", "curve25519-sha256"},
		{"HMACSHA256ETM", "hmac-sha2-256-etm@openssh.com"},
		{"HMACSHA512ETM", "hmac-sha2-512-etm@openssh.com"},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(sshdConfig, tc.want) {
				t.Errorf("sshd_config missing %q", tc.want)
			}
		})
	}
}

func Test_sshdUnit_Content(t *testing.T) {
	checks := []struct {
		name string
		want string
	}{
		{"Description", "Description=Brev SSH Daemon (port 2222)"},
		{"ExecStartPre", "ExecStartPre=/usr/sbin/sshd -t -f /etc/brev-sshd/sshd_config"},
		{"ExecStart", "ExecStart=/usr/sbin/sshd -D -f /etc/brev-sshd/sshd_config"},
		{"Restart", "Restart=on-failure"},
		{"WantedBy", "WantedBy=multi-user.target"},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(sshdUnit, tc.want) {
				t.Errorf("systemd unit missing %q", tc.want)
			}
		})
	}
}

func Test_BrevSSHD_ImplementsInterface(t *testing.T) {
	// Compile-time check that BrevSSHD satisfies ManagedSSHDaemon.
	var _ ManagedSSHDaemon = BrevSSHD{}
}

// helper to write a temp sshd_config and return its path.
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "sshd_config")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("writing temp config: %v", err)
	}
	return p
}

func readConfig(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}
	return string(data)
}

func Test_addAllowedUser_CreatesDirective(t *testing.T) {
	p := writeTempConfig(t, "Port 2222\n")
	if err := addAllowedUser(p, "alice"); err != nil {
		t.Fatalf("addAllowedUser: %v", err)
	}
	got := readConfig(t, p)
	if !strings.Contains(got, "AllowUsers alice") {
		t.Errorf("expected AllowUsers alice, got:\n%s", got)
	}
}

func Test_addAllowedUser_AppendsToExisting(t *testing.T) {
	p := writeTempConfig(t, "Port 2222\nAllowUsers alice\n")
	if err := addAllowedUser(p, "bob"); err != nil {
		t.Fatalf("addAllowedUser: %v", err)
	}
	got := readConfig(t, p)
	if !strings.Contains(got, "AllowUsers alice bob") {
		t.Errorf("expected AllowUsers alice bob, got:\n%s", got)
	}
}

func Test_addAllowedUser_Idempotent(t *testing.T) {
	p := writeTempConfig(t, "Port 2222\nAllowUsers alice\n")
	if err := addAllowedUser(p, "alice"); err != nil {
		t.Fatalf("addAllowedUser: %v", err)
	}
	got := readConfig(t, p)
	if strings.Count(got, "alice") != 1 {
		t.Errorf("expected exactly one 'alice', got:\n%s", got)
	}
}

func Test_removeAllowedUser_RemovesUser(t *testing.T) {
	p := writeTempConfig(t, "Port 2222\nAllowUsers alice bob\n")
	if err := removeAllowedUser(p, "alice"); err != nil {
		t.Fatalf("removeAllowedUser: %v", err)
	}
	got := readConfig(t, p)
	if !strings.Contains(got, "AllowUsers bob") {
		t.Errorf("expected AllowUsers bob, got:\n%s", got)
	}
	if strings.Contains(got, "alice") {
		t.Errorf("alice should have been removed, got:\n%s", got)
	}
}

func Test_removeAllowedUser_RemovesDirectiveWhenEmpty(t *testing.T) {
	p := writeTempConfig(t, "Port 2222\nAllowUsers alice\n")
	if err := removeAllowedUser(p, "alice"); err != nil {
		t.Fatalf("removeAllowedUser: %v", err)
	}
	got := readConfig(t, p)
	if strings.Contains(got, "AllowUsers") {
		t.Errorf("AllowUsers directive should be removed, got:\n%s", got)
	}
}

func Test_removeAllowedUser_NoOpWhenMissing(t *testing.T) {
	p := writeTempConfig(t, "Port 2222\n")
	if err := removeAllowedUser(p, "alice"); err != nil {
		t.Fatalf("removeAllowedUser: %v", err)
	}
}

func Test_removeAllowedUser_NoOpWhenFileAbsent(t *testing.T) {
	if err := removeAllowedUser("/nonexistent/path/sshd_config", "alice"); err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
}
