package secrets

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type SecretStore interface {
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	CreateManagedSecret(organizationID, name, value string) (*nodev1.ManagedSecret, error)
	ListManagedSecrets(organizationID string) ([]*nodev1.ManagedSecret, error)
	GetManagedSecret(secretID string) (*nodev1.ManagedSecret, error)
	GetManagedSecretByName(organizationID, name string) (*nodev1.ManagedSecret, error)
	ListManagedSecretVersions(secretID string) ([]*nodev1.ManagedSecretVersion, error)
	GetManagedSecretValue(secretID, versionID string) (string, error)
	SetManagedSecretValue(secretID, value string) (*nodev1.ManagedSecretVersion, error)
	DeleteManagedSecret(secretID string) error
}

func NewCmdSecrets(t *terminal.Terminal, store SecretStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "secrets",
		DisableFlagsInUseLine: true,
		Short:                 "Manage organization secrets (versioned, like AWS SSM parameters)",
		Long: `Manage versioned secrets for your organization.

Each write to a secret creates a new immutable version, so previous values
remain retrievable by version number.`,
		Example: `
  brev secrets ls
  brev secrets create --name my-secret --value supersecret
  brev secrets get --id <ID>
  brev secrets get --name my-secret
  brev secrets set-value --id <ID> --value rotated-secret
  brev secrets get-value --id <ID>
  brev secrets get-value --id <ID> --version 2
  brev secrets delete --id <ID>
		`,
		Args: cmderrors.TransformToValidationError(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newCmdSecretLs(t, store))
	cmd.AddCommand(newCmdSecretCreate(t, store))
	cmd.AddCommand(newCmdSecretSetValue(t, store))
	cmd.AddCommand(newCmdSecretGet(t, store))
	cmd.AddCommand(newCmdSecretGetValue(t, store))
	cmd.AddCommand(newCmdSecretDelete(t, store))

	// Use default cobra template to show all subcommands
	cmd.SetUsageTemplate(`Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`)

	return cmd
}

func newCmdSecretLs(t *terminal.Terminal, store SecretStore) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List secrets for your organization",
		Args:  cmderrors.TransformToValidationError(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			orgID, err := getOrgID(store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			secrets, err := store.ListManagedSecrets(orgID)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			if len(secrets) == 0 {
				t.Vprint(t.Yellow("No secrets found. Create one with 'brev secrets create --name <NAME> --value <VALUE>'"))
				return nil
			}

			ta := table.NewWriter()
			ta.SetOutputMirror(os.Stdout)
			ta.Style().Options = getSecretTableOptions()
			ta.AppendHeader(table.Row{"ID", "NAME", "CREATED"})
			for _, s := range secrets {
				ta.AppendRow(table.Row{s.GetSecretId(), s.GetName(), formatTimestamp(s.GetCreateTime())})
			}
			ta.Render()
			return nil
		},
	}
}

func newCmdSecretCreate(t *terminal.Terminal, store SecretStore) *cobra.Command {
	var name string
	var value string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new secret with an initial value",
		Long: `Create a new secret for your organization.

The value can be passed with --value or piped via stdin:
  echo -n "supersecret" | brev secrets create --name my-secret`,
		Args: cmderrors.TransformToValidationError(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return breverrors.NewValidationError("secret name required: pass --name")
			}
			val, err := readSecretValue(value)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			orgID, err := getOrgID(store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			secret, err := store.CreateManagedSecret(orgID, name, val)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			t.Vprintf("Created secret \"%s\" (version %d)\n", secret.GetName(), secret.GetLatestVersionNumber())
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "secret name")
	cmd.Flags().StringVar(&value, "value", "", "secret value (reads from stdin if omitted)")
	return cmd
}

func newCmdSecretSetValue(t *terminal.Terminal, store SecretStore) *cobra.Command {
	var id string
	var name string
	var value string
	cmd := &cobra.Command{
		Use:   "set-value",
		Short: "Set a secret's value, creating a new version",
		Long: `Set a new value for an existing secret, looked up by --id or --name.
The previous value is preserved as an older version and can be retrieved
with 'brev secrets get-value --id <ID> --version <VERSION>'.

The value can be passed with --value or piped via stdin:
  echo -n "rotated-secret" | brev secrets set --id <ID>`,
		Args: cmderrors.TransformToValidationError(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			val, err := readSecretValue(value)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			secret, err := resolveSecret(store, id, name)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			version, err := store.SetManagedSecretValue(secret.GetSecretId(), val)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			t.Vprintf("Updated secret \"%s\" (version %d)\n", secret.GetName(), version.GetVersionNumber())
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "secret ID")
	cmd.Flags().StringVar(&name, "name", "", "look up the secret by name instead of ID")
	cmd.Flags().StringVar(&value, "value", "", "secret value (reads from stdin if omitted)")
	return cmd
}

func newCmdSecretGet(t *terminal.Terminal, store SecretStore) *cobra.Command {
	var id string
	var name string
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a secret's metadata and versions",
		Long: `Get a secret's name, ID, creation time, and version list, looked up by
--id or --name. Use 'brev secrets get-value' to retrieve the value.`,
		Args: cmderrors.TransformToValidationError(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			secret, err := resolveSecret(store, id, name)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			versions, err := store.ListManagedSecretVersions(secret.GetSecretId())
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			t.Vprintf("NAME:     %s\n", secret.GetName())
			t.Vprintf("ID:       %s\n", secret.GetSecretId())
			t.Vprintf("CREATED:  %s\n", formatTimestamp(secret.GetCreateTime()))

			if len(versions) == 0 {
				return nil
			}
			t.Vprint("")
			t.Vprint(t.White("VERSIONS"))
			ta := table.NewWriter()
			ta.SetOutputMirror(os.Stdout)
			ta.Style().Options = getSecretTableOptions()
			ta.AppendHeader(table.Row{"ID", "VERSION", "CREATED"})
			for _, v := range versions {
				ta.AppendRow(table.Row{v.GetVersionId(), v.GetVersionNumber(), formatTimestamp(v.GetCreateTime())})
			}
			ta.Render()
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "secret ID")
	cmd.Flags().StringVar(&name, "name", "", "look up the secret by name instead of ID")
	return cmd
}

func newCmdSecretGetValue(t *terminal.Terminal, store SecretStore) *cobra.Command {
	var id string
	var name string
	var version string
	cmd := &cobra.Command{
		Use:   "get-value",
		Short: "Get a secret's value as-is (no trailing newline)",
		Long: `Get a secret's value, looked up by --id or --name. Returns the latest
version by default, or a specific version with --version (a version number
or a version ID). The value is written as-is, without a trailing newline.`,
		Args: cmderrors.TransformToValidationError(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			secret, err := resolveSecret(store, id, name)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			versionID := ""
			if version != "" {
				if versionNumber, parseErr := strconv.ParseInt(version, 10, 64); parseErr == nil {
					versionID, err = findVersionID(store, secret.GetSecretId(), versionNumber)
				} else {
					versionID = version
				}
			} else {
				versionID, err = findLatestVersionID(store, secret.GetSecretId())
			}
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			value, err := store.GetManagedSecretValue(secret.GetSecretId(), versionID)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			fmt.Print(value)
			return nil
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "secret version number or version ID (latest if omitted)")
	cmd.Flags().StringVar(&id, "id", "", "secret ID")
	cmd.Flags().StringVar(&name, "name", "", "look up the secret by name instead of ID")
	return cmd
}

func newCmdSecretDelete(t *terminal.Terminal, store SecretStore) *cobra.Command {
	var id string
	var name string
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a secret and all its versions",
		Long:  "Delete a secret and all its versions, looked up by --id or --name.",
		Args:  cmderrors.TransformToValidationError(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			secret, err := resolveSecret(store, id, name)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			err = store.DeleteManagedSecret(secret.GetSecretId())
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			t.Vprintf("Deleted secret %s and all its versions\n", secret.GetName())
			return nil
		},
	}
	cmd.Flags().StringVar(&id, "id", "", "secret ID")
	cmd.Flags().StringVar(&name, "name", "", "look up the secret by name instead of ID")
	return cmd
}

func getOrgID(store SecretStore) (string, error) {
	org, err := store.GetActiveOrganizationOrDefault()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return "", breverrors.NewValidationError("no organization found; run 'brev login' or create an organization first")
	}
	return org.ID, nil
}

func readSecretValue(value string) (string, error) {
	if value != "" {
		return value, nil
	}
	if !util.IsStdinPiped() {
		return "", breverrors.NewValidationError("secret value required: pass --value or pipe it via stdin")
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return strings.TrimSpace(string(data)), nil
}

func findVersionID(store SecretStore, secretID string, versionNumber int64) (string, error) {
	versions, err := store.ListManagedSecretVersions(secretID)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	for _, v := range versions {
		if v.GetVersionNumber() == versionNumber {
			return v.GetVersionId(), nil
		}
	}
	return "", breverrors.NewValidationError(fmt.Sprintf("version %d not found", versionNumber))
}

func resolveSecret(store SecretStore, id string, name string) (*nodev1.ManagedSecret, error) {
	if id != "" && name != "" {
		return nil, breverrors.NewValidationError("provide either --id or --name, not both")
	}
	if id != "" {
		return store.GetManagedSecret(id)
	}
	if name != "" {
		orgID, err := getOrgID(store)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		return store.GetManagedSecretByName(orgID, name)
	}
	return nil, breverrors.NewValidationError("provide --id or --name")
}

func findLatestVersionID(store SecretStore, secretID string) (string, error) {
	versions, err := store.ListManagedSecretVersions(secretID)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	var latest *nodev1.ManagedSecretVersion
	for _, v := range versions {
		if latest == nil || v.GetVersionNumber() > latest.GetVersionNumber() {
			latest = v
		}
	}
	if latest == nil {
		return "", breverrors.NewValidationError("secret has no versions")
	}
	return latest.GetVersionId(), nil
}

func getSecretTableOptions() table.Options {
	options := table.OptionsDefault
	options.DrawBorder = false
	options.SeparateColumns = false
	options.SeparateRows = false
	options.SeparateHeader = false
	return options
}
func formatTimestamp(ts *timestamppb.Timestamp) string {
	if ts == nil || !ts.IsValid() {
		return ""
	}
	return ts.AsTime().Format(time.RFC3339)
}
