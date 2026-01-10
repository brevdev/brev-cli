package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	unsetConfigEnv(t)
	t.Setenv(EnvBrevCloudURL, "https://example.dev/v1/brevcloudagent")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	home, _ := os.UserHomeDir()
	expectedStateDir := filepath.Join(home, defaultStateDirName)
	if cfg.StateDir != expectedStateDir {
		t.Fatalf("StateDir = %s, want %s", cfg.StateDir, expectedStateDir)
	}

	expectedDevicePath := filepath.Join(expectedStateDir, defaultDeviceTokenName)
	if cfg.DeviceTokenPath != expectedDevicePath {
		t.Fatalf("DeviceTokenPath = %s, want %s", cfg.DeviceTokenPath, expectedDevicePath)
	}

	if cfg.HeartbeatInterval != defaultHeartbeatInterval {
		t.Fatalf("HeartbeatInterval = %s, want %s", cfg.HeartbeatInterval, defaultHeartbeatInterval)
	}

	if !cfg.EnableTunnel {
		t.Fatalf("EnableTunnel = false, want true")
	}

	if cfg.TunnelSSHPort != defaultTunnelSSHPort {
		t.Fatalf("TunnelSSHPort = %d, want %d", cfg.TunnelSSHPort, defaultTunnelSSHPort)
	}

	if !cfg.TunnelCritical {
		t.Fatalf("TunnelCritical = false, want true")
	}
}

func TestLoadOverrides(t *testing.T) {
	unsetConfigEnv(t)
	t.Setenv(EnvBrevCloudURL, "https://example.dev/v1")
	t.Setenv(EnvRegistrationToken, "secret")
	t.Setenv(EnvDisplayName, "edge-node")
	t.Setenv(EnvCloudName, "private")
	t.Setenv(EnvStateDir, "~/custom/.brevagent")
	t.Setenv(EnvDeviceTokenPath, "~/custom/device_token.json")
	t.Setenv(EnvHeartbeatInterval, "45s")
	t.Setenv(EnvEnableTunnel, "false")
	t.Setenv(EnvTunnelSSHPort, "2202")
	t.Setenv(EnvTunnelCritical, "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.RegistrationToken != "secret" {
		t.Fatalf("RegistrationToken = %s, want secret", cfg.RegistrationToken)
	}
	if cfg.DisplayName != "edge-node" {
		t.Fatalf("DisplayName = %s, want edge-node", cfg.DisplayName)
	}
	if cfg.CloudName != "private" {
		t.Fatalf("CloudName = %s, want private", cfg.CloudName)
	}
	if cfg.HeartbeatInterval != 45*time.Second {
		t.Fatalf("HeartbeatInterval = %s, want 45s", cfg.HeartbeatInterval)
	}
	if cfg.EnableTunnel {
		t.Fatalf("EnableTunnel = true, want false")
	}
	if cfg.TunnelSSHPort != 2202 {
		t.Fatalf("TunnelSSHPort = %d, want 2202", cfg.TunnelSSHPort)
	}
	if cfg.TunnelCritical {
		t.Fatalf("TunnelCritical = true, want false")
	}
	home, _ := os.UserHomeDir()
	expectedStateDir := filepath.Join(home, "custom", ".brevagent")
	if cfg.StateDir != expectedStateDir {
		t.Fatalf("StateDir = %s, want %s", cfg.StateDir, expectedStateDir)
	}
	expectedDevicePath := filepath.Join(home, "custom", "device_token.json")
	if cfg.DeviceTokenPath != expectedDevicePath {
		t.Fatalf("DeviceTokenPath = %s, want %s", cfg.DeviceTokenPath, expectedDevicePath)
	}
}

func TestLoadMissingBrevCloudURL(t *testing.T) {
	unsetConfigEnv(t)
	_, err := Load()
	if err == nil {
		t.Fatalf("expected error for missing brevcloud url")
	}
}

func TestLoadInvalidValues(t *testing.T) {
	unsetConfigEnv(t)
	t.Setenv(EnvBrevCloudURL, "https://example.dev/v1")
	t.Setenv(EnvHeartbeatInterval, "abc")

	if _, err := Load(); err == nil {
		t.Fatalf("expected invalid interval error")
	}

	t.Setenv(EnvHeartbeatInterval, "30s")
	t.Setenv(EnvEnableTunnel, "not-bool")
	if _, err := Load(); err == nil {
		t.Fatalf("expected invalid bool error")
	}

	t.Setenv(EnvEnableTunnel, "true")
	t.Setenv(EnvTunnelSSHPort, "100000")
	if _, err := Load(); err == nil {
		t.Fatalf("expected invalid port error")
	}

	t.Setenv(EnvTunnelSSHPort, "22")
	t.Setenv(EnvTunnelCritical, "not-bool")
	if _, err := Load(); err == nil {
		t.Fatalf("expected invalid tunnel critical bool error")
	}
}

func unsetConfigEnv(t *testing.T) {
	t.Helper()
	envs := []string{
		EnvBrevCloudURL,
		EnvRegistrationToken,
		EnvDisplayName,
		EnvCloudName,
		EnvCloudCredID,
		EnvBrevCloudNodeID,
		EnvStateDir,
		EnvDeviceTokenPath,
		EnvHeartbeatInterval,
		EnvEnableTunnel,
		EnvTunnelSSHPort,
		EnvTunnelCritical,
	}
	for _, key := range envs {
		t.Setenv(key, "")
	}
}
