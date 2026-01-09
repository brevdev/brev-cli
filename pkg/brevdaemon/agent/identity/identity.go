package identity

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"

	brevapiv2connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/brevapi/v2/brevapiv2connect"
	brevapiv2 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/brevapi/v2"
	"connectrpc.com/connect"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/client"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/config"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/telemetry"
	"github.com/brevdev/dev-plane/pkg/errors"
	"go.uber.org/zap"
)

// Identity represents the durable device identity for the agent.
type Identity struct {
	InstanceID              string
	DeviceToken             string
	HardwareFingerprint     string
	DeviceFingerprintHash   string
	DeviceFingerprintStored string
	DeviceSalt              string
}

// IdentityStore persists the instance identity to disk.
type IdentityStore struct {
	cfg                config.Config
	tokenPath          string
	instanceIDPath     string
	stateDir           string
	deviceSaltPath     string
	deviceFPHashPath   string
	deviceFPStoredPath string
	hardwareFPPath     string
}

// NewIdentityStore constructs a store backed by the configured state directory.
func NewIdentityStore(cfg config.Config) *IdentityStore {
	instancePath := filepath.Join(cfg.StateDir, "instance_id")
	deviceSaltPath := filepath.Join(cfg.StateDir, "device_salt")
	return &IdentityStore{
		cfg:                cfg,
		tokenPath:          cfg.DeviceTokenPath,
		instanceIDPath:     instancePath,
		stateDir:           cfg.StateDir,
		deviceSaltPath:     deviceSaltPath,
		deviceFPHashPath:   filepath.Join(cfg.StateDir, "device_fingerprint_hash"),
		deviceFPStoredPath: filepath.Join(cfg.StateDir, "device_fingerprint_stored"),
		hardwareFPPath:     filepath.Join(cfg.StateDir, "hardware_fingerprint"),
	}
}

// Load returns the stored identity if present.
func (s *IdentityStore) Load() (Identity, bool, error) {
	data, err := os.ReadFile(s.tokenPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Identity{}, false, nil
		}
		return Identity{}, false, errors.WrapAndTrace(err)
	}

	token := strings.TrimSpace(string(data))
	var instanceID string
	if data, err := os.ReadFile(s.instanceIDPath); err == nil {
		instanceID = strings.TrimSpace(string(data))
	}

	if err := ensurePermissions(s.tokenPath); err != nil {
		return Identity{}, false, errors.WrapAndTrace(err)
	}

	deviceSalt, err := readOptionalFile(s.deviceSaltPath)
	if err != nil {
		return Identity{}, false, errors.WrapAndTrace(err)
	}
	deviceFPHash, err := readOptionalFile(s.deviceFPHashPath)
	if err != nil {
		return Identity{}, false, errors.WrapAndTrace(err)
	}
	deviceFPStored, err := readOptionalFile(s.deviceFPStoredPath)
	if err != nil {
		return Identity{}, false, errors.WrapAndTrace(err)
	}
	hardwareFP, err := readOptionalFile(s.hardwareFPPath)
	if err != nil {
		return Identity{}, false, errors.WrapAndTrace(err)
	}

	if token == "" {
		return Identity{}, false, nil
	}
	return Identity{
		InstanceID:              instanceID,
		DeviceToken:             token,
		DeviceSalt:              deviceSalt,
		DeviceFingerprintHash:   deviceFPHash,
		DeviceFingerprintStored: deviceFPStored,
		HardwareFingerprint:     hardwareFP,
	}, true, nil
}

// Save writes the identity to disk.
func (s *IdentityStore) Save(id Identity) error {
	dir := s.stateDir
	if dir == "" {
		dir = filepath.Dir(s.tokenPath)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return errors.WrapAndTrace(err)
	}

	if err := writeRequired(s.tokenPath, id.DeviceToken); err != nil {
		return err
	}
	if err := writeOptional(s.deviceSaltPath, id.DeviceSalt); err != nil {
		return err
	}
	if err := writeOptional(s.deviceFPHashPath, id.DeviceFingerprintHash); err != nil {
		return err
	}
	if err := writeOptional(s.deviceFPStoredPath, id.DeviceFingerprintStored); err != nil {
		return err
	}
	if err := writeOptional(s.hardwareFPPath, id.HardwareFingerprint); err != nil {
		return err
	}
	return writeOptional(s.instanceIDPath, id.InstanceID)
}

func writeRequired(path, value string) error {
	if err := os.WriteFile(path, []byte(value+"\n"), 0o600); err != nil {
		return errors.WrapAndTrace(err)
	}
	if err := ensurePermissions(path); err != nil {
		return errors.WrapAndTrace(err)
	}
	return nil
}

func writeOptional(path, value string) error {
	if value == "" {
		return nil
	}
	return writeRequired(path, value)
}

// EnsureIdentity loads an existing device identity or registers a new one.
func EnsureIdentity(
	ctx context.Context,
	cfg config.Config,
	agentClient brevapiv2connect.BrevCloudAgentServiceClient,
	store *IdentityStore,
	hw telemetry.HardwareInfo,
	log *zap.Logger,
) (Identity, error) {
	if store == nil {
		return Identity{}, errors.Errorf("identity store is required")
	}
	if agentClient == nil {
		return Identity{}, errors.Errorf("brevcloud agent client is required")
	}

	id, ok, err := store.Load()
	if err != nil {
		return Identity{}, errors.WrapAndTrace(err)
	}
	if ok && id.DeviceToken != "" {
		log.Info("loaded device identity from disk",
			zap.String("instance_id", id.InstanceID))
		return id, nil
	}

	if cfg.RegistrationToken == "" {
		return Identity{}, errors.Errorf("registration token required to register device")
	}

	salt, err := ensureDeviceSalt(store.deviceSaltPath)
	if err != nil {
		return Identity{}, errors.WrapAndTrace(err)
	}

	req := &brevapiv2.RegisterRequest{
		RegistrationToken: cfg.RegistrationToken,
		Hardware:          hw.ToProto(),
	}
	if cfg.DisplayName != "" {
		req.DisplayName = client.ProtoString(cfg.DisplayName)
	}
	if cfg.CloudName != "" {
		req.CloudName = client.ProtoString(cfg.CloudName)
	}

	log.Info("registering device with brevcloud agent service")
	resp, err := agentClient.Register(ctx, connect.NewRequest(req))
	if err != nil {
		return Identity{}, errors.WrapAndTrace(client.ClassifyError(err))
	}

	newIdentity := Identity{
		InstanceID:              resp.Msg.GetBrevCloudNodeId(),
		DeviceToken:             resp.Msg.GetDeviceToken(),
		DeviceSalt:              salt,
		DeviceFingerprintStored: resp.Msg.GetDeviceFingerprint(),
	}
	if err := store.Save(newIdentity); err != nil {
		return Identity{}, errors.WrapAndTrace(err)
	}

	log.Info("device registration complete",
		zap.String("instance_id", newIdentity.InstanceID))
	return newIdentity, nil
}

func ensurePermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	const desired = 0o600
	if info.Mode().Perm() != desired {
		if err := os.Chmod(path, desired); err != nil {
			return errors.WrapAndTrace(err)
		}
	}
	return nil
}

func readOptionalFile(path string) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is derived from controlled config/state dir
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", errors.WrapAndTrace(err)
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return "", nil
	}
	if err := ensurePermissions(path); err != nil {
		return "", errors.WrapAndTrace(err)
	}
	return value, nil
}

func ensureDeviceSalt(path string) (string, error) {
	if value, err := readOptionalFile(path); err == nil && value != "" {
		return value, nil
	} else if err != nil {
		return "", errors.WrapAndTrace(err)
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", errors.WrapAndTrace(err)
	}
	salt := base64.RawURLEncoding.EncodeToString(raw)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", errors.WrapAndTrace(err)
	}
	if err := os.WriteFile(path, []byte(salt+"\n"), 0o600); err != nil {
		return "", errors.WrapAndTrace(err)
	}
	if err := ensurePermissions(path); err != nil {
		return "", errors.WrapAndTrace(err)
	}
	return salt, nil
}
