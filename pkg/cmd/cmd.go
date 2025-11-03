// Package cmd is the entrypoint to cli
package cmd

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/cmd/background"
	"github.com/brevdev/brev-cli/pkg/cmd/clipboard"
	"github.com/brevdev/brev-cli/pkg/cmd/configureenvvars"
	"github.com/brevdev/brev-cli/pkg/cmd/connect"
	"github.com/brevdev/brev-cli/pkg/cmd/copy"
	"github.com/brevdev/brev-cli/pkg/cmd/create"
	"github.com/brevdev/brev-cli/pkg/cmd/delete"
	"github.com/brevdev/brev-cli/pkg/cmd/envvars"
	"github.com/brevdev/brev-cli/pkg/cmd/fu"
	"github.com/brevdev/brev-cli/pkg/cmd/healthcheck"
	"github.com/brevdev/brev-cli/pkg/cmd/hello"
	"github.com/brevdev/brev-cli/pkg/cmd/importideconfig"
	"github.com/brevdev/brev-cli/pkg/cmd/initfile"
	"github.com/brevdev/brev-cli/pkg/cmd/invite"
	"github.com/brevdev/brev-cli/pkg/cmd/login"
	"github.com/brevdev/brev-cli/pkg/cmd/logout"
	"github.com/brevdev/brev-cli/pkg/cmd/ls"
	"github.com/brevdev/brev-cli/pkg/cmd/notebook"
	"github.com/brevdev/brev-cli/pkg/cmd/ollama"
	"github.com/brevdev/brev-cli/pkg/cmd/open"
	"github.com/brevdev/brev-cli/pkg/cmd/org"
	"github.com/brevdev/brev-cli/pkg/cmd/portforward"
	"github.com/brevdev/brev-cli/pkg/cmd/profile"
	"github.com/brevdev/brev-cli/pkg/cmd/proxy"
	"github.com/brevdev/brev-cli/pkg/cmd/recreate"
	"github.com/brevdev/brev-cli/pkg/cmd/redeem"
	"github.com/brevdev/brev-cli/pkg/cmd/refresh"
	"github.com/brevdev/brev-cli/pkg/cmd/reset"
	"github.com/brevdev/brev-cli/pkg/cmd/runtasks"
	"github.com/brevdev/brev-cli/pkg/cmd/scale"
	"github.com/brevdev/brev-cli/pkg/cmd/secret"
	"github.com/brevdev/brev-cli/pkg/cmd/set"
	"github.com/brevdev/brev-cli/pkg/cmd/setupworkspace"
	"github.com/brevdev/brev-cli/pkg/cmd/shell"
	"github.com/brevdev/brev-cli/pkg/cmd/sshkeys"
	"github.com/brevdev/brev-cli/pkg/cmd/start"
	"github.com/brevdev/brev-cli/pkg/cmd/status"
	"github.com/brevdev/brev-cli/pkg/cmd/stop"
	"github.com/brevdev/brev-cli/pkg/cmd/tasks"
	"github.com/brevdev/brev-cli/pkg/cmd/test"
	"github.com/brevdev/brev-cli/pkg/cmd/updatemodel"
	"github.com/brevdev/brev-cli/pkg/cmd/version"
	"github.com/brevdev/brev-cli/pkg/cmd/workspacegroups"
	"github.com/brevdev/brev-cli/pkg/cmd/writeconnectionevent"
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

var (
	userFlag      string
	printVersion  bool
	noCheckLatest bool
)

func NewDefaultBrevCommand() *cobra.Command {
	cmd := NewBrevCommand()

	// Define help flag before Cobra does with proper capitalization
	cmd.PersistentFlags().BoolP("help", "h", false, "Help for Brev")

	cmd.PersistentFlags().StringVar(&userFlag, "user", "", "Non root user to use for per user configuration of commands run as root")
	cmd.PersistentFlags().BoolVar(&printVersion, "version", false, "Print version output")
	cmd.PersistentFlags().BoolVar(&noCheckLatest, "no-check-latest", false, "Do not check for the latest version when printing version")

	return cmd
}

func NewBrevCommand() *cobra.Command { //nolint:funlen,gocognit,gocyclo // define brev command
	// in io.Reader, out io.Writer, err io.Writer
	t := terminal.New()

	conf := config.NewConstants()
	fs := files.AppFs

	fsStore := store.
		NewBasicStore().
		WithFileSystem(fs)

	tokens, _ := fsStore.GetAuthTokens()

	authenticator := auth.StandardLogin("", "", tokens)

	// super annoying. this is needed to make the import stay
	_ = color.New(color.FgYellow, color.Bold).SprintFunc()

	loginAuth := auth.NewLoginAuth(fsStore, authenticator)
	noLoginAuth := auth.NewNoLoginAuth(fsStore, authenticator)

	loginCmdStore := fsStore.WithNoAuthHTTPClient(
		store.NewNoAuthHTTPClient(conf.GetBrevAPIURl()),
	).
		WithAuth(loginAuth, store.WithDebug(conf.GetDebugHTTP()))

	err := loginCmdStore.SetForbiddenStatusRetryHandler(func() error {
		_, err1 := loginAuth.GetAccessToken()
		if err1 != nil {
			return breverrors.WrapAndTrace(err1)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	noAuthCmdStore := fsStore.WithNoAuthHTTPClient(
		store.NewNoAuthHTTPClient(conf.GetBrevAPIURl()),
	)
	noLoginCmdStore := noAuthCmdStore.WithAuth(noLoginAuth)

	workspaceGroupID, err := fsStore.GetCurrentWorkspaceGroupID()
	if err != nil {
		fmt.Printf("%v\n", err)
	}
	if workspaceGroupID != "" {
		loginCmdStore.WithStaticHeader("X-Workspace-Group-ID", workspaceGroupID)
		noLoginCmdStore.WithStaticHeader("X-Workspace-Group-ID", workspaceGroupID)
	}

	cmds := &cobra.Command{
		SilenceErrors: true,
		SilenceUsage:  true,
		Use:           "brev",
		Short:         "brev client for managing instances",
		Long: `
      brev client for managing instances

      Find more information at:
            https://brev.nvidia.com`,
		PostRun: func(cmd *cobra.Command, args []string) {
			shouldWe := hello.ShouldWeRunOnboarding(noLoginCmdStore)
			if shouldWe {
				user, err := loginCmdStore.GetCurrentUser()
				if err != nil {
					return
				}
				err = hello.CanWeOnboard(t, user, loginCmdStore)
				if err != nil {
					return
				}
			}
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			breverrors.GetDefaultErrorReporter().AddTag("command", cmd.Name())
			// version info gets in the way of the output for
			// configure-env-vars, since shells are going to eval it
			if featureflag.ShowVersionOnRun() && !printVersion && cmd.Name() != "configure-env-vars" {
				v, err := remoteversion.BuildCheckLatestVersionString(t, noLoginCmdStore)
				// todo this should not be fatal when it errors
				if err != nil {
					t.Errprint(err, "Failed to determine version")
					return breverrors.WrapAndTrace(err)
				}
				if v != "" {
					fmt.Println(v)
				}
			}
			if userFlag != "" {
				_, err := noLoginCmdStore.WithUserID(userFlag)
				if err != nil {
					return breverrors.WrapAndTrace(err)
				}
				_, err = loginCmdStore.WithUserID(userFlag)
				if err != nil {
					return breverrors.WrapAndTrace(err)
				}
				_, err = fsStore.WithUserID(userFlag)
				if err != nil {
					return breverrors.WrapAndTrace(err)
				}

			}
			home, err := fsStore.GetBrevHomePath()
			if err != nil {
				fmt.Printf("Warning: %v", err)
			}

			err = featureflag.LoadFeatureFlags(home)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if printVersion {
				if noCheckLatest {
					// Only print the current version, no network call
					t.Vprint(fmt.Sprintf("Current Version: %s", version.Version))
					return nil
				}
				v, err := remoteversion.BuildVersionString(t, noAuthCmdStore)
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
	cobra.AddTemplateFunc("hasWorkspaceCommands", hasWorkspaceCommands)
	cobra.AddTemplateFunc("workspaceCommands", workspaceCommands)
	cobra.AddTemplateFunc("hasAccessCommands", hasAccessCommands)
	cobra.AddTemplateFunc("accessCommands", accessCommands)
	cobra.AddTemplateFunc("hasOrganizationCommands", hasOrganizationCommands)
	cobra.AddTemplateFunc("organizationCommands", organizationCommands)
	cobra.AddTemplateFunc("hasConfigurationCommands", hasConfigurationCommands)
	cobra.AddTemplateFunc("configurationCommands", configurationCommands)
	cobra.AddTemplateFunc("hasQuickstartCommands", hasQuickstartCommands)
	cobra.AddTemplateFunc("quickstartCommands", quickstartCommands)
	cobra.AddTemplateFunc("hasDebugCommands", hasDebugCommands)
	cobra.AddTemplateFunc("debugCommands", debugCommands)

	cmds.SetUsageTemplate(usageTemplate)

	createCmdTree(cmds, t, loginCmdStore, noLoginCmdStore, loginAuth)

	return cmds
}

func createCmdTree(cmd *cobra.Command, t *terminal.Terminal, loginCmdStore *store.AuthHTTPStore, noLoginCmdStore *store.AuthHTTPStore, loginAuth *auth.LoginAuth) { //nolint:funlen // define brev command
	cmd.AddCommand(set.NewCmdSet(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(ls.NewCmdLs(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(org.NewCmdOrg(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(invite.NewCmdInvite(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(redeem.NewCmdRedeem(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(portforward.NewCmdPortForwardSSH(loginCmdStore, t))
	cmd.AddCommand(login.NewCmdLogin(t, noLoginCmdStore, loginAuth))
	cmd.AddCommand(logout.NewCmdLogout(loginAuth, noLoginCmdStore))
	cmd.AddCommand(tasks.NewCmdTasks(t, noLoginCmdStore))
	cmd.AddCommand(tasks.NewCmdConfigure(t, noLoginCmdStore))
	cmd.AddCommand(initfile.NewCmdInitFile(t, noLoginCmdStore))
	cmd.AddCommand(hello.NewCmdHello(t, noLoginCmdStore))
	cmd.AddCommand(notebook.NewCmdNotebook(noLoginCmdStore, t))
	// dev feature toggle
	if featureflag.IsDev() {
		_ = 0 // noop
		cmd.AddCommand(test.NewCmdTest(t, noLoginCmdStore))
		cmd.AddCommand(clipboard.EstablishConnection(t, loginCmdStore))
		cmd.AddCommand(clipboard.SendToClipboard(t, loginCmdStore))
		cmd.AddCommand(clipboard.ForwardPort(t, loginCmdStore))
		cmd.AddCommand(envvars.NewCmdEnvVars(t, loginCmdStore))
		cmd.AddCommand(connect.NewCmdConnect(t, noLoginCmdStore))
		cmd.AddCommand(fu.NewCmdFu(t, loginCmdStore, noLoginCmdStore))
	} else {
		_ = 0 // noop
	}
	cmd.AddCommand(workspacegroups.NewCmdWorkspaceGroups(t, loginCmdStore))
	cmd.AddCommand(scale.NewCmdScale(t, noLoginCmdStore))
	cmd.AddCommand(configureenvvars.NewCmdConfigureEnvVars(t, loginCmdStore))
	cmd.AddCommand(importideconfig.NewCmdImportIDEConfig(t, noLoginCmdStore))
	cmd.AddCommand(shell.NewCmdShell(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(copy.NewCmdCopy(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(open.NewCmdOpen(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(ollama.NewCmdOllama(t, loginCmdStore))
	cmd.AddCommand(background.NewCmdBackground(t, loginCmdStore))
	cmd.AddCommand(status.NewCmdStatus(t, loginCmdStore))
	cmd.AddCommand(secret.NewCmdSecret(loginCmdStore, t))
	cmd.AddCommand(sshkeys.NewCmdSSHKeys(t, loginCmdStore))
	cmd.AddCommand(start.NewCmdStart(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(create.NewCmdCreate(t, loginCmdStore))
	cmd.AddCommand(stop.NewCmdStop(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(delete.NewCmdDelete(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(reset.NewCmdReset(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(profile.NewCmdProfile(t, loginCmdStore, noLoginCmdStore))
	cmd.AddCommand(refresh.NewCmdRefresh(t, loginCmdStore))
	cmd.AddCommand(runtasks.NewCmdRunTasks(t, noLoginCmdStore))
	cmd.AddCommand(proxy.NewCmdProxy(t, noLoginCmdStore))
	cmd.AddCommand(healthcheck.NewCmdHealthcheck(t, noLoginCmdStore))

	cmd.AddCommand(setupworkspace.NewCmdSetupWorkspace(noLoginCmdStore))
	cmd.AddCommand(recreate.NewCmdRecreate(t, loginCmdStore))
	cmd.AddCommand(writeconnectionevent.NewCmdwriteConnectionEvent(t, loginCmdStore))
	cmd.AddCommand(updatemodel.NewCmdupdatemodel(t, loginCmdStore))
}

func hasWorkspaceCommands(cmd *cobra.Command) bool {
	return len(workspaceCommands(cmd)) > 0
}

func hasAccessCommands(cmd *cobra.Command) bool {
	return len(accessCommands(cmd)) > 0
}

func hasOrganizationCommands(cmd *cobra.Command) bool {
	return len(organizationCommands(cmd)) > 0
}

func hasConfigurationCommands(cmd *cobra.Command) bool {
	return len(configurationCommands(cmd)) > 0
}

func hasQuickstartCommands(cmd *cobra.Command) bool {
	return len(quickstartCommands(cmd)) > 0
}

func hasDebugCommands(cmd *cobra.Command) bool {
	return len(debugCommands(cmd)) > 0
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

func accessCommands(cmd *cobra.Command) []*cobra.Command {
	cmds := []*cobra.Command{}
	for _, sub := range cmd.Commands() {
		if isAccessCommand(sub) {
			cmds = append(cmds, sub)
		}
	}
	return cmds
}

func organizationCommands(cmd *cobra.Command) []*cobra.Command {
	cmds := []*cobra.Command{}
	for _, sub := range cmd.Commands() {
		if isOrganizationCommand(sub) {
			cmds = append(cmds, sub)
		}
	}
	return cmds
}

func configurationCommands(cmd *cobra.Command) []*cobra.Command {
	cmds := []*cobra.Command{}
	for _, sub := range cmd.Commands() {
		if isConfigurationCommand(sub) {
			cmds = append(cmds, sub)
		}
	}
	return cmds
}

func quickstartCommands(cmd *cobra.Command) []*cobra.Command {
	cmds := []*cobra.Command{}
	for _, sub := range cmd.Commands() {
		if isQuickstartCommand(sub) {
			cmds = append(cmds, sub)
		}
	}
	return cmds
}

func debugCommands(cmd *cobra.Command) []*cobra.Command {
	cmds := []*cobra.Command{}
	for _, sub := range cmd.Commands() {
		if isDebugCommand(sub) {
			cmds = append(cmds, sub)
		}
	}
	return cmds
}

func isWorkspaceCommand(cmd *cobra.Command) bool {
	_, ok := cmd.Annotations["workspace"]
	return ok
}

func isAccessCommand(cmd *cobra.Command) bool {
	_, ok := cmd.Annotations["access"]
	return ok
}

func isOrganizationCommand(cmd *cobra.Command) bool {
	_, ok := cmd.Annotations["organization"]
	return ok
}

func isConfigurationCommand(cmd *cobra.Command) bool {
	_, ok := cmd.Annotations["configuration"]
	return ok
}

func isQuickstartCommand(cmd *cobra.Command) bool {
	_, ok := cmd.Annotations["quickstart"]
	return ok
}

func isDebugCommand(cmd *cobra.Command) bool {
	_, ok := cmd.Annotations["debug"]
	return ok
}

var usageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

{{- if hasWorkspaceCommands . }}

Instance Commands:
{{- range workspaceCommands . }}
  {{rpad .Name .NamePadding }} {{.Short}}
{{- end}}{{- end}}

{{- if hasAccessCommands . }}

Instance Access:
{{- range accessCommands . }}
  {{rpad .Name .NamePadding }} {{.Short}}
{{- end}}{{- end}}

{{- if hasOrganizationCommands . }}

Organization Management:
{{- range organizationCommands . }}
  {{rpad .Name .NamePadding }} {{.Short}}
{{- end}}{{- end}}

{{- if hasConfigurationCommands . }}

Configuration:
{{- range configurationCommands . }}
  {{rpad .Name .NamePadding }} {{.Short}}
{{- end}}{{- end}}

{{- if hasQuickstartCommands . }}

Quick Start:
{{- range quickstartCommands . }}
  {{rpad .Name .NamePadding }} {{.Short}}
{{- end}}{{- end}}

{{- if hasDebugCommands . }}

Debug Commands:
{{- range debugCommands . }}
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
