// Package launch provides local and remote launchable execution.
package launch

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/url"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/gpucreate"
	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/names"
	"github.com/brevdev/brev-cli/pkg/ssh"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

const defaultLaunchTimeoutSeconds = 300

// Store contains the authenticated APIs used by remote and local launches.
type Store interface {
	gpucreate.GPUCreateStore
	GetAccessToken() (string, error)
}

type commandOptions struct {
	name         string
	instanceType string
	parameters   []string
	secrets      []string
	local        bool
	detached     bool
	approve      bool
	explain      bool
	timeout      int
}

type launchCommandArgs struct {
	cmd        *cobra.Command
	terminal   *terminal.Terminal
	store      Store
	launchable string
	options    commandOptions
	deps       launchDeps
}

type remoteLaunchArgs struct {
	terminal     *terminal.Terminal
	store        Store
	launchableID string
	info         *store.LaunchableResponse
	bindings     []store.ParameterBinding
	name         string
	options      commandOptions
}

// NewCmdLaunch creates the launch command.
func NewCmdLaunch(t *terminal.Terminal, launchStore Store) *cobra.Command {
	return newCmdLaunch(t, launchStore, defaultLaunchDeps(launchStore))
}

func newCmdLaunch(t *terminal.Terminal, launchStore Store, deps launchDeps) *cobra.Command {
	opts := commandOptions{}
	cmd := &cobra.Command{
		Annotations: map[string]string{"workspace": ""},
		Use:         "launch <launchable-id-or-url>",
		Short:       "Launch a launchable locally or on a Brev instance",
		Long: `Launch a launchable on its recommended remote Brev instance, or use
--local to run its build on this machine. Local mode supports VM startup
scripts, custom containers, and Docker Compose builds.`,
		DisableFlagsInUseLine: true,
		Args:                  cobra.ExactArgs(1),
		Example: `  # Provision the launchable on a remote Brev instance
  brev launch env-abc

  # Inspect its definition without launching
  brev launch env-abc --explain

  # Launch on this machine
  brev launch env-abc --local

  # Supply direct parameter values
  brev launch env-abc --param MODEL=llama --param PORT=8080

  # Bind a text parameter to the latest or a specific managed-secret version
  brev launch env-abc --param-secret API_TOKEN=msec-abc
  brev launch env-abc --param-secret API_TOKEN=msec-abc:msecv-123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLaunchCommand(cmd.Context(), launchCommandArgs{
				cmd:        cmd,
				terminal:   t,
				store:      launchStore,
				launchable: args[0],
				options:    opts,
				deps:       deps,
			})
		},
	}
	cmd.Flags().StringVarP(&opts.name, "name", "n", "", "Instance, container, or Compose project name")
	cmd.Flags().StringVarP(&opts.instanceType, "type", "t", "", "Comma-separated remote instance types to try")
	cmd.Flags().StringArrayVar(&opts.parameters, "param", nil, "Launchable parameter NAME=VALUE (repeatable)")
	cmd.Flags().StringArrayVar(&opts.secrets, "param-secret", nil, "Text parameter NAME=SECRET_ID[:VERSION_ID] (repeatable)")
	cmd.Flags().BoolVar(&opts.local, "local", false, "Run on this machine without provisioning an instance")
	cmd.Flags().BoolVarP(&opts.detached, "detached", "d", false, "Do not wait for the remote instance or local Docker workload")
	cmd.Flags().BoolVar(&opts.approve, "approve", false, "Run a local startup script without prompting")
	cmd.Flags().BoolVar(&opts.explain, "explain", false, "Show launchable details without launching")
	cmd.Flags().IntVar(&opts.timeout, "timeout", defaultLaunchTimeoutSeconds, "Remote instance readiness timeout in seconds")
	return cmd
}

func runLaunchCommand(ctx context.Context, args launchCommandArgs) error {
	if err := validateCommandOptions(args.cmd, args.options); err != nil {
		return err
	}

	launchableID, err := parseLaunchableID(args.launchable)
	if err != nil {
		return err
	}
	if args.options.explain {
		return explainLaunchable(args.cmd.OutOrStdout(), args.store, launchableID)
	}
	info, err := fetchLaunchable(args.store, launchableID)
	if err != nil {
		return err
	}
	displayLaunchable(args.terminal, info, args.options.local)

	values, err := parseParameterValues(args.options.parameters)
	if err != nil {
		return err
	}
	secretRefs, err := parseParameterSecrets(args.options.secrets)
	if err != nil {
		return err
	}
	bindings, err := resolveParameterBindings(ctx, parameterBindingArgs{
		parameters: info.BuildRequest.Parameters,
		values:     values,
		secrets:    secretRefs,
		resolver:   args.deps.secrets,
	})
	if err != nil {
		return err
	}

	name, err := launchName(info.Name, args.options.name, args.options.local)
	if err != nil {
		return err
	}
	if args.options.local {
		return runLocalLaunchable(ctx, localLaunchArgs{
			terminal:     args.terminal,
			launchableID: launchableID,
			info:         info,
			bindings:     bindings,
			options: localOptions{
				name:     name,
				detached: args.options.detached,
				approve:  args.options.approve,
				stdin:    args.cmd.InOrStdin(),
				stdout:   args.cmd.OutOrStdout(),
				stderr:   args.cmd.ErrOrStderr(),
			},
			deps: args.deps,
		})
	}
	return runRemoteLaunch(remoteLaunchArgs{
		terminal:     args.terminal,
		store:        args.store,
		launchableID: launchableID,
		info:         info,
		bindings:     bindings,
		name:         name,
		options:      args.options,
	})
}

func validateCommandOptions(cmd *cobra.Command, opts commandOptions) error {
	if opts.local && cmd.Flags().Changed("type") {
		return breverrors.NewValidationError("--type cannot be used with --local")
	}
	if opts.local && cmd.Flags().Changed("timeout") {
		return breverrors.NewValidationError("--timeout cannot be used with --local")
	}
	if !opts.local && opts.approve {
		return breverrors.NewValidationError("--approve can only be used with --local")
	}
	if opts.timeout < 1 {
		return breverrors.NewValidationError("--timeout must be at least 1 second")
	}
	return nil
}

func parseLaunchableID(input string) (string, error) {
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		parsed, err := url.Parse(input)
		if err != nil {
			return "", fmt.Errorf("invalid launchable URL: %w", err)
		}
		if id := parsed.Query().Get("launchableID"); id != "" {
			return validateLaunchableID(id)
		}
		parts := strings.Split(strings.TrimRight(parsed.Path, "/"), "/")
		if len(parts) > 0 && strings.HasPrefix(parts[len(parts)-1], "env-") {
			return validateLaunchableID(parts[len(parts)-1])
		}
		return "", fmt.Errorf("could not extract a launchable ID from URL %q", input)
	}
	return validateLaunchableID(input)
}

func explainLaunchable(out io.Writer, launchStore Store, launchableID string) error {
	info, err := fetchLaunchableMetadata(launchStore, launchableID)
	if err != nil {
		return err
	}
	lines := []string{info.Name}
	if description := strings.TrimSpace(info.Description); description != "" {
		lines = append(lines, "", description)
	}
	definitionURL, err := launchableDefinitionURL(config.GlobalConfig.GetConsoleURL(), launchableID)
	if err != nil {
		return err
	}
	lines = append(lines, "", "URL: "+definitionURL, "Build mode: "+buildModeName(info.BuildRequest), "")
	if parameterLines := parameterDisplayLines(info.BuildRequest.Parameters); len(parameterLines) > 0 {
		lines = append(lines, parameterLines...)
	} else {
		lines = append(lines, "Parameters: none")
	}
	writer := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	for _, line := range lines {
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return fmt.Errorf("write launchable explanation: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("write launchable explanation: %w", err)
	}
	return nil
}

func launchableDefinitionURL(consoleURL string, launchableID string) (string, error) {
	parsed, err := url.Parse(consoleURL)
	if err != nil {
		return "", fmt.Errorf("parse BREV_CONSOLE_URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("BREV_CONSOLE_URL must include a scheme and host")
	}
	parsed.Path = "/launchable/deploy/now"
	parsed.RawPath = ""
	parsed.RawQuery = url.Values{"launchableID": {launchableID}}.Encode()
	parsed.Fragment = ""
	return parsed.String(), nil
}

func launchName(launchableName string, requestedName string, local bool) (string, error) {
	name := requestedName
	if name == "" {
		name = ssh.SanitizeNodeName(launchableName)
		if !local {
			name = fmt.Sprintf("%s-%05d", name, rand.IntN(100000)) //nolint:gosec // uniqueness only
		}
	}
	if err := names.ValidateNodeName(name); err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return name, nil
}

func runRemoteLaunch(args remoteLaunchArgs) error {
	instanceTypes, err := remoteInstanceTypes(args.options.instanceType, args.info.CreateWorkspaceRequest.InstanceType)
	if err != nil {
		return err
	}
	err = gpucreate.RunGPUCreate(args.terminal, args.store, gpucreate.GPUCreateOptions{
		Name:              args.name,
		InstanceTypes:     instanceTypes,
		Count:             1,
		Parallel:          1,
		Detached:          args.options.detached,
		Timeout:           time.Duration(args.options.timeout) * time.Second,
		LaunchableID:      args.launchableID,
		LaunchableInfo:    args.info,
		ParameterBindings: args.bindings,
	})
	if err != nil {
		return fmt.Errorf("launch on remote Brev instance: %w", err)
	}
	return nil
}

func remoteInstanceTypes(flagValue string, recommended string) ([]gpucreate.InstanceSpec, error) {
	if strings.TrimSpace(flagValue) == "" {
		if strings.TrimSpace(recommended) == "" {
			return nil, breverrors.NewValidationError("launchable has no instance type configured; provide --type")
		}
		return []gpucreate.InstanceSpec{{Type: recommended}}, nil
	}
	parts := strings.Split(flagValue, ",")
	result := make([]gpucreate.InstanceSpec, 0, len(parts))
	for _, part := range parts {
		instanceType := strings.TrimSpace(part)
		if instanceType == "" {
			return nil, breverrors.NewValidationError("--type contains an empty instance type")
		}
		result = append(result, gpucreate.InstanceSpec{Type: instanceType})
	}
	return result, nil
}

func fetchLaunchable(launchStore Store, launchableID string) (*store.LaunchableResponse, error) {
	info, err := fetchLaunchableMetadata(launchStore, launchableID)
	if err != nil {
		return nil, err
	}
	if info.BuildRequest.VMBuild == nil || info.BuildRequest.VMBuild.LifeCycleScriptAttr == nil {
		return info, nil
	}
	attr := info.BuildRequest.VMBuild.LifeCycleScriptAttr
	if attr.ID == "" {
		return info, nil
	}
	script, err := launchStore.GetLaunchableLifeCycleScript(launchableID, attr.ID)
	if err != nil {
		return nil, fmt.Errorf("fetch startup script %q for launchable %q: %w", attr.ID, launchableID, err)
	}
	if script != nil && script.Attrs != nil {
		attr.Script = script.Attrs.Script
	}
	return info, nil
}

func fetchLaunchableMetadata(launchStore Store, launchableID string) (*store.LaunchableResponse, error) {
	info, err := launchStore.GetLaunchable(launchableID)
	if err != nil {
		return nil, fmt.Errorf("fetch launchable %q: %w", launchableID, err)
	}
	if info == nil {
		return nil, fmt.Errorf("fetch launchable %q: API returned no configuration", launchableID)
	}
	return info, nil
}

func displayLaunchable(t *terminal.Terminal, info *store.LaunchableResponse, local bool) {
	target := "a remote Brev instance"
	if local {
		target = "this machine"
	}
	t.Vprintf("Launching %q on %s.\n", info.Name, target)
	if info.Description != "" {
		t.Vprintf("Description: %s\n", info.Description)
	}
	t.Vprintf("Build mode: %s\n", buildModeName(info.BuildRequest))
	for _, line := range parameterDisplayLines(info.BuildRequest.Parameters) {
		t.Vprintf("%s\n", line)
	}
	t.Vprint("")
}

func validateLaunchableID(id string) (string, error) {
	if strings.TrimSpace(id) == "" || strings.ContainsAny(id, "/?&#") {
		return "", fmt.Errorf("invalid launchable ID %q", id)
	}
	return id, nil
}

func buildModeName(build store.LaunchableBuildRequest) string {
	switch {
	case build.VerbBuild != nil:
		return "Verb"
	case build.CustomContainer != nil:
		return "Container"
	case build.DockerCompose != nil:
		return "Docker Compose"
	case build.VMBuild != nil:
		return "VM"
	default:
		return "Unknown"
	}
}
