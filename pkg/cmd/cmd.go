// Package cmd is the entrypoint to cli
package cmd

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/cmd/approve"
	"github.com/brevdev/brev-cli/pkg/cmd/delete"
	"github.com/brevdev/brev-cli/pkg/cmd/healthcheck"
	"github.com/brevdev/brev-cli/pkg/cmd/importpkg"
	"github.com/brevdev/brev-cli/pkg/cmd/invite"
	"github.com/brevdev/brev-cli/pkg/cmd/login"
	"github.com/brevdev/brev-cli/pkg/cmd/logout"
	"github.com/brevdev/brev-cli/pkg/cmd/ls"
	"github.com/brevdev/brev-cli/pkg/cmd/meshd"
	"github.com/brevdev/brev-cli/pkg/cmd/open"
	"github.com/brevdev/brev-cli/pkg/cmd/portforward"
	"github.com/brevdev/brev-cli/pkg/cmd/profile"
	"github.com/brevdev/brev-cli/pkg/cmd/proxy"
	"github.com/brevdev/brev-cli/pkg/cmd/refresh"
	"github.com/brevdev/brev-cli/pkg/cmd/reset"
	"github.com/brevdev/brev-cli/pkg/cmd/runtasks"
	"github.com/brevdev/brev-cli/pkg/cmd/secret"
	"github.com/brevdev/brev-cli/pkg/cmd/set"
	"github.com/brevdev/brev-cli/pkg/cmd/sshkeys"
	"github.com/brevdev/brev-cli/pkg/cmd/start"
	"github.com/brevdev/brev-cli/pkg/cmd/stop"
	"github.com/brevdev/brev-cli/pkg/cmd/tasks"
	"github.com/brevdev/brev-cli/pkg/cmd/test"
	"github.com/brevdev/brev-cli/pkg/cmd/up"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/remoteversion"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

var user string

func NewDefaultBrevCommand() *cobra.Command {
	// cmd := NewBrevCommand(os.Stdin, os.Stdout, os.Stderr)
	cmd := NewBrevCommand()
	cmd.PersistentFlags().StringVar(&user, "user", "", "non root user to use for per user configuration of commands run as root")
	return cmd
}

func NewBrevCommand() *cobra.Command {
	// in io.Reader, out io.Writer, err io.Writer
	t := terminal.New()
	var printVersion bool

	conf := config.NewConstants()
	fs := files.AppFs
	authenticator := auth.Authenticator{
		Audience:           "https://brevdev.us.auth0.com/api/v2/",
		ClientID:           "JaqJRLEsdat5w7Tb0WqmTxzIeqwqepmk",
		DeviceCodeEndpoint: "https://brevdev.us.auth0.com/oauth/device/code",
		OauthTokenEndpoint: "https://brevdev.us.auth0.com/oauth/token",
	}
	// super annoying. this is needed to make the import stay
	_ = color.New(color.FgYellow, color.Bold).SprintFunc()

	fsStore := store.
		NewBasicStore().
		WithFileSystem(fs)
	loginAuth := auth.NewLoginAuth(fsStore, authenticator)
	noLoginAuth := auth.NewNoLoginAuth(fsStore, authenticator)

	loginCmdStore := fsStore.WithNoAuthHTTPClient(
		store.NewNoAuthHTTPClient(conf.GetBrevAPIURl()),
	).
		WithAuth(loginAuth)

	err := loginCmdStore.SetForbiddenStatusRetryHandler(func() error {
		_, err := loginAuth.GetAccessToken()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	if user != "" {
		loginCmdStore.WithUserID(user)
	}
	noLoginCmdStore := fsStore.WithNoAuthHTTPClient(
		store.NewNoAuthHTTPClient(conf.GetBrevAPIURl()),
	).
		WithAuth(noLoginAuth)
	if user != "" {
		noLoginCmdStore.WithUserID(user)
	}

	workspaceGroupID, err := fsStore.GetCurrentWorkspaceGroupID()
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	if workspaceGroupID != "" {
		loginCmdStore.WithStaticHeader("X-Workspace-Group-ID", workspaceGroupID)
		noLoginCmdStore.WithStaticHeader("X-Workspace-Group-ID", workspaceGroupID)
	}

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
				v, err := remoteversion.BuildVersionString(t, noLoginCmdStore)
				if err != nil {
					t.Errprint(err, "Failed to determine version")
					return breverrors.WrapAndTrace(err)
				}
				t.Vprint(v)
				return nil
			} else {
				err := cmd.Usage()
				if err != nil {
					return breverrors.WrapAndTrace(err)
				}
				return nil
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
	cobra.AddTemplateFunc("hasDebugCommands", hasDebugCommands)
	cobra.AddTemplateFunc("printCautiousMetaCmdMessage", printCautiousMetaCmdMessage)
	cobra.AddTemplateFunc("isHousekeepingCommand", isHousekeepingCommand)
	cobra.AddTemplateFunc("housekeepingCommands", housekeepingCommands)

	cmds.SetUsageTemplate(usageTemplate)

	cmds.PersistentFlags().BoolVar(&printVersion, "version", false, "Print version output")

	createCmdTree(cmds, t, loginCmdStore, noLoginCmdStore, loginAuth)

	return cmds
}

func createCmdTree(cmd *cobra.Command, t *terminal.Terminal, loginCmdStore *store.AuthHTTPStore, noLoginCmdStore *store.AuthHTTPStore, loginAuth *auth.LoginAuth) {
	cmd.AddCommand(set.NewCmdSet(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(ls.NewCmdLs(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(invite.NewCmdInvite(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(portforward.NewCmdPortForward(loginCmdStore, t))
	cmd.AddCommand(login.NewCmdLogin(t, noLoginCmdStore, loginAuth))
	cmd.AddCommand(logout.NewCmdLogout(loginAuth))

	// dev feature toggle
	if featureflag.IsDev() {
		_ = 0 // noop

		cmd.AddCommand(test.NewCmdTest(t, noLoginCmdStore))
		cmd.AddCommand(approve.NewCmdApprove(t, loginCmdStore))
		cmd.AddCommand(meshd.NewCmdMeshD(t, noLoginCmdStore))
		cmd.AddCommand(tasks.NewCmdTasks(t, noLoginCmdStore))
	} else {
		_ = 0 // noop
	}

	cmd.AddCommand(open.NewCmdOpen(t, loginCmdStore))
	cmd.AddCommand(secret.NewCmdSecret(loginCmdStore, t))
	cmd.AddCommand(sshkeys.NewCmdSSHKeys(t, loginCmdStore))
	cmd.AddCommand(start.NewCmdStart(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(importpkg.NewCmdImport(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(stop.NewCmdStop(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(delete.NewCmdDelete(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(reset.NewCmdReset(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(profile.NewCmdProfile(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(up.NewCmdJetbrains(loginCmdStore, t, true))
	cmd.AddCommand(refresh.NewCmdRefresh(t, loginCmdStore))
	cmd.AddCommand(runtasks.NewCmdRunTasks(t, noLoginCmdStore))
	cmd.AddCommand(proxy.NewCmdProxy(t, noLoginCmdStore))
	cmd.AddCommand(healthcheck.NewCmdHealthcheck(t, noLoginCmdStore))
}

func runHelp(cmd *cobra.Command, _ []string) {
	err := cmd.Help()
	if err != nil {
		panic(err)
	}
}

func hasHousekeepingCommands(cmd *cobra.Command) bool {
	return len(housekeepingCommands(cmd)) > 0
}

func hasDebugCommands(cmd *cobra.Command) bool {
	return len(debugsCommands(cmd)) > 0
}

func printCautiousMetaCmdMessage() string {
	yellow := color.New(color.FgYellow, color.Bold).SprintFunc()
	return yellow("(we're actively working on getting rid of these commands)")
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

func debugsCommands(cmd *cobra.Command) []*cobra.Command {
	cmds := []*cobra.Command{}
	for _, sub := range cmd.Commands() {
		if isDebugCommand(sub) {
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

func isDebugCommand(cmd *cobra.Command) bool {
	if _, ok := cmd.Annotations["debug"]; ok {
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

{{- if hasDebugCommands . }}

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

var (
	_ store.Auth     = auth.LoginAuth{}
	_ store.Auth     = auth.NoLoginAuth{}
	_ auth.AuthStore = store.FileStore{}
)
