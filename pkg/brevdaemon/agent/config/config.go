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
	envPrefix                 = "BREV_AGENT_"
	envBrevCloudURL           = envPrefix + "BREV_CLOUD_URL"
	envRegistrationToken      = envPrefix + "REGISTRATION_TOKEN"
	envDisplayName            = envPrefix + "DISPLAY_NAME"
	envCloudName              = envPrefix + "CLOUD_NAME"
	envStateDir               = envPrefix + "STATE_DIR"
	envDeviceTokenPath        = envPrefix + "DEVICE_TOKEN_PATH"
	envHeartbeatInterval      = envPrefix + "HEARTBEAT_INTERVAL"
	envEnableTunnel           = envPrefix + "ENABLE_TUNNEL"
	envTunnelSSHPort          = envPrefix + "TUNNEL_SSH_PORT"
	envTunnelCritical         = envPrefix + "TUNNEL_CRITICAL"
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

// Load constructs a Config from environment variables, applying defaults and
// validation as defined in the MVP plan.
func Load() (Config, error) {
	cfg := Config{
		HeartbeatInterval: defaultHeartbeatInterval,
		EnableTunnel:      defaultEnableTunnel,
		TunnelSSHPort:     defaultTunnelSSHPort,
		TunnelCritical:    defaultTunnelCritical,
	}

	cfg.BrevCloudAgentURL = strings.TrimSpace(os.Getenv(envBrevCloudURL))
	if cfg.BrevCloudAgentURL == "" {
		return Config{}, errors.Errorf("%s is required", envBrevCloudURL)
	}

	cfg.RegistrationToken = strings.TrimSpace(os.Getenv(envRegistrationToken))
	cfg.DisplayName = strings.TrimSpace(os.Getenv(envDisplayName))
	cfg.CloudName = strings.TrimSpace(os.Getenv(envCloudName))
	stateDir, err := deriveStateDir(os.Getenv(envStateDir))
	if err != nil {
		return Config{}, errors.WrapAndTrace(err)
	}
	cfg.StateDir = stateDir

	deviceToken, err := deriveDeviceTokenPath(os.Getenv(envDeviceTokenPath), stateDir)
	if err != nil {
		return Config{}, errors.WrapAndTrace(err)
	}
	cfg.DeviceTokenPath = deviceToken

	if intervalRaw := strings.TrimSpace(os.Getenv(envHeartbeatInterval)); intervalRaw != "" {
		interval, err := time.ParseDuration(intervalRaw)
		if err != nil {
			return Config{}, errors.WrapAndTrace(errors.Errorf("%s must be a valid duration: %v", envHeartbeatInterval, err))
		}
		if interval <= 0 {
			return Config{}, errors.Errorf("%s must be positive", envHeartbeatInterval)
		}
		cfg.HeartbeatInterval = interval
	}

	if enableTunnelRaw := strings.TrimSpace(os.Getenv(envEnableTunnel)); enableTunnelRaw != "" {
		val, err := strconv.ParseBool(enableTunnelRaw)
		if err != nil {
			return Config{}, errors.WrapAndTrace(errors.Errorf("%s must be a boolean: %v", envEnableTunnel, err))
		}
		cfg.EnableTunnel = val
	}

	if portRaw := strings.TrimSpace(os.Getenv(envTunnelSSHPort)); portRaw != "" {
		port, err := strconv.Atoi(portRaw)
		if err != nil {
			return Config{}, errors.WrapAndTrace(errors.Errorf("%s must be an integer: %v", envTunnelSSHPort, err))
		}
		if port <= 0 || port > 65535 {
			return Config{}, errors.Errorf("%s must be between 1 and 65535", envTunnelSSHPort)
		}
		cfg.TunnelSSHPort = port
	}

	if tunnelCriticalRaw := strings.TrimSpace(os.Getenv(envTunnelCritical)); tunnelCriticalRaw != "" {
		val, err := strconv.ParseBool(tunnelCriticalRaw)
		if err != nil {
			return Config{}, errors.WrapAndTrace(errors.Errorf("%s must be a boolean: %v", envTunnelCritical, err))
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
