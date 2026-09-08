package main

import (
	stderrors "errors"
	"os"

	"github.com/brevdev/brev-cli/pkg/analytics"
	"github.com/brevdev/brev-cli/pkg/cmd"
	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/exec"
	"github.com/brevdev/brev-cli/pkg/errors"
)

func main() {
	done := errors.GetDefaultErrorReporter().Setup()
	defer done()
	defer analytics.Close()
	command := cmd.NewDefaultBrevCommand()

	if err := command.Execute(); err != nil {
		// A remote command exiting non-zero is not a CLI error: pass its exit
		// code through so callers can branch on it, and print nothing extra.
		var remoteErr exec.RemoteExitError
		if stderrors.As(err, &remoteErr) {
			done()
			os.Exit(remoteErr.Code) //nolint:gocritic // manually call done
		}
		analytics.CaptureCommandError()
		cmderrors.DisplayAndHandleError(err)
		done()
		os.Exit(1) //nolint:gocritic // manually call done
	}
}
