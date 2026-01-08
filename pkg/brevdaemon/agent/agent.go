package agent

import (
	"context"
	"sync"
	"time"

	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/client"
	agentconfig "github.com/brevdev/brev-cli/pkg/brevdaemon/agent/config"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/health"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/heartbeat"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/identity"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/telemetry"
	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent/tunnel"
	"github.com/brevdev/brev-cli/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Agent is the top-level interface that drives the agent lifecycle.
type Agent interface {
	Run(ctx context.Context) error
}

type runner interface {
	Run(ctx context.Context) error
}

type tunnelProcess interface {
	Start(ctx context.Context) error
}

type agent struct {
	cfg       agentconfig.Config
	log       *zap.Logger
	heartbeat runner
	tunnel    tunnelProcess

	statusReporter *health.Reporter
	statusUpdates  chan client.HeartbeatStatus
}

var (
	newBrevCloudAgentClient = client.New
	detectHardware          = telemetry.DetectHardware
	ensureIdentity          = identity.EnsureIdentity
)

const defaultHeartbeatMaxInterval = 5 * time.Minute

// NewAgent wires the agent components together.
func NewAgent(cfg agentconfig.Config, log *zap.Logger) (Agent, error) {
	if log == nil {
		return nil, errors.Errorf("logger cannot be nil")
	}

	cli, err := newBrevCloudAgentClient(cfg)
	if err != nil {
		return nil, errors.WrapAndTrace(err)
	}

	ctx := context.Background()
	hw, err := detectHardware(ctx)
	if err != nil {
		return nil, errors.WrapAndTrace(err)
	}

	store := identity.NewIdentityStore(cfg)
	ident, err := ensureIdentity(ctx, cfg, cli, store, hw, log.Named("identity"))
	if err != nil {
		return nil, errors.WrapAndTrace(err)
	}

	statusReporter := health.NewReporter(health.Status{
		Phase:              client.NodePhaseActive,
		LastTransitionTime: time.Now(),
	})
	defaultStatus := client.HeartbeatStatus{
		Phase: client.NodePhaseActive,
	}
	statusUpdates := make(chan client.HeartbeatStatus, 1)

	hbRunner := &heartbeat.Runner{
		Client:   cli,
		Identity: ident,
		Cfg: heartbeat.HeartbeatConfig{
			BaseInterval: cfg.HeartbeatInterval,
			MaxInterval:  defaultHeartbeatMaxInterval,
		},
		Log:           log.Named("heartbeat"),
		DefaultStatus: &defaultStatus,
		StatusUpdates: statusUpdates,
	}

	var tunnelMgr tunnelProcess
	if cfg.EnableTunnel {
		tunnelMgr = &tunnel.Manager{
			Client:   cli,
			Identity: ident,
			Cfg: tunnel.TunnelConfig{
				SSHPort: cfg.TunnelSSHPort,
			},
			Log:    log.Named("tunnel"),
			Health: statusReporter,
		}
	}

	return &agent{
		cfg:            cfg,
		log:            log.Named("agent"),
		heartbeat:      hbRunner,
		tunnel:         tunnelMgr,
		statusReporter: statusReporter,
		statusUpdates:  statusUpdates,
	}, nil
}

func (a *agent) Run(ctx context.Context) error {
	a.log.Info("brev-agent starting")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	g, gctx := errgroup.WithContext(ctx)

	stopStatusBridge := a.startStatusBridge(gctx)
	defer stopStatusBridge()

	g.Go(func() error {
		if err := a.heartbeat.Run(gctx); err != nil {
			return errors.WrapAndTrace(err)
		}
		return nil
	})

	var tunnelWG sync.WaitGroup
	if a.tunnel != nil {
		tunnelWG.Add(1)
		go func() {
			defer tunnelWG.Done()
			a.runTunnel(gctx)
		}()
	}

	err := g.Wait()
	cancel()
	tunnelWG.Wait()

	if err != nil {
		a.log.Error("brev-agent stopped with error", zap.Error(err))
		return errors.WrapAndTrace(err)
	}

	a.log.Info("brev-agent stopped cleanly")
	return nil
}

func (a *agent) startStatusBridge(ctx context.Context) func() {
	if a.statusReporter == nil || a.statusUpdates == nil {
		return func() {}
	}

	bridgeCtx, cancel := context.WithCancel(ctx)
	updates := a.statusReporter.Updates()

	go func() {
		for {
			select {
			case <-bridgeCtx.Done():
				return
			case status, ok := <-updates:
				if !ok {
					return
				}
				hbStatus := toHeartbeatStatus(status)
				select {
				case a.statusUpdates <- hbStatus:
				case <-bridgeCtx.Done():
					return
				}
			}
		}
	}()

	return cancel
}

func toHeartbeatStatus(status health.Status) client.HeartbeatStatus {
	hbStatus := client.HeartbeatStatus{
		Phase:  status.Phase,
		Detail: status.Detail,
	}
	if !status.LastTransitionTime.IsZero() {
		t := status.LastTransitionTime
		hbStatus.LastTransitionTime = &t
	}
	return hbStatus
}

func (a *agent) runTunnel(ctx context.Context) {
	if a.tunnel == nil {
		return
	}

	for {
		err := a.tunnel.Start(ctx)
		switch {
		case err == nil:
			return
		case errors.Is(err, context.Canceled):
			return
		default:
			a.log.Warn("tunnel subsystem failed", zap.Error(err))
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}
