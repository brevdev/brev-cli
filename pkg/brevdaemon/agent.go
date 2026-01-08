package brevdaemon

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/brevdev/brev-cli/pkg/brevdaemon/agent"
	agentconfig "github.com/brevdev/brev-cli/pkg/brevdaemon/agent/config"
	"github.com/brevdev/brev-cli/pkg/errors"
	"go.uber.org/zap"
)

const (
	exitCodeOK     = 0
	exitCodeConfig = 2
	exitCodeError  = 3
)

func main() {
	os.Exit(runMain())
}

func runMain() int {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	code, runErr := run(ctx)
	if runErr != nil {
		zap.L().Error("brev-agent exited with error", zap.Error(runErr))
	} else {
		zap.L().Info("brev-agent exited cleanly")
	}
	return code
}

func run(ctx context.Context) (int, error) {
	cfg, err := agentconfig.Load()
	if err != nil {
		return exitCodeConfig, errors.WrapAndTrace(err)
	}

	agentLogger := zap.L().Named("brev-agent")
	a, err := agent.NewAgent(cfg, agentLogger)
	if err != nil {
		return exitCodeError, errors.WrapAndTrace(err)
	}

	if err := a.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return exitCodeOK, nil
		}
		return exitCodeError, errors.WrapAndTrace(err)
	}

	return exitCodeOK, nil
}
