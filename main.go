package main

import (
	stderrors "errors"
	"os"

	"github.com/brevdev/brev-cli/pkg/analytics"
	"github.com/brevdev/brev-cli/pkg/cmd"
	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/errors"
)

func main() {
	done := errors.GetDefaultErrorReporter().Setup()
	defer done()
	defer analytics.Close()
	command := cmd.NewDefaultBrevCommand()

	if err := command.Execute(); err != nil {
		// Not a CLI error: pass the remote command's exit code straight through.
		var remoteErr errors.RemoteExitError
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
