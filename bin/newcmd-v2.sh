#!/bin/sh

cat <<EOF > pkg/cmd/$1/$1.go
package $1

import (
	"github.com/spf13/cobra"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

var (
	short   = "TODO"
	long    = "TODO"
	example = "TODO"
)

func NewCmd$1(t *terminal.Terminal, store $1Store) *cobra.Command {
    cmd := &cobra.Command{
        Use:                   "$1",
        DisableFlagsInUseLine: true,
        Short:                 short,
        Long:                  long,
        Example:               example,
        RunE: $1{
                t: t, 
                store: store,
            }.RunE,
    }
    return cmd
}

type $1Store interface {}

type $1 struct {
	t     *terminal.Terminal
	store $1Store
}

func ($1 $1) RunE(cmd *cobra.Command, args []string) error {
    return nil
}
EOF
