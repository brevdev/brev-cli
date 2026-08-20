// Package ports displays Brev-managed public port mappings for an instance or external node.
package ports

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/register"
	cmdutil "github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

// Store contains the dependencies needed to resolve both managed instances and
// registered compute nodes.
type Store interface {
	cmdutil.WorkspaceOrNodeResolver
}

// PortInfo is the stable JSON representation of a port mapping.
type PortInfo struct {
	PortID                     string   `json:"port_id"`
	Endpoint                   string   `json:"endpoint"`
	PublicPort                 int32    `json:"public_port"`
	DestinationPort            int32    `json:"destination_port"`
	Protocol                   string   `json:"protocol"`
	AllowedSources             []string `json:"allowed_sources"`
	AuthorizedEmails           []string `json:"authorized_emails"`
	AllowPublicUnauthenticated bool     `json:"allow_public_unauthenticated"`
	Type                       string   `json:"type"`
	isHTTP                     bool
}

// NewCmdPorts creates the `brev ports` command group.
func NewCmdPorts(portStore Store) *cobra.Command {
	cmd := &cobra.Command{
		Annotations: map[string]string{"access": ""},
		Use:         "ports",
		Short:       "[beta] Manage ports for an instance or external node",
		Args:        cmderrors.TransformToValidationError(cobra.NoArgs),
		Example: `
  brev ports ls my-instance
  brev ports ls my-node --json`,
	}
	cmd.AddCommand(NewCmdPortsLs(portStore), NewCmdCreatePort(portStore))
	return cmd
}

// NewCmdPortsLs creates the `brev ports ls` command.
func NewCmdPortsLs(portStore Store) *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"access": ""},
		Use:                   "ls <instance-or-node>",
		DisableFlagsInUseLine: true,
		Short:                 "[beta] List Brev-managed ports for an instance or external node",
		Example: `
  brev ports ls my-instance
  brev ports ls my-node --json`,
		Args: cmderrors.TransformToValidationError(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := Run(cmd.Context(), cmd.OutOrStdout(), portStore, args[0], jsonOutput); err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	return cmd
}

// Run resolves a managed instance or registered compute node and displays its ports.
func Run(ctx context.Context, out io.Writer, portStore Store, nameOrID string, jsonOutput bool) error {
	target, err := cmdutil.ResolveWorkspaceOrNodeWithContext(ctx, portStore, nameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	var apiPorts []*devplanev1.Port
	if target.Workspace != nil {
		client := register.NewEnvironmentServiceClient(portStore, config.GlobalConfig.GetBrevPublicAPIURL())
		resp, err := client.GetNetworkInfo(ctx, connect.NewRequest(&devplanev1.EnvironmentServiceGetNetworkInfoRequest{
			EnvironmentId: target.Workspace.ID,
		}))
		if err != nil {
			return breverrors.WrapAndTrace(fmt.Errorf("get ports for instance %q: %w", nameOrID, err))
		}
		var networkInfo *devplanev1.EnvironmentNetworkInfo
		if resp != nil && resp.Msg != nil {
			networkInfo = resp.Msg.GetNetworkInfo()
		}
		// A connected or disconnected member with no ports is a valid empty result.
		// The API uses an unspecified status and no ports when no Brev-managed network is available.
		if networkInfo == nil ||
			(networkInfo.GetStatus() == devplanev1.NetworkMemberStatus_NETWORK_MEMBER_STATUS_UNSPECIFIED &&
				len(networkInfo.GetPorts()) == 0) {
			return breverrors.NewValidationError(fmt.Sprintf(
				"cannot list ports for instance %q: no Brev-managed network configuration is available; "+
					"the instance may still be provisioning or may use legacy network access. "+
					"Try again when it is running, or view legacy secure links and firewall rules in the Brev console",
				nameOrID,
			))
		}
		apiPorts = networkInfo.GetPorts()
	} else if target.Node != nil {
		apiPorts = target.Node.GetPorts()
	}
	portInfos := toPortInfos(apiPorts)
	if jsonOutput {
		return writeJSON(out, portInfos)
	}
	return displayTables(out, nameOrID, portInfos)
}

func toPortInfos(apiPorts []*devplanev1.Port) []PortInfo {
	portInfos := make([]PortInfo, 0, len(apiPorts))
	for _, port := range apiPorts {
		if port == nil {
			continue
		}
		isHTTP := port.GetHttpProtocol() != devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_UNSPECIFIED
		portInfos = append(portInfos, PortInfo{
			PortID:                     port.GetPortId(),
			Endpoint:                   endpoint(port, isHTTP),
			PublicPort:                 port.GetPortNumber(),
			DestinationPort:            port.GetServerPort(),
			Protocol:                   protocolLabel(port, isHTTP),
			AllowedSources:             append([]string{}, port.GetAllowedSources()...),
			AuthorizedEmails:           append([]string{}, port.GetAuthorizedEmails()...),
			AllowPublicUnauthenticated: port.GetAllowPublicUnauthenticated(),
			Type:                       portTypeLabel(port.GetType()),
			isHTTP:                     isHTTP,
		})
	}
	return portInfos
}

func endpoint(port *devplanev1.Port, isHTTP bool) string {
	hostname := port.GetHostname()
	if hostname == "" {
		return ""
	}
	if isHTTP {
		return "https://" + hostname
	}
	if port.GetPortNumber() == 0 {
		return hostname
	}
	return net.JoinHostPort(hostname, strconv.Itoa(int(port.GetPortNumber())))
}

func protocolLabel(port *devplanev1.Port, isHTTP bool) string {
	if isHTTP {
		switch port.GetHttpProtocol() {
		case devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_HTTP:
			return "HTTP"
		case devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_HTTPS:
			return "HTTPS"
		default:
			return "UNKNOWN"
		}
	}

	switch port.GetProtocol() {
	case devplanev1.PortProtocol_PORT_PROTOCOL_SSH:
		return "SSH"
	case devplanev1.PortProtocol_PORT_PROTOCOL_TCP:
		return "TCP"
	case devplanev1.PortProtocol_PORT_PROTOCOL_UDP:
		return "UDP"
	default:
		return "UNKNOWN"
	}
}

func portTypeLabel(portType devplanev1.PortType) string {
	switch portType {
	case devplanev1.PortType_PORT_TYPE_UNSPECIFIED:
		return "unspecified"
	case devplanev1.PortType_PORT_TYPE_SYSTEM:
		return "system"
	case devplanev1.PortType_PORT_TYPE_USER:
		return "user"
	default:
		return "unknown"
	}
}

func writeJSON(out io.Writer, portInfos []PortInfo) error {
	encoded, err := json.MarshalIndent(portInfos, "", "  ")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	_, err = fmt.Fprintln(out, string(encoded))
	return breverrors.WrapAndTrace(err)
}

func displayTables(out io.Writer, nameOrID string, portInfos []PortInfo) error {
	if len(portInfos) == 0 {
		_, err := fmt.Fprintf(out, "No ports are open on %s.\n", nameOrID)
		return breverrors.WrapAndTrace(err)
	}

	httpPorts := make([]PortInfo, 0, len(portInfos))
	networkPorts := make([]PortInfo, 0, len(portInfos))
	for _, port := range portInfos {
		if port.isHTTP {
			httpPorts = append(httpPorts, port)
		} else {
			networkPorts = append(networkPorts, port)
		}
	}

	if len(httpPorts) > 0 {
		if _, err := fmt.Fprintln(out, "HTTP APPLICATIONS"); err != nil {
			return breverrors.WrapAndTrace(err)
		}
		displayHTTPTable(out, httpPorts)
	}
	if len(httpPorts) > 0 && len(networkPorts) > 0 {
		if _, err := fmt.Fprintln(out); err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	if len(networkPorts) > 0 {
		if _, err := fmt.Fprintln(out, "NETWORK PORTS"); err != nil {
			return breverrors.WrapAndTrace(err)
		}
		displayNetworkTable(out, networkPorts)
	}
	return nil
}

func displayHTTPTable(out io.Writer, portInfos []PortInfo) {
	tw := newTable(out)
	tw.AppendHeader(table.Row{"ENDPOINT", "AUTHORIZATION", "IP RESTRICTIONS", "PUBLIC PORT", "DESTINATION PORT", "PROTOCOL"})
	for _, port := range portInfos {
		tw.AppendRow(table.Row{
			valueOrDash(port.Endpoint),
			authorizationLabel(port),
			allowedSourcesLabel(port.AllowedSources),
			portNumberLabel(port.PublicPort),
			portNumberLabel(port.DestinationPort),
			port.Protocol,
		})
	}
	tw.Render()
}

func displayNetworkTable(out io.Writer, portInfos []PortInfo) {
	tw := newTable(out)
	tw.AppendHeader(table.Row{"ENDPOINT", "IP RESTRICTIONS", "PUBLIC PORT", "DESTINATION PORT", "PROTOCOL"})
	for _, port := range portInfos {
		tw.AppendRow(table.Row{
			valueOrDash(port.Endpoint),
			allowedSourcesLabel(port.AllowedSources),
			portNumberLabel(port.PublicPort),
			portNumberLabel(port.DestinationPort),
			port.Protocol,
		})
	}
	tw.Render()
}

func newTable(out io.Writer) table.Writer {
	tw := table.NewWriter()
	tw.SetOutputMirror(out)
	options := table.OptionsDefault
	options.DrawBorder = false
	options.SeparateColumns = false
	options.SeparateRows = false
	options.SeparateHeader = false
	tw.Style().Options = options
	return tw
}

func authorizationLabel(port PortInfo) string {
	if port.AllowPublicUnauthenticated {
		return "Public"
	}
	if len(port.AuthorizedEmails) > 0 {
		return strings.Join(port.AuthorizedEmails, ", ")
	}
	return "-"
}

func allowedSourcesLabel(allowedSources []string) string {
	if len(allowedSources) == 0 {
		return "Anywhere"
	}
	allAnywhere := true
	for _, source := range allowedSources {
		if source != "0.0.0.0/0" {
			allAnywhere = false
			break
		}
	}
	if allAnywhere {
		return "Anywhere"
	}
	return strings.Join(allowedSources, ", ")
}

func portNumberLabel(port int32) string {
	if port == 0 {
		return "-"
	}
	return strconv.Itoa(int(port))
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
