// Package launchable implements the `brev launchable` command tree.
//
// The launchable create path POSTs to a private Brev control-plane endpoint
// (/api/organizations/{orgID}/v2/launchables) that is not part of a public API
// surface. The request shape was reverse-engineered from the Console's wizard
// payload; the endpoint may evolve without CLI-visible versioning.
package launchable

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

// LaunchableStore is the subset of store methods needed by this command.
type LaunchableStore interface {
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	CreateLaunchable(organizationID string, req *store.CreateLaunchableRequest) (*store.LaunchableResponse, error)
}

// Valid values for --view-access.
const (
	viewAccessPublic  = "public"
	viewAccessPrivate = "private"
)

// subcommandUsageTemplate is the stock cobra usage template, used to override
// the root command's category-based template (which hides generic subcommands).
const subcommandUsageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

func NewCmdLaunchable(t *terminal.Terminal, s LaunchableStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "launchable",
		Short: "Manage launchables",
		Long: `Manage launchables — reusable, shareable instance + build templates.

To deploy an existing launchable, use ` + "`brev create --launchable <id>`" + `.`,
		Annotations: map[string]string{"workspace": ""},
	}
	cmd.AddCommand(newCmdCreate(t, s))

	// The root command installs a category-based usage template that omits any
	// subcommand without a category annotation. Restore the stock cobra
	// template so `brev launchable --help` lists its subcommands.
	cmd.SetUsageTemplate(subcommandUsageTemplate)
	return cmd
}

func newCmdCreate(t *terminal.Terminal, s LaunchableStore) *cobra.Command {
	var (
		specPath    string
		nameFlag    string
		description string
		viewAccess  string
		orgID       string
	)

	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new launchable from a JSON spec",
		Long: `Create a new launchable template.

The spec file is JSON matching the body accepted by the Brev control-plane
launchable endpoint. At minimum it must define createWorkspaceRequest
(instanceType, workspaceGroupId) and buildRequest (one of dockerCompose,
containerBuild, vmBuild).

The positional [name] and the --name flag both override the spec's "name"
field. --description, --view-access, and --org likewise override their
respective fields in the spec.`,
		Example: `  # Create from a spec file
  brev launchable create my-launchable -f spec.json

  # Override name, description, and visibility from the CLI
  brev launchable create -f spec.json --name "CUDA Tutorial" --view-access public

  # Pin to a specific organization instead of the active one
  brev launchable create -f spec.json --org org-XXXXXXXX`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			positional := ""
			if len(args) == 1 {
				positional = args[0]
			}
			return runCreate(t, s, specPath, positional, nameFlag, description, viewAccess, orgID)
		},
	}

	cmd.Flags().StringVarP(&specPath, "from-file", "f", "", "Path to a JSON launchable spec (required)")
	if err := cmd.MarkFlagRequired("from-file"); err != nil {
		// Unreachable: MarkFlagRequired only fails when the flag name doesn't exist.
		panic(fmt.Errorf("marking --from-file required: %w", err))
	}
	cmd.Flags().StringVar(&nameFlag, "name", "", "Launchable name (overrides spec)")
	cmd.Flags().StringVar(&description, "description", "", "Launchable description (overrides spec)")
	cmd.Flags().StringVar(&viewAccess, "view-access", "", `"public" or "private" (overrides spec)`)
	cmd.Flags().StringVar(&orgID, "org", "", "Organization ID (defaults to active org)")

	return cmd
}

func runCreate(t *terminal.Terminal, s LaunchableStore, specPath, positionalName, nameFlag, description, viewAccess, orgID string) error {
	req, err := loadSpec(specPath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	applyOverrides(req, positionalName, nameFlag, description, viewAccess)

	if err := validateRequest(req); err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// The API returns `"ports": null` as a validation error; the Console always
	// sends an array. Normalize here so callers don't have to think about it.
	if req.BuildRequest.Ports == nil {
		req.BuildRequest.Ports = []store.LaunchablePort{}
	}

	if orgID == "" {
		org, err := s.GetActiveOrganizationOrDefault()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if org == nil {
			return fmt.Errorf("no active organization — pass --org or set one with `brev set`")
		}
		orgID = org.ID
	}

	t.Vprintf("Creating launchable %s in org %s...\n", t.Yellow(req.Name), t.Yellow(orgID))

	resp, err := s.CreateLaunchable(orgID, req)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprintf("%s\n", t.Green(fmt.Sprintf("✓ Created launchable %s (%s)", resp.Name, resp.ID)))
	t.Vprintf("  Deploy with: %s\n", t.Yellow(fmt.Sprintf("brev create --launchable %s", resp.ID)))

	return nil
}

func loadSpec(path string) (*store.CreateLaunchableRequest, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is user-supplied on purpose
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	var req store.CreateLaunchableRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &req, nil
}

// applyOverrides layers CLI flag/positional values on top of the spec. The
// positional name wins over --name because `brev <verb> <noun>` is the more
// conventional CLI form.
func applyOverrides(req *store.CreateLaunchableRequest, positionalName, nameFlag, description, viewAccess string) {
	if positionalName != "" {
		req.Name = positionalName
	} else if nameFlag != "" {
		req.Name = nameFlag
	}
	if description != "" {
		req.Description = description
	}
	if viewAccess != "" {
		req.ViewAccess = viewAccess
	}
}

func validateRequest(req *store.CreateLaunchableRequest) error {
	if req.Name == "" {
		return fmt.Errorf("name is required (set in spec, via --name, or as positional arg)")
	}
	if req.CreateWorkspaceRequest.InstanceType == "" {
		return fmt.Errorf("createWorkspaceRequest.instanceType is required")
	}
	if req.CreateWorkspaceRequest.WorkspaceGroupID == "" {
		return fmt.Errorf("createWorkspaceRequest.workspaceGroupId is required")
	}
	build := req.BuildRequest
	if build.DockerCompose == nil && build.CustomContainer == nil && build.VMBuild == nil {
		return fmt.Errorf("buildRequest must set one of dockerCompose, containerBuild, or vmBuild")
	}
	if req.ViewAccess != "" {
		va := strings.ToLower(req.ViewAccess)
		if va != viewAccessPublic && va != viewAccessPrivate {
			return fmt.Errorf("viewAccess must be %q or %q, got %q", viewAccessPublic, viewAccessPrivate, req.ViewAccess)
		}
		req.ViewAccess = va
	}
	return nil
}
