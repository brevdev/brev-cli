package ports

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
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

// NewCmdOpenPort creates the `brev ports open` command.
func NewCmdOpenPort(portStore Store) *cobra.Command {
	var protocol string
	var allowedSources []string
	var jsonOutput bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"access": ""},
		Use:                   "open <instance-or-node> <port>",
		Aliases:               []string{"add"},
		DisableFlagsInUseLine: true,
		Short:                 "Open a public port on an instance or external node",
		Example: `
  brev ports open my-instance 8080
  brev ports open my-node 53 --protocol udp
  brev ports open my-instance 8080 --allow 203.0.113.10/32`,
		Args: cmderrors.TransformToValidationError(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			portNumber, err := parsePortNumber(args[1])
			if err != nil {
				return err
			}
			portProtocol, err := parseProtocol(protocol)
			if err != nil {
				return err
			}
			allowedSources, err = normalizeAllowedSources(allowedSources)
			if err != nil {
				return err
			}
			if err := Open(cmd.Context(), cmd.OutOrStdout(), portStore, args[0], portNumber, portProtocol, allowedSources, jsonOutput); err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&protocol, "protocol", "tcp", "port protocol (tcp, udp, or ssh)")
	cmd.Flags().StringArrayVar(&allowedSources, "allow", nil, "source CIDR allowed to connect (repeatable; omit to allow all)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output the opened port as JSON")
	_ = cmd.RegisterFlagCompletionFunc("protocol", cobra.FixedCompletions(
		[]string{"tcp", "udp", "ssh"},
		cobra.ShellCompDirectiveNoFileComp,
	))

	return cmd
}

// Open resolves a managed instance or registered compute node and opens a port.
func Open(
	ctx context.Context,
	out io.Writer,
	portStore Store,
	nameOrID string,
	portNumber int32,
	protocol devplanev1.PortProtocol,
	allowedSources []string,
	jsonOutput bool,
) error {
	target, err := cmdutil.ResolveWorkspaceOrNode(portStore, nameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	var openedPort *devplanev1.Port
	if target.Workspace != nil {
		client := register.NewEnvironmentServiceClient(portStore, config.GlobalConfig.GetBrevPublicAPIURL())
		resp, err := client.OpenPort(ctx, connect.NewRequest(&devplanev1.EnvironmentServiceOpenPortRequest{
			EnvironmentId:  target.Workspace.ID,
			Protocol:       protocol,
			PortNumber:     portNumber,
			AllowedSources: allowedSources,
		}))
		if err != nil {
			return fmt.Errorf("open port on instance %q: %w", nameOrID, err)
		}
		if resp != nil && resp.Msg != nil {
			openedPort = resp.Msg.GetPort()
		}
	} else if target.Node != nil {
		client := register.NewNodeServiceClient(portStore, config.GlobalConfig.GetBrevPublicAPIURL())
		resp, err := client.OpenPort(ctx, connect.NewRequest(&devplanev1.OpenPortRequest{
			ExternalNodeId: target.Node.GetExternalNodeId(),
			Protocol:       protocol,
			PortNumber:     portNumber,
			AllowedSources: allowedSources,
		}))
		if err != nil {
			return fmt.Errorf("open port on external node %q: %w", nameOrID, err)
		}
		if resp != nil && resp.Msg != nil {
			openedPort = resp.Msg.GetPort()
		}
	}

	if openedPort == nil {
		return fmt.Errorf("open port on %q: API returned no port", nameOrID)
	}
	return writeOpenResult(out, nameOrID, openedPort, jsonOutput)
}

func parsePortNumber(value string) (int32, error) {
	portNumber, err := strconv.ParseInt(value, 10, 32)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return 0, fmt.Errorf("invalid port %q: must be a number between 1 and 65535", value)
	}
	return int32(portNumber), nil
}

func parseProtocol(value string) (devplanev1.PortProtocol, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "tcp":
		return devplanev1.PortProtocol_PORT_PROTOCOL_TCP, nil
	case "udp":
		return devplanev1.PortProtocol_PORT_PROTOCOL_UDP, nil
	case "ssh":
		return devplanev1.PortProtocol_PORT_PROTOCOL_SSH, nil
	default:
		return devplanev1.PortProtocol_PORT_PROTOCOL_UNSPECIFIED,
			fmt.Errorf("invalid protocol %q: must be tcp, udp, or ssh", value)
	}
}

func normalizeAllowedSources(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("allowed source cannot be empty")
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func writeOpenResult(out io.Writer, nameOrID string, port *devplanev1.Port, jsonOutput bool) error {
	portInfo := toPortInfos([]*devplanev1.Port{port})[0]
	if jsonOutput {
		encoded, err := json.MarshalIndent(portInfo, "", "  ")
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		_, err = fmt.Fprintln(out, string(encoded))
		return breverrors.WrapAndTrace(err)
	}

	_, err := fmt.Fprintf(out, "Opened %s port %d on %s.\n", portInfo.Protocol, port.GetServerPort(), nameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return displayTables(out, nameOrID, []PortInfo{portInfo})
}
