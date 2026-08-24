package ports

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/register"
	cmdutil "github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type updateOptions struct {
	portID           string
	destinationPort  string
	allowedSources   []string
	allowAnywhere    bool
	protocol         string
	authorizedEmails []string
	public           bool
	jsonOutput       bool

	destinationPortSet  bool
	allowedSourcesSet   bool
	allowAnywhereSet    bool
	protocolSet         bool
	authorizedEmailsSet bool
	publicSet           bool
}

type portUpdates struct {
	destinationPort  *int32
	allowedSources   *[]string
	httpProtocol     *devplanev1.HttpPortProtocol
	authorizedEmails *[]string
	public           bool
}

type updatePrompter interface {
	terminal.Selector
}

// NewCmdUpdatePort creates the `brev ports update` command.
func NewCmdUpdatePort(portStore Store) *cobra.Command {
	return newCmdUpdatePort(portStore, register.TerminalPrompter{})
}

func newCmdUpdatePort(portStore Store, prompter updatePrompter) *cobra.Command {
	var opts updateOptions

	cmd := &cobra.Command{
		Annotations:           map[string]string{"access": ""},
		Use:                   "update <instance-or-node>",
		Aliases:               []string{"edit"},
		Hidden:                true,
		DisableFlagsInUseLine: true,
		Short:                 "[beta] Update a public port on an instance or external node",
		Example: `
  brev ports update my-instance --id nport-abc123 --destination-port 8081
  brev ports update my-instance --id nport-abc123 --allow 203.0.113.10/32
  brev ports update my-node --id nport-abc123 --allow-anywhere
  brev ports update my-instance --id nport-abc123 --protocol https
  brev ports update my-instance --id nport-abc123 --public`,
		Args: cmderrors.TransformToValidationError(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.destinationPortSet = cmd.Flags().Changed("destination-port")
			opts.allowedSourcesSet = cmd.Flags().Changed("allow")
			opts.allowAnywhereSet = cmd.Flags().Changed("allow-anywhere")
			opts.protocolSet = cmd.Flags().Changed("protocol")
			opts.authorizedEmailsSet = cmd.Flags().Changed("authorize")
			opts.publicSet = cmd.Flags().Changed("public")

			updates, err := buildPortUpdates(opts)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return breverrors.WrapAndTrace(runUpdate(
				cmd.Context(), cmd.OutOrStdout(), portStore, prompter, args[0], opts.portID, updates, opts.jsonOutput,
			))
		},
	}

	cmd.Flags().StringVar(&opts.portID, "id", "", "update the exact port mapping with this port_id (omit to select interactively)")
	cmd.Flags().StringVar(&opts.destinationPort, "destination-port", "", "new destination port (1-65535)")
	cmd.Flags().StringArrayVar(&opts.allowedSources, "allow", nil, "replace source restrictions with this CIDR (repeatable)")
	cmd.Flags().BoolVar(&opts.allowAnywhere, "allow-anywhere", false, "clear all source restrictions")
	cmd.Flags().StringVar(&opts.protocol, "protocol", "", "new origin protocol for an HTTP mapping (http or https)")
	cmd.Flags().StringArrayVar(&opts.authorizedEmails, "authorize", nil, "replace HTTP access with this authorized email (repeatable)")
	cmd.Flags().BoolVar(&opts.public, "public", false, "make an HTTP mapping publicly accessible without authentication")
	cmd.Flags().BoolVar(&opts.jsonOutput, "json", false, "output the updated port as JSON")
	_ = cmd.RegisterFlagCompletionFunc("protocol", cobra.FixedCompletions(
		[]string{"http", "https"},
		cobra.ShellCompDirectiveNoFileComp,
	))

	return cmd
}

func buildPortUpdates(opts updateOptions) (portUpdates, error) {
	var updates portUpdates
	var err error

	updates.destinationPort, err = destinationPortUpdate(opts)
	if err != nil {
		return updates, err
	}
	updates.allowedSources, err = allowedSourcesUpdate(opts)
	if err != nil {
		return updates, err
	}
	updates.httpProtocol, err = httpProtocolUpdate(opts)
	if err != nil {
		return updates, err
	}
	updates.authorizedEmails, updates.public, err = httpAccessUpdate(opts)
	if err != nil {
		return updates, err
	}

	if !updates.hasChanges() {
		return updates, breverrors.NewValidationError("specify at least one update: --destination-port, --allow, --allow-anywhere, --protocol, --authorize, or --public")
	}
	return updates, nil
}

func destinationPortUpdate(opts updateOptions) (*int32, error) {
	if !opts.destinationPortSet {
		return nil, nil
	}
	portNumber, err := parsePortNumber(opts.destinationPort)
	if err != nil {
		return nil, err
	}
	return &portNumber, nil
}

func allowedSourcesUpdate(opts updateOptions) (*[]string, error) {
	if opts.allowedSourcesSet && opts.allowAnywhereSet {
		return nil, breverrors.NewValidationError("--allow and --allow-anywhere cannot be used together")
	}
	if opts.allowAnywhereSet && !opts.allowAnywhere {
		return nil, breverrors.NewValidationError("--allow-anywhere=false does not update the port; omit the flag or use --allow")
	}
	if opts.allowedSourcesSet {
		allowedSources, err := normalizeAllowedSources(opts.allowedSources)
		if err != nil {
			return nil, err
		}
		return &allowedSources, nil
	}
	if opts.allowAnywhere {
		allowedSources := []string{}
		return &allowedSources, nil
	}
	return nil, nil
}

func httpProtocolUpdate(opts updateOptions) (*devplanev1.HttpPortProtocol, error) {
	if !opts.protocolSet {
		return nil, nil
	}
	httpProtocol, err := parseHTTPProtocol(opts.protocol)
	if err != nil {
		return nil, err
	}
	return &httpProtocol, nil
}

func httpAccessUpdate(opts updateOptions) (*[]string, bool, error) {
	if opts.authorizedEmailsSet && opts.publicSet {
		return nil, false, breverrors.NewValidationError("--authorize and --public cannot be used together")
	}
	if opts.publicSet && !opts.public {
		return nil, false, breverrors.NewValidationError("--public=false requires an authorization policy; use --authorize instead")
	}
	if opts.authorizedEmailsSet {
		authorizedEmails, err := normalizeAuthorizedEmails(opts.authorizedEmails)
		if err != nil {
			return nil, false, err
		}
		if len(authorizedEmails) == 0 {
			return nil, false, breverrors.NewValidationError("--authorize requires at least one email")
		}
		return &authorizedEmails, false, nil
	}
	if opts.public {
		authorizedEmails := []string{}
		return &authorizedEmails, true, nil
	}
	return nil, false, nil
}

func (u portUpdates) hasChanges() bool {
	return u.destinationPort != nil || u.allowedSources != nil || u.httpProtocol != nil || u.authorizedEmails != nil
}

func runUpdate(
	ctx context.Context,
	out io.Writer,
	portStore Store,
	prompter updatePrompter,
	nameOrID string,
	portID string,
	updates portUpdates,
	jsonOutput bool,
) error {
	target, apiPorts, err := resolveTargetPorts(ctx, portStore, nameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	port, err := selectPortToUpdate(prompter, apiPorts, strings.TrimSpace(portID))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !isHTTPPort(port) && (updates.httpProtocol != nil || updates.authorizedEmails != nil) {
		return breverrors.NewValidationError("--protocol, --authorize, and --public can only update an HTTP mapping")
	}

	updated, err := applyPortUpdates(ctx, portStore, target, port, updates)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return writeUpdateResult(out, nameOrID, updated, jsonOutput)
}

func selectPortToUpdate(
	prompter terminal.Selector,
	apiPorts []*devplanev1.Port,
	portID string,
) (*devplanev1.Port, error) {
	ports := removablePorts(apiPorts)
	if len(ports) == 0 {
		return nil, fmt.Errorf("no updatable ports are open on this target")
	}
	if portID != "" {
		for _, port := range ports {
			if port.GetPortId() == portID {
				return port, nil
			}
		}
		return nil, fmt.Errorf("port_id %q is not open on this target", portID)
	}

	labels := make([]string, len(ports))
	for i, port := range ports {
		labels[i] = closeSelectionLabel(i, port)
	}
	chosen := prompter.Select("Select a port to update", labels)
	for i, label := range labels {
		if label == chosen {
			return ports[i], nil
		}
	}
	return nil, fmt.Errorf("selected item did not match any open port")
}

func isHTTPPort(port *devplanev1.Port) bool {
	return port.GetHttpProtocol() != devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_UNSPECIFIED
}

func applyPortUpdates(
	ctx context.Context,
	portStore Store,
	target *cmdutil.WorkspaceOrNode,
	port *devplanev1.Port,
	updates portUpdates,
) (*devplanev1.Port, error) {
	updated := port
	var err error

	if updates.destinationPort != nil {
		updated, err = setPortTarget(ctx, portStore, target, port.GetPortId(), *updates.destinationPort)
		if err != nil {
			return nil, fmt.Errorf("update destination port: %w", err)
		}
	}
	if updates.allowedSources != nil {
		updated, err = setPortAllowedSources(ctx, portStore, target, port.GetPortId(), *updates.allowedSources)
		if err != nil {
			return nil, fmt.Errorf("update allowed sources: %w", err)
		}
	}
	if updates.httpProtocol != nil {
		updated, err = setHTTPPortProtocol(ctx, portStore, target, port.GetPortId(), *updates.httpProtocol)
		if err != nil {
			return nil, fmt.Errorf("update HTTP protocol: %w", err)
		}
	}
	if updates.authorizedEmails != nil {
		updated, err = setHTTPPortAccess(ctx, portStore, target, port.GetPortId(), *updates.authorizedEmails, updates.public)
		if err != nil {
			return nil, fmt.Errorf("update HTTP access: %w", err)
		}
	}

	if updated == nil {
		return nil, fmt.Errorf("update port %q: API returned no port", port.GetPortId())
	}
	return updated, nil
}

//nolint:dupl // Environment and node RPCs intentionally have parallel request types.
func setPortTarget(
	ctx context.Context,
	portStore Store,
	target *cmdutil.WorkspaceOrNode,
	portID string,
	destinationPort int32,
) (*devplanev1.Port, error) {
	if target.Workspace != nil {
		client := register.NewEnvironmentServiceClient(portStore, config.GlobalConfig.GetBrevPublicAPIURL())
		resp, err := client.SetPortTarget(ctx, connect.NewRequest(&devplanev1.EnvironmentServiceSetPortTargetRequest{
			PortId: portID, PortNumber: destinationPort,
		}))
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		if resp == nil || resp.Msg == nil || resp.Msg.GetPort() == nil {
			return nil, fmt.Errorf("set destination port: API returned no port")
		}
		return resp.Msg.GetPort(), nil
	}
	if target.Node != nil {
		client := register.NewNodeServiceClient(portStore, config.GlobalConfig.GetBrevPublicAPIURL())
		resp, err := client.SetPortTarget(ctx, connect.NewRequest(&devplanev1.SetPortTargetRequest{
			PortId: portID, PortNumber: destinationPort,
		}))
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		if resp == nil || resp.Msg == nil || resp.Msg.GetPort() == nil {
			return nil, fmt.Errorf("set destination port: API returned no port")
		}
		return resp.Msg.GetPort(), nil
	}
	return nil, fmt.Errorf("resolved target has no instance or external node")
}

func setPortAllowedSources(
	ctx context.Context,
	portStore Store,
	target *cmdutil.WorkspaceOrNode,
	portID string,
	allowedSources []string,
) (*devplanev1.Port, error) {
	if target.Workspace != nil {
		client := register.NewEnvironmentServiceClient(portStore, config.GlobalConfig.GetBrevPublicAPIURL())
		resp, err := client.SetPortAllowedSources(ctx, connect.NewRequest(&devplanev1.EnvironmentServiceSetPortAllowedSourcesRequest{
			PortId: portID,
			AllowedSources: &devplanev1.EnvironmentServiceSetPortAllowedSourcesRequestAllowedSources{
				CidrBlocks: allowedSources,
			},
		}))
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		if resp == nil || resp.Msg == nil || resp.Msg.GetPort() == nil {
			return nil, fmt.Errorf("set allowed sources: API returned no port")
		}
		return resp.Msg.GetPort(), nil
	}
	if target.Node != nil {
		client := register.NewNodeServiceClient(portStore, config.GlobalConfig.GetBrevPublicAPIURL())
		resp, err := client.SetPortAllowedSources(ctx, connect.NewRequest(&devplanev1.SetPortAllowedSourcesRequest{
			PortId: portID, AllowedSources: allowedSources,
		}))
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		if resp == nil || resp.Msg == nil || resp.Msg.GetPort() == nil {
			return nil, fmt.Errorf("set allowed sources: API returned no port")
		}
		return resp.Msg.GetPort(), nil
	}
	return nil, fmt.Errorf("resolved target has no instance or external node")
}

//nolint:dupl // Environment and node RPCs intentionally have parallel request types.
func setHTTPPortProtocol(
	ctx context.Context,
	portStore Store,
	target *cmdutil.WorkspaceOrNode,
	portID string,
	protocol devplanev1.HttpPortProtocol,
) (*devplanev1.Port, error) {
	if target.Workspace != nil {
		client := register.NewEnvironmentServiceClient(portStore, config.GlobalConfig.GetBrevPublicAPIURL())
		resp, err := client.SetHTTPPortProtocol(ctx, connect.NewRequest(&devplanev1.EnvironmentServiceSetHTTPPortProtocolRequest{
			PortId: portID, HttpProtocol: protocol,
		}))
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		if resp == nil || resp.Msg == nil || resp.Msg.GetPort() == nil {
			return nil, fmt.Errorf("set HTTP protocol: API returned no port")
		}
		return resp.Msg.GetPort(), nil
	}
	if target.Node != nil {
		client := register.NewNodeServiceClient(portStore, config.GlobalConfig.GetBrevPublicAPIURL())
		resp, err := client.SetHTTPPortProtocol(ctx, connect.NewRequest(&devplanev1.SetHTTPPortProtocolRequest{
			PortId: portID, HttpProtocol: protocol,
		}))
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		if resp == nil || resp.Msg == nil || resp.Msg.GetPort() == nil {
			return nil, fmt.Errorf("set HTTP protocol: API returned no port")
		}
		return resp.Msg.GetPort(), nil
	}
	return nil, fmt.Errorf("resolved target has no instance or external node")
}

func setHTTPPortAccess(
	ctx context.Context,
	portStore Store,
	target *cmdutil.WorkspaceOrNode,
	portID string,
	authorizedEmails []string,
	public bool,
) (*devplanev1.Port, error) {
	if target.Workspace != nil {
		client := register.NewEnvironmentServiceClient(portStore, config.GlobalConfig.GetBrevPublicAPIURL())
		resp, err := client.SetHTTPPortAccess(ctx, connect.NewRequest(&devplanev1.EnvironmentServiceSetHTTPPortAccessRequest{
			PortId: portID,
			AuthorizedEmails: &devplanev1.EnvironmentServiceSetHTTPPortAccessRequestAuthorizedEmails{
				Emails: authorizedEmails,
			},
			AllowPublicUnauthenticated: public,
		}))
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		if resp == nil || resp.Msg == nil || resp.Msg.GetPort() == nil {
			return nil, fmt.Errorf("set HTTP access: API returned no port")
		}
		return resp.Msg.GetPort(), nil
	}
	if target.Node != nil {
		client := register.NewNodeServiceClient(portStore, config.GlobalConfig.GetBrevPublicAPIURL())
		resp, err := client.SetHTTPPortAccess(ctx, connect.NewRequest(&devplanev1.SetHTTPPortAccessRequest{
			PortId:                     portID,
			AuthorizedEmails:           authorizedEmails,
			AllowPublicUnauthenticated: public,
		}))
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		if resp == nil || resp.Msg == nil || resp.Msg.GetPort() == nil {
			return nil, fmt.Errorf("set HTTP access: API returned no port")
		}
		return resp.Msg.GetPort(), nil
	}
	return nil, fmt.Errorf("resolved target has no instance or external node")
}

func writeUpdateResult(out io.Writer, nameOrID string, port *devplanev1.Port, jsonOutput bool) error {
	portInfo := toPortInfos([]*devplanev1.Port{port})[0]
	if jsonOutput {
		encoded, err := json.MarshalIndent(portInfo, "", "  ")
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		_, err = fmt.Fprintln(out, string(encoded))
		return breverrors.WrapAndTrace(err)
	}

	if _, err := fmt.Fprintf(out, "Updated port %s on %s.\n", port.GetPortId(), nameOrID); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return displayTables(out, nameOrID, []PortInfo{portInfo})
}
