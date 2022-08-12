#!/bin/sh

cat <<EOF > pkg/cmd/$1/$1.go
package $1

import (
	"github.com/spf13/cobra"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type $1Store interface {}

func NewCmd$1(t *terminal.Terminal, store $1Store) *cobra.Command {
    cmd := &cobra.Command{
        Use:                   "$1",
        DisableFlagsInUseLine: true,
        Short:                 "TODO",
        Long:                  "TODO",
        Example:               "TODO",
        RunE: func(cmd *cobra.Command, args []string) error {
            err := Run$1(t, args, store)
            if err != nil {
                return breverrors.WrapAndTrace(err)
            }
            return nil
        },
    }
    return cmd
}

func Run$1(t *terminal.Terminal, args []string, store $1Store) error {
    return nil
}
EOF
