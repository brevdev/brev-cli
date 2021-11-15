package cmdcontext

import (
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/spf13/cobra"
)

// InvokeParentPersistentPreRun executes the immediate parent command's
// PersistentPreRunE and PersistentPreRun functions, in that order. If
// an error is returned from PersistentPreRunE, it is immediately returned.
//
// TODO: reverse walk up command tree? would need to ensure no one parent is invoked multiple times.
func InvokeParentPersistentPreRun(cmd *cobra.Command, args []string) error {
	parentCmd := cmd.Parent()
	if parentCmd == nil {
		return nil
	}

	var err error

	// Invoke PersistentPreRunE, returning an error if one occurs
	// If no error is returned, proceed with PersistentPreRun
	parentPersistentPreRunE := parentCmd.PersistentPreRunE
	if parentPersistentPreRunE != nil {
		err = parentPersistentPreRunE(parentCmd, args)
	}
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Invoke PersistentPreRun
	parentPersistentPreRun := parentCmd.PersistentPreRun
	if parentPersistentPreRun != nil {
		parentPersistentPreRun(parentCmd, args)
	}

	return nil
}
