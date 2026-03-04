package register

import (
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
