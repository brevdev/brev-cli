package main

import (
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
		os.Exit(1)
	}
}
