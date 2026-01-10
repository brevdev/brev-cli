package tunnel

import (
	"context"
	cryptorand "crypto/rand"
	"fmt"
	"math"
	"math/big"
	"os"
	"os/exec"
	"time"

	brevapiv2connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/brevapi/v2/brevapiv2connect"
	brevapiv2 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/brevapi/v2"
	"connectrpc.com/connect"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/client"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/health"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/identity"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/telemetry"
	"github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/dev-plane/pkg/brevcloud/appaccess"
	"go.uber.org/zap"
)

const (
	defaultTunnelName = "default"

	minRetryDelay = 5 * time.Second
	maxRetryDelay = 5 * time.Minute
)

// TunnelConfig configures tunnel management.
type TunnelConfig struct {
	SSHPort      int32
	ClientBinary string
}

// Manager fetches tunnel tokens and boots the tunnel client.
type Manager struct {
	Client   brevapiv2connect.BrevCloudAgentServiceClient
	Identity identity.Identity
	Cfg      TunnelConfig
	Log      *zap.Logger

	CommandFactory func(ctx context.Context, name string, args ...string) Command
	Health         StatusPublisher
	Sleep          func(context.Context, time.Duration) error
	LookPath       func(string) (string, error)

	DetectHardware func(context.Context) (telemetry.HardwareInfo, error)
	ProbeTimeout   time.Duration
	AppConfig      appaccess.Config

	jitter func(time.Duration) time.Duration
}

// Command abstracts exec.Cmd for testability.
type Command interface {
	Start() error
	Wait() error
	SetEnv([]string)
	CombinedOutput() ([]byte, error)
}

// StatusPublisher captures the subset of health reporter APIs used by the tunnel manager.
type StatusPublisher interface {
	MarkActive(detail string) health.Status
	MarkError(detail string) health.Status
}

// Start requests a tunnel token and launches the tunnel binary. It blocks until
// the context is canceled or the process exits unexpectedly.
func (m *Manager) Start(ctx context.Context) error { //nolint:gocognit,gocyclo,funlen // orchestration loop with retries and state handling
	if err := m.validate(); err != nil {
		return errors.WrapAndTrace(err)
	}

	sleepFn := m.Sleep
	if sleepFn == nil {
		sleepFn = defaultSleep
	}
	jitterFn := m.jitter
	if jitterFn == nil {
		jitterFn = addJitter
	}

	req := &brevapiv2.GetTunnelTokenRequest{
		BrevCloudNodeId: m.Identity.InstanceID,
		RequestedPorts: tunnelPortsToProto([]brevapiv2.TunnelPortMapping{
			{
				LocalPort: m.Cfg.SSHPort,
			},
		}),
		TunnelName: client.ProtoString(defaultTunnelName),
	}

	retryDelay := minRetryDelay

	for {
		if errors.Is(ctx.Err(), context.Canceled) {
			return nil
		}

		req.AppIngresses = nil

		token, err := m.requestTunnelToken(ctx, req)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			m.publishError("failed to fetch tunnel token", err)
			if err := m.delay(ctx, sleepFn, jitterFn(retryDelay)); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return err
			}
			retryDelay = nextDelay(retryDelay)
			continue
		}

		if err := m.configureCloudflaredService(ctx, token); err != nil {
			if errors.Is(err, context.Canceled) && errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}
			m.publishError("failed to configure cloudflared service", err)
			if err := m.delay(ctx, sleepFn, jitterFn(retryDelay)); err != nil {
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return err
			}
			retryDelay = nextDelay(retryDelay)
			continue
		}

		m.markActive("cloudflared service configured")
		return nil
	}
}

func (m *Manager) validate() error {
	if m.Client == nil {
		return errors.Errorf("client is required")
	}
	if m.Log == nil {
		return errors.Errorf("logger is required")
	}
	if m.Identity.DeviceToken == "" || m.Identity.InstanceID == "" {
		return errors.Errorf("identity is required")
	}
	if m.Cfg.SSHPort <= 0 {
		m.Cfg.SSHPort = 22
	}
	return nil
}

func (m *Manager) newCommand(ctx context.Context, name string, args ...string) Command {
	if m.CommandFactory != nil {
		return m.CommandFactory(ctx, name, args...)
	}
	return &execCmdWrapper{Cmd: exec.CommandContext(ctx, name, args...)}
}

type execCmdWrapper struct {
	*exec.Cmd
}

func (w *execCmdWrapper) SetEnv(env []string) {
	w.Env = env
}

func (w *execCmdWrapper) CombinedOutput() ([]byte, error) {
	out, err := w.Cmd.CombinedOutput()
	if err != nil {
		return out, errors.WrapAndTrace(err)
	}
	return out, nil
}

func (m *Manager) requestTunnelToken(ctx context.Context, req *brevapiv2.GetTunnelTokenRequest) (*brevapiv2.GetTunnelTokenResponse, error) {
	connectReq := connect.NewRequest(req)
	connectReq.Header().Set("Authorization", client.BearerToken(m.Identity.DeviceToken))

	resp, err := m.Client.GetTunnelToken(ctx, connectReq)
	if err != nil {
		return nil, errors.WrapAndTrace(client.ClassifyError(err))
	}
	return resp.Msg, nil
}

func (m *Manager) delay(ctx context.Context, sleepFn func(context.Context, time.Duration) error, delay time.Duration) error {
	return m.sleepWithBackoff(ctx, sleepFn, delay)
}

func (m *Manager) publishError(msg string, err error, fields ...zap.Field) {
	if err != nil {
		fields = append(fields, zap.Error(err))
		m.Log.Warn(msg, fields...)
		m.markError(fmt.Sprintf("%s: %v", msg, err))
		return
	}
	m.Log.Warn(msg, fields...)
	m.markError(msg)
}

func (m *Manager) markError(detail string) {
	if m.Health != nil {
		m.Health.MarkError(detail)
	}
}

func (m *Manager) markActive(detail string) {
	if m.Health != nil {
		m.Health.MarkActive(detail)
	}
}

func (m *Manager) configureCloudflaredService(ctx context.Context, token *brevapiv2.GetTunnelTokenResponse) error {
	commands := []struct {
		name string
		args []string
	}{
		{
			name: "sudo",
			args: []string{"cloudflared", "service", "install", token.GetToken()},
		},
	}

	for _, cmdSpec := range commands {
		if err := m.runCommand(ctx, cmdSpec.name, cmdSpec.args...); err != nil {
			return errors.WrapAndTrace(err)
		}
	}

	return nil
}

func (m *Manager) runCommand(ctx context.Context, name string, args ...string) error {
	cmd := m.newCommand(ctx, name, args...)
	cmd.SetEnv(os.Environ())
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.WrapAndTrace(fmt.Errorf("command %s %v failed: %w\n%s", name, args, err, string(out)))
	}
	return nil
}

func (m *Manager) sleepWithBackoff(ctx context.Context, sleepFn func(context.Context, time.Duration) error, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	if err := sleepFn(ctx, delay); err != nil {
		return errors.WrapAndTrace(err)
	}
	return nil
}

func nextDelay(current time.Duration) time.Duration {
	if current <= 0 {
		return minRetryDelay
	}
	next := current * 2
	if next > maxRetryDelay {
		return maxRetryDelay
	}
	return next
}

func addJitter(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}

	randRange := big.NewInt(401) // 0-400 inclusive
	n, err := cryptorand.Int(cryptorand.Reader, randRange)
	if err != nil {
		return delay
	}
	factor := 0.8 + float64(n.Int64())/1000.0
	return time.Duration(float64(delay) * factor)
}

func tunnelPortsToProto(ports []brevapiv2.TunnelPortMapping) []*brevapiv2.TunnelPortMapping {
	if len(ports) == 0 {
		return nil
	}
	out := make([]*brevapiv2.TunnelPortMapping, 0, len(ports))
	for _, port := range ports {
		lp := port.LocalPort
		rp := port.RemotePort
		if lp <= 0 && rp <= 0 {
			continue
		}
		if rp <= 0 {
			rp = lp
		}
		if lp <= 0 {
			lp = rp
		}
		if rp > math.MaxInt32 || lp > math.MaxInt32 || lp < math.MinInt32 || rp < math.MinInt32 {
			continue
		}
		out = append(out, &brevapiv2.TunnelPortMapping{
			LocalPort:  lp,
			RemotePort: rp,
			Protocol:   port.Protocol,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func defaultSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return errors.WrapAndTrace(ctx.Err())
	case <-timer.C:
		return nil
	}
}

func (m *Manager) detectHardwareFunc() func(context.Context) (telemetry.HardwareInfo, error) {
	if m.DetectHardware != nil {
		return m.DetectHardware
	}
	return telemetry.DetectHardware
}
