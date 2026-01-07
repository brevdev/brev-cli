package identity

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/client"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/config"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/telemetry"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestIdentityStoreSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		StateDir:        dir,
		DeviceTokenPath: filepath.Join(dir, "device_token"),
	}

	store := NewIdentityStore(cfg)
	id := Identity{
		InstanceID:              "inst-123",
		DeviceToken:             "token-abc",
		DeviceSalt:              "salt-xyz",
		DeviceFingerprintHash:   "fp-device-hash",
		DeviceFingerprintStored: "fp-device-scoped",
		HardwareFingerprint:     "fp-hw",
	}

	require.NoError(t, store.Save(id))

	loaded, ok, err := store.Load()
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, id.InstanceID, loaded.InstanceID)
	require.Equal(t, id.DeviceToken, loaded.DeviceToken)
	require.Equal(t, id.DeviceSalt, loaded.DeviceSalt)
	require.Equal(t, id.DeviceFingerprintHash, loaded.DeviceFingerprintHash)
	require.Equal(t, id.DeviceFingerprintStored, loaded.DeviceFingerprintStored)
	require.Equal(t, id.HardwareFingerprint, loaded.HardwareFingerprint)
}

func TestIdentityStoreLoadMissing(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		StateDir:        dir,
		DeviceTokenPath: filepath.Join(dir, "device_token"),
	}

	store := NewIdentityStore(cfg)
	_, ok, err := store.Load()
	require.NoError(t, err)
	require.False(t, ok)
}

func TestEnsureIdentityUsesStoredToken(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		StateDir:        dir,
		DeviceTokenPath: filepath.Join(dir, "device_token"),
	}
	store := NewIdentityStore(cfg)

	err := store.Save(Identity{
		InstanceID:            "existing",
		DeviceToken:           "token",
		DeviceFingerprintHash: "existing-fp-hash",
		HardwareFingerprint:   "existing-hw",
	})
	require.NoError(t, err)

	var registerCalled bool
	client := &stubClient{
		registerFn: func(context.Context, client.RegisterParams) (client.RegisterResult, error) {
			registerCalled = true
			return client.RegisterResult{}, nil
		},
	}

	hw := telemetry.HardwareInfo{}
	log := zaptest.NewLogger(t)
	id, err := EnsureIdentity(context.Background(), cfg, client, store, hw, log)
	require.NoError(t, err)
	require.Equal(t, "existing", id.InstanceID)
	require.False(t, registerCalled)
}

func TestEnsureIdentityRegistersWhenMissing(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		StateDir:          dir,
		DeviceTokenPath:   filepath.Join(dir, "device_token"),
		RegistrationToken: "reg-token",
		DisplayName:       "node",
		CloudName:         "cloud",
	}

	store := NewIdentityStore(cfg)

	client := &stubClient{
		registerFn: func(_ context.Context, params client.RegisterParams) (client.RegisterResult, error) {
			require.Equal(t, "reg-token", params.RegistrationToken)
			require.Equal(t, "node", params.DisplayName)
			require.Equal(t, "cloud", params.CloudName)
			require.NotEmpty(t, params.DeviceFingerprintHash)
			require.NotEmpty(t, params.HardwareFingerprint)
			return client.RegisterResult{
				BrevCloudNodeID:   "fn-new",
				DeviceToken:       "new-token",
				CloudCredID:       "cc-123",
				DeviceFingerprint: "scoped-device-fp",
			}, nil
		},
	}

	hw := telemetry.HardwareInfo{CPUCount: 8, RAMBytes: 1024}
	log := zaptest.NewLogger(t)

	id, err := EnsureIdentity(context.Background(), cfg, client, store, hw, log)
	require.NoError(t, err)
	require.Equal(t, "fn-new", id.InstanceID)
	require.Equal(t, "new-token", id.DeviceToken)
	require.Equal(t, "scoped-device-fp", id.DeviceFingerprintStored)
	require.NotEmpty(t, id.DeviceFingerprintHash)

	data, err := os.ReadFile(cfg.DeviceTokenPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "new-token")
}

func TestEnsureIdentityRequiresRegistrationToken(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{
		StateDir:        dir,
		DeviceTokenPath: filepath.Join(dir, "device_token"),
	}
	store := NewIdentityStore(cfg)
	log := zaptest.NewLogger(t)

	client := &stubClient{}
	_, err := EnsureIdentity(context.Background(), cfg, client, store, telemetry.HardwareInfo{}, log)
	require.Error(t, err)
}

type stubClient struct {
	registerFn func(ctx context.Context, params client.RegisterParams) (client.RegisterResult, error)
}

func (s *stubClient) Register(ctx context.Context, params client.RegisterParams) (client.RegisterResult, error) {
	if s.registerFn != nil {
		return s.registerFn(ctx, params)
	}
	return client.RegisterResult{}, nil
}

func (s *stubClient) Heartbeat(context.Context, client.HeartbeatParams) (client.HeartbeatResult, error) {
	return client.HeartbeatResult{}, nil
}

func (s *stubClient) GetTunnelToken(context.Context, client.TunnelTokenParams) (client.TunnelTokenResult, error) {
	return client.TunnelTokenResult{}, nil
}
