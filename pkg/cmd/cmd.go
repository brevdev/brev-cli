// Package cmd is the entrypoint to cli
package cmd

import (
	"io"
	"os"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/clone"
	"github.com/brevdev/brev-cli/pkg/cmd/configure"
	"github.com/brevdev/brev-cli/pkg/cmd/delete"
	"github.com/brevdev/brev-cli/pkg/cmd/link"
	"github.com/brevdev/brev-cli/pkg/cmd/login"
	"github.com/brevdev/brev-cli/pkg/cmd/logout"
	"github.com/brevdev/brev-cli/pkg/cmd/ls"
	"github.com/brevdev/brev-cli/pkg/cmd/refresh"
	"github.com/brevdev/brev-cli/pkg/cmd/set"
	"github.com/brevdev/brev-cli/pkg/cmd/sshall"
	"github.com/brevdev/brev-cli/pkg/cmd/start"
	"github.com/brevdev/brev-cli/pkg/cmd/stop"
	"github.com/brevdev/brev-cli/pkg/cmd/version"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

func NewDefaultBrevCommand() *cobra.Command {
	cmd := NewBrevCommand(os.Stdin, os.Stdout, os.Stderr)
	return cmd
}

func NewBrevCommand(in io.Reader, out io.Writer, err io.Writer) *cobra.Command {
	t := terminal.New()
	var printVersion bool

	cmds := &cobra.Command{
		Use:   "brev",
		Short: "brev client for managing workspaces",
		Long: `
      brev client for managing workspaces

      Find more information at:
            https://brev.dev`,
		Run: runHelp,
		RunE: func(cmd *cobra.Command, args []string) error {
			if printVersion {
				v, err := version.BuildVersionString(t)
				if err != nil {
					t.Errprint(err, "Failed to determine version")
					return err
				}
				t.Vprint(v)
				return nil
			} else {
				return cmd.Usage()
			}
		},
	}
	cobra.AddTemplateFunc("hasContextCommands", hasContextCommands)
	cobra.AddTemplateFunc("isContextCommand", isContextCommand)
	cobra.AddTemplateFunc("contextCommands", contextCommands)
	cobra.AddTemplateFunc("hasSSHCommands", hasSSHCommands)
	cobra.AddTemplateFunc("isSSHCommand", isSSHCommand)
	cobra.AddTemplateFunc("sshCommands", sshCommands)
	cobra.AddTemplateFunc("hasWorkspaceCommands", hasWorkspaceCommands)
	cobra.AddTemplateFunc("isWorkspaceCommand", isWorkspaceCommand)
	cobra.AddTemplateFunc("workspaceCommands", workspaceCommands)
	cobra.AddTemplateFunc("hasHousekeepingCommands", hasHousekeepingCommands)
	cobra.AddTemplateFunc("isHousekeepingCommand", isHousekeepingCommand)
	cobra.AddTemplateFunc("housekeepingCommands", housekeepingCommands)

	cmds.SetUsageTemplate(usageTemplate)

	cmds.PersistentFlags().BoolVar(&printVersion, "version", false, "Print version output")

	createCmdTree(cmds, t)

	return cmds
}

func createCmdTree(cmd *cobra.Command, t *terminal.Terminal) {
	cmd.AddCommand(clone.NewCmdClone(t))
	cmd.AddCommand(set.NewCmdSet(t))
	cmd.AddCommand(ls.NewCmdLs(t))
	cmd.AddCommand(link.NewCmdLink(t))
	cmd.AddCommand(login.NewCmdLogin())
	cmd.AddCommand(logout.NewCmdLogout())
	cmd.AddCommand(refresh.NewCmdRefresh(t))

	// dev feature toggle
	if isDev() {
		cmd.AddCommand(sshall.NewCmdSSHAll())
	}
	cmd.AddCommand(configure.NewCmdConfigure())

	cmd.AddCommand(start.NewCmdStart(t))
	cmd.AddCommand(stop.NewCmdStop(t))
	cmd.AddCommand(delete.NewCmdDelete(t))
}

func runHelp(cmd *cobra.Command, _ []string) {
	err := cmd.Help()
	if err != nil {
		panic(err)
	}
}

func isDev() bool {
	return version.Version == "" || strings.HasPrefix(version.Version, "dev")
}

func hasHousekeepingCommands(cmd *cobra.Command) bool {
	return len(housekeepingCommands(cmd)) > 0
}

func hasSSHCommands(cmd *cobra.Command) bool {
	return len(sshCommands(cmd)) > 0
}

func hasWorkspaceCommands(cmd *cobra.Command) bool {
	return len(workspaceCommands(cmd)) > 0
}

func hasContextCommands(cmd *cobra.Command) bool {
	return len(contextCommands(cmd)) > 0
}

func housekeepingCommands(cmd *cobra.Command) []*cobra.Command {
	cmds := []*cobra.Command{}
	for _, sub := range cmd.Commands() {
		if isHousekeepingCommand(sub) {
			cmds = append(cmds, sub)
		}
	}
	return cmds
}

func sshCommands(cmd *cobra.Command) []*cobra.Command {
	cmds := []*cobra.Command{}
	for _, sub := range cmd.Commands() {
		if isSSHCommand(sub) {
			cmds = append(cmds, sub)
		}
	}
	return cmds
}

func workspaceCommands(cmd *cobra.Command) []*cobra.Command {
	cmds := []*cobra.Command{}
	for _, sub := range cmd.Commands() {
		if isWorkspaceCommand(sub) {
			cmds = append(cmds, sub)
		}
	}
	return cmds
}

func contextCommands(cmd *cobra.Command) []*cobra.Command {
	cmds := []*cobra.Command{}
	for _, sub := range cmd.Commands() {
		if isContextCommand(sub) {
			cmds = append(cmds, sub)
		}
	}
	return cmds
}

func isHousekeepingCommand(cmd *cobra.Command) bool {
	if _, ok := cmd.Annotations["housekeeping"]; ok {
		return true
	} else {
		return false
	}
}

func isSSHCommand(cmd *cobra.Command) bool {
	if _, ok := cmd.Annotations["ssh"]; ok {
		return true
	} else {
		return false
	}
}

func isWorkspaceCommand(cmd *cobra.Command) bool {
	if _, ok := cmd.Annotations["workspace"]; ok {
		return true
	} else {
		return false
	}
}

func isContextCommand(cmd *cobra.Command) bool {
	if _, ok := cmd.Annotations["context"]; ok {
		return true
	} else {
		return false
	}
}

var usageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

{{- if hasWorkspaceCommands . }}

Workspace Commands:
{{- range workspaceCommands . }}
  {{rpad .Name .NamePadding }} {{.Short}}
{{- end}}{{- end}}

{{- if hasSSHCommands . }}

{{- if hasContextCommands . }}

Context Commands:
{{- range contextCommands . }}
  {{rpad .Name .NamePadding }} {{.Short}}
{{- end}}{{- end}}

SSH Commands:
{{- range sshCommands . }}
  {{rpad .Name .NamePadding }} {{.Short}}
{{- end}}{{- end}}

{{- if hasHousekeepingCommands . }}

Housekeeping Commands:
{{- range housekeepingCommands . }}
  {{rpad .Name .NamePadding }} {{.Short}}
{{- end}}{{- end}}

{{- end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`
