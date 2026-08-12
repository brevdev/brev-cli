package ports

import (
	"context"
	"errors"
	"fmt"
	"io"

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

type closeOptions struct {
	portID  string
	all     bool
	approve bool
}

type closePrompter interface {
	terminal.Selector
	terminal.Confirmer
}

// NewCmdClosePort creates the `brev ports close` command.
func NewCmdClosePort(portStore Store) *cobra.Command {
	return newCmdClosePort(portStore, register.TerminalPrompter{})
}

func newCmdClosePort(portStore Store, prompter closePrompter) *cobra.Command {
	var opts closeOptions

	cmd := &cobra.Command{
		Annotations:           map[string]string{"access": ""},
		Use:                   "close <instance-or-node>",
		Aliases:               []string{"remove"},
		DisableFlagsInUseLine: true,
		Short:                 "[beta] Close public ports on an instance or external node",
		Example: `
  brev ports close my-instance
  brev ports close my-instance --id nport-abc123 --approve
  brev ports close my-node --all --approve`,
		Args: cmderrors.TransformToValidationError(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.all && opts.portID != "" {
				return breverrors.NewValidationError("--all and --id cannot be used together")
			}
			if err := runClose(cmd.Context(), cmd.OutOrStdout(), portStore, prompter, args[0], opts); err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&opts.portID, "id", "", "close the exact port mapping with this port_id")
	cmd.Flags().BoolVar(&opts.all, "all", false, "close every port mapping on the target")
	cmd.Flags().BoolVar(&opts.approve, "approve", false, "skip confirmation prompt (assume yes)")
	return cmd
}

func runClose(
	ctx context.Context,
	out io.Writer,
	portStore Store,
	prompter closePrompter,
	nameOrID string,
	opts closeOptions,
) error {
	target, apiPorts, err := resolveTargetPorts(ctx, portStore, nameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	removable := removablePorts(apiPorts)
	if len(removable) == 0 {
		return fmt.Errorf("no removable ports are open on %s", nameOrID)
	}

	selected, err := selectPortsToClose(prompter, removable, opts)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if err := displayCloseConfirmation(out, nameOrID, selected); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !opts.approve && !prompter.ConfirmYesNo(closeConfirmationLabel(nameOrID, len(selected))) {
		_, err := fmt.Fprintln(out, "No ports were closed.")
		return breverrors.WrapAndTrace(err)
	}

	return closePorts(ctx, out, portStore, target, nameOrID, selected)
}

func removablePorts(apiPorts []*devplanev1.Port) []*devplanev1.Port {
	ports := make([]*devplanev1.Port, 0, len(apiPorts))
	for _, port := range apiPorts {
		if port != nil && port.GetPortId() != "" {
			ports = append(ports, port)
		}
	}
	return ports
}

func selectPortsToClose(
	prompter terminal.Selector,
	ports []*devplanev1.Port,
	opts closeOptions,
) ([]*devplanev1.Port, error) {
	if opts.all {
		return ports, nil
	}
	if opts.portID != "" {
		for _, port := range ports {
			if port.GetPortId() == opts.portID {
				return []*devplanev1.Port{port}, nil
			}
		}
		return nil, fmt.Errorf("port_id %q is not open on this target", opts.portID)
	}

	labels := make([]string, len(ports))
	for i, port := range ports {
		labels[i] = closeSelectionLabel(i, port)
	}
	chosen := prompter.Select("Select a port to close", labels)
	for i, label := range labels {
		if label == chosen {
			return []*devplanev1.Port{ports[i]}, nil
		}
	}
	return nil, fmt.Errorf("selected item did not match any open port")
}

func closeSelectionLabel(index int, port *devplanev1.Port) string {
	info := toPortInfos([]*devplanev1.Port{port})[0]
	return fmt.Sprintf(
		"%d. %s  %s  public %s -> destination %s",
		index+1,
		info.Protocol,
		valueOrDash(info.Endpoint),
		portNumberLabel(info.PublicPort),
		portNumberLabel(info.DestinationPort),
	)
}

func displayCloseConfirmation(out io.Writer, nameOrID string, ports []*devplanev1.Port) error {
	if _, err := fmt.Fprintf(out, "The following port mapping(s) will be permanently removed from %s:\n\n", nameOrID); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if err := displayTables(out, nameOrID, toPortInfos(ports)); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	_, err := fmt.Fprintln(out, "\nActive connections may be dropped and this action cannot be undone.")
	return breverrors.WrapAndTrace(err)
}

func closeConfirmationLabel(nameOrID string, count int) string {
	portWord := "ports"
	if count == 1 {
		portWord = "port"
	}
	return fmt.Sprintf("Close %d %s on %s?", count, portWord, nameOrID)
}

func closePorts(
	ctx context.Context,
	out io.Writer,
	portStore Store,
	target *cmdutil.WorkspaceOrNode,
	nameOrID string,
	ports []*devplanev1.Port,
) error {
	var closeErrors []error
	closed := 0
	for _, port := range ports {
		if err := closePort(ctx, portStore, target, port.GetPortId()); err != nil {
			closeErrors = append(closeErrors, fmt.Errorf("close port_id %q: %w", port.GetPortId(), err))
			continue
		}
		closed++
	}

	if closed > 0 {
		portWord := "ports"
		if closed == 1 {
			portWord = "port"
		}
		if _, err := fmt.Fprintf(out, "Closed %d %s on %s.\n", closed, portWord, nameOrID); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	if err := errors.Join(closeErrors...); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
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
	return fmt.Errorf("resolved target has no instance or external node")
}
