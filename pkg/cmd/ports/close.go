package ports

import (
	"context"
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
)

// NewCmdClosePort creates the `brev ports remove` command.
func NewCmdClosePort(portStore Store) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"networking": ""},
		Use:                   "remove <port-id> | <instance-or-brev-connect-machine> <destination-port>",
		Aliases:               []string{"rm"},
		DisableFlagsInUseLine: true,
		Short:                 "Remove a Brev-managed port by ID or destination port",
		Long: `Remove a Brev-managed port.

Use a globally unique port ID without specifying its owner. To select by
destination port instead, also provide the environment or Brev Connect machine.
When multiple mappings share that destination, use an exact ID instead.`,
		Example: `
  brev ports remove nport-abc123
  brev ports remove my-instance 8080
  brev ports rm nport-abc123`,
		Args: cmderrors.TransformToValidationError(cobra.RangeArgs(1, 2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return breverrors.WrapAndTrace(runRemoveByID(
					cmd.Context(), cmd.OutOrStdout(), portStore, args[0],
				))
			}
			return breverrors.WrapAndTrace(runRemoveByDestination(
				cmd.Context(), cmd.OutOrStdout(), portStore, args[0], args[1],
			))
		},
	}
	return cmd
}

func runRemoveByID(
	ctx context.Context,
	out io.Writer,
	portStore Store,
	portID string,
) error {
	target, port, err := resolvePortID(ctx, portStore, portID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if err := closePort(ctx, portStore, target, port.GetPortId()); err != nil {
		return breverrors.WrapAndTrace(fmt.Errorf("remove port_id %q: %w", port.GetPortId(), err))
	}
	return writeRemoveResult(out, target, port)
}

func runRemoveByDestination(
	ctx context.Context,
	out io.Writer,
	portStore Store,
	nameOrID string,
	destination string,
) error {
	target, apiPorts, err := resolveTargetPorts(ctx, portStore, nameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	selected, err := resolvePortByDestination(apiPorts, destination)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if err := closePort(ctx, portStore, target, selected.GetPortId()); err != nil {
		return breverrors.WrapAndTrace(fmt.Errorf("remove port_id %q: %w", selected.GetPortId(), err))
	}
	return writeRemoveResult(out, target, selected)
}

func writeRemoveResult(out io.Writer, target *cmdutil.WorkspaceOrNode, port *devplanev1.Port) error {
	name := "unknown target"
	if target.Workspace != nil {
		name = target.Workspace.Name
		if name == "" {
			name = target.Workspace.ID
		}
	} else if target.Node != nil {
		name = target.Node.GetName()
		if name == "" {
			name = target.Node.GetExternalNodeId()
		}
	}
	_, err := fmt.Fprintf(
		out,
		"Removed %s port %d on %s.\n",
		protocolLabel(port, isHTTPPort(port)),
		port.GetServerPort(),
		name,
	)
	return breverrors.WrapAndTrace(err)
}

func resolvePortByDestination(ports []*devplanev1.Port, value string) (*devplanev1.Port, error) {
	value = strings.TrimSpace(value)
	destinationPort, err := parsePortNumber(value)
	if err != nil {
		return nil, breverrors.NewValidationError("destination port must be a number between 1 and 65535")
	}
	matches := make([]*devplanev1.Port, 0, 1)
	for _, port := range ports {
		if port != nil && port.GetPortId() != "" && port.GetServerPort() == destinationPort {
			matches = append(matches, port)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("destination port %d is not open on this target", destinationPort)
	}
	if len(matches) > 1 {
		portIDs := make([]string, len(matches))
		for i, port := range matches {
			portIDs[i] = port.GetPortId()
		}
		return nil, fmt.Errorf(
			"destination port %d matches multiple ports (%s); use an exact port_id from `brev ports ls`",
			destinationPort, strings.Join(portIDs, ", "),
		)
	}
	return matches[0], nil
}

func closePort(
	ctx context.Context,
	portStore Store,
	target *cmdutil.WorkspaceOrNode,
	portID string,
) error {
	if target.Workspace != nil {
		client := register.NewEnvironmentServiceClient(portStore, config.GlobalConfig.GetBrevPublicAPIURL())
		_, err := client.ClosePort(ctx, connect.NewRequest(&devplanev1.EnvironmentServiceClosePortRequest{
			PortId: portID,
		}))
		return breverrors.WrapAndTrace(err)
	}
	if target.Node != nil {
		client := register.NewNodeServiceClient(portStore, config.GlobalConfig.GetBrevPublicAPIURL())
		_, err := client.ClosePort(ctx, connect.NewRequest(&devplanev1.ClosePortRequest{
			PortId: portID,
		}))
		return breverrors.WrapAndTrace(err)
	}
	return fmt.Errorf("resolved target has no instance or Brev Connect machine")
}
