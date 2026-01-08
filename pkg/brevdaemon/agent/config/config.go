package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/brevdev/dev-plane/pkg/errors"
)

const (
	EnvBrevCloudURL           = "BREV_AGENT_BREV_CLOUD_URL"
	EnvRegistrationToken      = "BREV_AGENT_REGISTRATION_TOKEN" //nolint:gosec // token env var name, not secret
	EnvDisplayName            = "BREV_AGENT_DISPLAY_NAME"
	EnvCloudName              = "BREV_AGENT_CLOUD_NAME"
	EnvCloudCredID            = "BREV_AGENT_CLOUD_CRED_ID" //nolint:gosec // credential env var name, not secret
	EnvBrevCloudNodeID        = "BREV_AGENT_BREV_CLOUD_NODE_ID"
	EnvStateDir               = "BREV_AGENT_STATE_DIR"
	EnvDeviceTokenPath        = "BREV_AGENT_DEVICE_TOKEN_PATH" //nolint:gosec // path key, not a credential
	EnvHeartbeatInterval      = "BREV_AGENT_HEARTBEAT_INTERVAL"
	EnvEnableTunnel           = "BREV_AGENT_ENABLE_TUNNEL"
	EnvTunnelSSHPort          = "BREV_AGENT_TUNNEL_SSH_PORT"
	EnvTunnelCritical         = "BREV_AGENT_TUNNEL_CRITICAL"
	defaultStateDirName       = ".brev-agent"
	defaultDeviceTokenName    = "device_token"
	defaultHeartbeatInterval  = 30 * time.Second
	defaultTunnelSSHPort      = 22
	defaultEnableTunnel       = true
	defaultTunnelCritical     = true
	fallbackRelativeStateBase = "."
)

// Config captures all runtime configuration for the brev-agent binary.
type Config struct {
	BrevCloudAgentURL string
	RegistrationToken string
	DisplayName       string
	CloudName         string

	StateDir        string
	DeviceTokenPath string

	HeartbeatInterval time.Duration

	EnableTunnel   bool
	TunnelSSHPort  int
	TunnelCritical bool
}

// Load constructs a Config from environment variables, applying defaults and validations
func Load() (Config, error) {
	cfg := Config{
		HeartbeatInterval: defaultHeartbeatInterval,
		EnableTunnel:      defaultEnableTunnel,
		TunnelSSHPort:     defaultTunnelSSHPort,
		TunnelCritical:    defaultTunnelCritical,
	}

	cfg.BrevCloudAgentURL = strings.TrimSpace(os.Getenv(EnvBrevCloudURL))
	if cfg.BrevCloudAgentURL == "" {
		return Config{}, errors.Errorf("%s is required", EnvBrevCloudURL)
	}

	cfg.RegistrationToken = strings.TrimSpace(os.Getenv(EnvRegistrationToken))
	cfg.DisplayName = strings.TrimSpace(os.Getenv(EnvDisplayName))
	cfg.CloudName = strings.TrimSpace(os.Getenv(EnvCloudName))
	stateDir, err := deriveStateDir(os.Getenv(EnvStateDir))
	if err != nil {
		return Config{}, errors.WrapAndTrace(err)
	}
	cfg.StateDir = stateDir

	deviceToken, err := deriveDeviceTokenPath(os.Getenv(EnvDeviceTokenPath), stateDir)
	if err != nil {
		return Config{}, errors.WrapAndTrace(err)
	}
	cfg.DeviceTokenPath = deviceToken

	if intervalRaw := strings.TrimSpace(os.Getenv(EnvHeartbeatInterval)); intervalRaw != "" {
		interval, err := time.ParseDuration(intervalRaw)
		if err != nil {
			return Config{}, errors.WrapAndTrace(errors.Errorf("%s must be a valid duration: %v", EnvHeartbeatInterval, err))
		}
		if interval <= 0 {
			return Config{}, errors.Errorf("%s must be positive", EnvHeartbeatInterval)
		}
		cfg.HeartbeatInterval = interval
	}

	if enableTunnelRaw := strings.TrimSpace(os.Getenv(EnvEnableTunnel)); enableTunnelRaw != "" {
		val, err := strconv.ParseBool(enableTunnelRaw)
		if err != nil {
			return Config{}, errors.WrapAndTrace(errors.Errorf("%s must be a boolean: %v", EnvEnableTunnel, err))
		}
		cfg.EnableTunnel = val
	}

	if portRaw := strings.TrimSpace(os.Getenv(EnvTunnelSSHPort)); portRaw != "" {
		port, err := strconv.Atoi(portRaw)
		if err != nil {
			return Config{}, errors.WrapAndTrace(errors.Errorf("%s must be an integer: %v", EnvTunnelSSHPort, err))
		}
		if port <= 0 || port > 65535 {
			return Config{}, errors.Errorf("%s must be between 1 and 65535", EnvTunnelSSHPort)
		}
		cfg.TunnelSSHPort = port
	}

	if tunnelCriticalRaw := strings.TrimSpace(os.Getenv(EnvTunnelCritical)); tunnelCriticalRaw != "" {
		val, err := strconv.ParseBool(tunnelCriticalRaw)
		if err != nil {
			return Config{}, errors.WrapAndTrace(errors.Errorf("%s must be a boolean: %v", EnvTunnelCritical, err))
		}
		cfg.TunnelCritical = val
	}

	return cfg, nil
}

func deriveStateDir(override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return expandPath(override)
	}

	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, defaultStateDirName), nil
	}

	return filepath.Join(fallbackRelativeStateBase, defaultStateDirName), nil
}

func deriveDeviceTokenPath(override, stateDir string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return expandPath(override)
	}
	return filepath.Join(stateDir, defaultDeviceTokenName), nil
}

func expandPath(path string) (string, error) {
	if path == "" {
		return "", errors.Errorf("path cannot be empty")
	}

	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", errors.WrapAndTrace(err)
		}
		if home == "" {
			return "", errors.Errorf("cannot expand ~, home directory unset")
		}
		path = filepath.Join(home, path[1:])
	}

	return filepath.Clean(path), nil
}
