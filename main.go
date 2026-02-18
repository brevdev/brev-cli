package main

import (
	stderrors "errors"
	"os"

	"github.com/brevdev/brev-cli/pkg/cmd"
	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/errors"
)

func main() {
	done := errors.GetDefaultErrorReporter().Setup()
	defer done()
	command := cmd.NewDefaultBrevCommand()

	if err := command.Execute(); err != nil {
		cmderrors.DisplayAndHandleError(err)
		done()
		exitCode := 1
		var exitErr errors.ExitCodeError
		if stderrors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode
		}
		os.Exit(exitCode) //nolint:gocritic // manually call done
	}
}
