package main

import (
	"os"

	"github.com/brevdev/brev-cli/pkg/cmd"
)

func main() {
	command := cmd.NewDefaultBrevCommand()

	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
