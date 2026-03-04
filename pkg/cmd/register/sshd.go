package register

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	brevSSHDDir        = "/etc/brev-sshd"
	brevSSHDConfigPath = "/etc/brev-sshd/sshd_config"
	brevSSHDUnitPath   = "/etc/systemd/system/brev-sshd.service"
	brevSSHDHostKey = "/etc/brev-sshd/ssh_host_ed25519_key"
)

// sshdConfig is the hardened sshd_config for the brev-managed sshd on port 2222.
const sshdConfig = `# Brev-managed sshd configuration
# Do not edit — managed by brev register/deregister

# Non-standard port to avoid conflicting with the system sshd on port 22.
Port 2222

# Isolated ed25519 host key in brev's own directory for clean install/uninstall.
HostKey /etc/brev-sshd/ssh_host_ed25519_key

# Key-only authentication — brev manages keys via sshkeys.go.
PubkeyAuthentication yes
# Disable password auth to prevent brute-force attacks.
PasswordAuthentication no
# Disable PAM to ensure it can't re-enable password or keyboard-interactive auth.
UsePAM no
# Allow root login only via public key, never password.
PermitRootLogin prohibit-password
# Limit auth attempts per connection (default 6) to reduce brute-force window.
MaxAuthTries 3
# Disconnect unauthenticated sessions after 30s (default 120) to limit resource waste.
LoginGraceTime 30

# Reuse the same authorized_keys managed by sshkeys.go — no separate key store needed.
AuthorizedKeysFile %h/.ssh/authorized_keys

# Modern AEAD ciphers only; excludes legacy CBC and non-AEAD modes.
Ciphers chacha20-poly1305@openssh.com,aes256-gcm@openssh.com,aes128-gcm@openssh.com
# Post-quantum hybrid KEX + standard curve25519; excludes weak DH groups.
KexAlgorithms sntrup761x25519-sha512@openssh.com,curve25519-sha256
# Encrypt-then-MAC variants only; stronger than MAC-then-encrypt.
MACs hmac-sha2-256-etm@openssh.com,hmac-sha2-512-etm@openssh.com

# Dedicated PID file to avoid collision with system sshd's /run/sshd.pid.
PidFile /run/brev-sshd.pid
`

// sshdUnit is the systemd unit file for the brev-managed sshd.
// ExecStartPre validates the config before starting (fail-fast on typos).
// ExecStart runs sshd in foreground mode (-D) so systemd can supervise it.
// Restart=on-failure auto-recovers from crashes without restarting on clean exit.
const sshdUnit = `[Unit]
Description=Brev SSH Daemon (port 2222)
After=network.target

[Service]
Type=simple
ExecStartPre=/usr/sbin/sshd -t -f /etc/brev-sshd/sshd_config
ExecStart=/usr/sbin/sshd -D -f /etc/brev-sshd/sshd_config
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`

// InstallBrevSSHD sets up the brev-managed sshd on port 2222.
// It creates the config directory, generates a host key (idempotent),
// writes the sshd_config and systemd unit, then enables and starts the service.
func InstallBrevSSHD() error {
	// Create config directory
	if err := os.MkdirAll(brevSSHDDir, 0o755); err != nil {
		return fmt.Errorf("creating brev-sshd directory: %w", err)
	}

	// Generate ed25519 host key if it doesn't exist
	if _, err := os.Stat(brevSSHDHostKey); os.IsNotExist(err) {
		cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", brevSSHDHostKey, "-N", "") // #nosec G204
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("generating host key: %w", err)
		}
	}

	// Write sshd_config
	if err := os.WriteFile(brevSSHDConfigPath, []byte(sshdConfig), 0o644); err != nil {
		return fmt.Errorf("writing sshd_config: %w", err)
	}

	// Write systemd unit
	if err := os.WriteFile(brevSSHDUnitPath, []byte(sshdUnit), 0o644); err != nil {
		return fmt.Errorf("writing systemd unit: %w", err)
	}

	// Reload systemd, enable and start the service.
	// Each step is run separately so failures produce actionable messages.
	for _, args := range [][]string{
		{"systemctl", "daemon-reload"},
		{"systemctl", "enable", "brev-sshd.service"},
		{"systemctl", "start", "brev-sshd.service"},
	} {
		cmd := exec.Command("sudo", args...) // #nosec G204
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("running 'sudo %s': %w", strings.Join(args, " "), err)
		}
	}

	return nil
}

// UninstallBrevSSHD stops and removes the brev-managed sshd service and its
// configuration. Errors are best-effort — the function attempts all cleanup
// steps even if individual steps fail.
func UninstallBrevSSHD() error {
	// Stop and disable the service (best-effort)
	_ = exec.Command("bash", "-c", "sudo systemctl stop brev-sshd.service 2>/dev/null").Run()   // #nosec G204
	_ = exec.Command("bash", "-c", "sudo systemctl disable brev-sshd.service 2>/dev/null").Run() // #nosec G204

	// Remove systemd unit file
	if err := os.Remove(brevSSHDUnitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing systemd unit: %w", err)
	}

	// Reload systemd
	_ = exec.Command("bash", "-c", "sudo systemctl daemon-reload").Run() // #nosec G204

	// Remove config directory
	if err := os.RemoveAll(brevSSHDDir); err != nil {
		return fmt.Errorf("removing brev-sshd directory: %w", err)
	}

	// Remove PID file if leftover
	_ = os.Remove(filepath.Join("/run", "brev-sshd.pid"))

	return nil
}

// AddAllowedUser appends the given username to the AllowUsers directive in the
// brev-managed sshd_config. If AllowUsers does not yet exist, it is created.
// Idempotent: if the user is already listed, this is a no-op.
func AddAllowedUser(username string) error {
	return addAllowedUser(brevSSHDConfigPath, username)
}

func addAllowedUser(configPath, username string) error {
	data, err := os.ReadFile(configPath) // #nosec G304
	if err != nil {
		return fmt.Errorf("reading sshd_config: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "AllowUsers ") {
			continue
		}
		// AllowUsers line exists — check if user is already listed.
		users := strings.Fields(line)[1:]
		for _, u := range users {
			if u == username {
				return nil // already allowed
			}
		}
		lines[i] = line + " " + username
		return os.WriteFile(configPath, []byte(strings.Join(lines, "\n")), 0o644)
	}

	// No AllowUsers line — append one.
	lines = append(lines, "AllowUsers "+username)
	return os.WriteFile(configPath, []byte(strings.Join(lines, "\n")), 0o644)
}

// RemoveAllowedUser removes the given username from the AllowUsers directive.
// If removing the user leaves AllowUsers empty, the directive is removed entirely.
// No-op if the user is not listed or AllowUsers does not exist.
func RemoveAllowedUser(username string) error {
	return removeAllowedUser(brevSSHDConfigPath, username)
}

func removeAllowedUser(configPath, username string) error {
	data, err := os.ReadFile(configPath) // #nosec G304
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading sshd_config: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if !strings.HasPrefix(line, "AllowUsers ") {
			continue
		}
		users := strings.Fields(line)[1:]
		var kept []string
		for _, u := range users {
			if u != username {
				kept = append(kept, u)
			}
		}
		if len(kept) == 0 {
			lines = append(lines[:i], lines[i+1:]...)
		} else {
			lines[i] = "AllowUsers " + strings.Join(kept, " ")
		}
		return os.WriteFile(configPath, []byte(strings.Join(lines, "\n")), 0o644)
	}

	return nil // no AllowUsers line, nothing to do
}

// ReloadBrevSSHD sends a reload signal to the brev-sshd service so it picks
// up config changes without dropping existing connections.
func ReloadBrevSSHD() error {
	cmd := exec.Command("bash", "-c", "sudo systemctl reload brev-sshd.service") // #nosec G204
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reloading brev-sshd: %w", err)
	}
	return nil
}
