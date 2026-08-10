package main

import (
	"context"
	"fmt"
	"os"

	"github.com/brevdev/brev-cli/pkg/analytics"
	"github.com/brevdev/brev-cli/pkg/cmd"
	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/disablessh"
	"github.com/brevdev/brev-cli/pkg/errors"
)

func main() {
	handled, err := disablessh.RunLocalKeyCleanupHelper(context.Background(), os.Args[1:], os.Stdout)
	if handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	done := errors.GetDefaultErrorReporter().Setup()
	defer done()
	defer analytics.Close()
	command := cmd.NewDefaultBrevCommand()

	if err := command.Execute(); err != nil {
		analytics.CaptureCommandError()
		cmderrors.DisplayAndHandleError(err)
		done()
		os.Exit(1) //nolint:gocritic // manually call done
	}
}
