package workspacegroups

import (
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type WorkspaceGroupsStore interface {
	GetWorkspaceGroups(organizationID string) ([]entity.WorkspaceGroup, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
}

func NewCmdWorkspaceGroups(t *terminal.Terminal, store WorkspaceGroupsStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "workspacegroups",
		DisableFlagsInUseLine: true,
		Short:                 "TODO",
		Long:                  "TODO",
		Example:               "TODO",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunWorkspaceGroups(t, args, store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	return cmd
}

func RunWorkspaceGroups(_ *terminal.Terminal, _ []string, store WorkspaceGroupsStore) error {
	org, err := store.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	wsgs, err := store.GetWorkspaceGroups(org.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	ta := table.NewWriter()
	ta.SetOutputMirror(os.Stdout)
	ta.Style().Options = getBrevTableOptions()
	header := table.Row{"NAME", "PLATFORM ID", "PLATFORM TYPE"}
	ta.AppendHeader(header)
	for _, w := range wsgs {
		workspaceRow := []table.Row{{
			w.Name, w.PlatformID, w.Platform,
		}}
		ta.AppendRows(workspaceRow)
	}
	ta.Render()
	return nil
}

func getBrevTableOptions() table.Options {
	options := table.OptionsDefault
	options.DrawBorder = false
	options.SeparateColumns = false
	options.SeparateRows = false
	options.SeparateHeader = false
	return options
}
