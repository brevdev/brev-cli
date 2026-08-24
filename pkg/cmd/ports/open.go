package ports

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"regexp"
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

// NewCmdCreatePort creates the `brev ports create` command.
func NewCmdCreatePort(portStore Store) *cobra.Command {
	var opts openOptions

	cmd := &cobra.Command{
		Annotations:           map[string]string{"access": ""},
		Use:                   "create <instance-or-node> <port>",
		Aliases:               []string{"open", "add"},
		Hidden:                true,
		DisableFlagsInUseLine: true,
		Short:                 "[beta] Create a public port on an instance or external node",
		Example: "\n  brev ports create my-instance 8080" +
			"\n  brev ports create my-node 53 --protocol udp" +
			"\n  brev ports create my-instance 8080 --allow 203.0.113.10/32" +
			"\n  brev ports create my-instance 3000 --protocol http --public",
		Args: cmderrors.TransformToValidationError(cobra.ExactArgs(2)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runOpenCommand(cmd.Context(), cmd.OutOrStdout(), portStore, args[0], args[1], opts)
		},
	}

	cmd.Flags().StringVar(&opts.protocol, "protocol", "tcp", "port protocol (tcp, udp, ssh, http, or https)")
	cmd.Flags().StringArrayVar(&opts.allowedSources, "allow", nil, "source CIDR allowed to connect (repeatable; omit to allow all)")
	cmd.Flags().StringArrayVar(&opts.authorizedEmails, "authorize", nil, "email authorized for an HTTP port (repeatable; defaults to you)")
	cmd.Flags().StringVar(&opts.customHostname, "hostname", "", "hostname prefix for an HTTP port (defaults to the destination port)")
	cmd.Flags().BoolVar(&opts.allowPublicUnauthenticated, "public", false, "disable authentication for an HTTP port")
	cmd.Flags().BoolVar(&opts.jsonOutput, "json", false, "output the created port as JSON")
	_ = cmd.RegisterFlagCompletionFunc("protocol", cobra.FixedCompletions(
		[]string{"tcp", "udp", "ssh", "http", "https"},
		cobra.ShellCompDirectiveNoFileComp,
	))

	return cmd
}

type openOptions struct {
	protocol                   string
	allowedSources             []string
	authorizedEmails           []string
	customHostname             string
	allowPublicUnauthenticated bool
	jsonOutput                 bool
}

func runOpenCommand(
	ctx context.Context,
	out io.Writer,
	portStore Store,
	nameOrID string,
	portValue string,
	opts openOptions,
) error {
	portNumber, err := parsePortNumber(portValue)
	if err != nil {
		return err
	}
	if isHTTPProtocol(opts.protocol) {
		return runOpenHTTPCommand(ctx, out, portStore, nameOrID, portNumber, opts)
	}
	return runOpenNetworkCommand(ctx, out, portStore, nameOrID, portNumber, opts)
}

func runOpenHTTPCommand(
	ctx context.Context,
	out io.Writer,
	portStore Store,
	nameOrID string,
	portNumber int32,
	opts openOptions,
) error {
	httpProtocol, err := parseHTTPProtocol(opts.protocol)
	if err != nil {
		return err
	}
	if len(opts.allowedSources) > 0 {
		return breverrors.NewValidationError("--allow is only supported for tcp, udp, and ssh ports")
	}
	authorizedEmails, err := normalizeAuthorizedEmails(opts.authorizedEmails)
	if err != nil {
		return err
	}
	if opts.allowPublicUnauthenticated && len(authorizedEmails) > 0 {
		return breverrors.NewValidationError("--public and --authorize cannot be used together")
	}
	if err := validateHTTPHostname(opts.customHostname); err != nil {
		return err
	}
	return breverrors.WrapAndTrace(OpenHTTP(
		ctx, out, portStore, nameOrID, portNumber, httpProtocol, opts.customHostname,
		authorizedEmails, opts.allowPublicUnauthenticated, opts.jsonOutput,
	))
}

func runOpenNetworkCommand(
	ctx context.Context,
	out io.Writer,
	portStore Store,
	nameOrID string,
	portNumber int32,
	opts openOptions,
) error {
	if opts.customHostname != "" || len(opts.authorizedEmails) > 0 || opts.allowPublicUnauthenticated {
		return breverrors.NewValidationError("--hostname, --authorize, and --public are only supported for http and https ports")
	}
	portProtocol, err := parseProtocol(opts.protocol)
	if err != nil {
		return err
	}
	allowedSources, err := normalizeAllowedSources(opts.allowedSources)
	if err != nil {
		return err
	}
	return breverrors.WrapAndTrace(Open(
		ctx, out, portStore, nameOrID, portNumber, portProtocol, allowedSources, opts.jsonOutput,
	))
}

// OpenHTTP resolves a managed instance or registered compute node and creates
// an authenticated or public HTTP application endpoint.
func OpenHTTP(
	ctx context.Context,
	out io.Writer,
	portStore Store,
	nameOrID string,
	portNumber int32,
	httpProtocol devplanev1.HttpPortProtocol,
	customHostname string,
	authorizedEmails []string,
	allowPublicUnauthenticated bool,
	jsonOutput bool,
) error {
	target, err := cmdutil.ResolveWorkspaceOrNodeWithContext(ctx, portStore, nameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if !allowPublicUnauthenticated && len(authorizedEmails) == 0 {
		user, err := portStore.GetCurrentUser()
		if err != nil {
			return fmt.Errorf("get current user for HTTP port authorization: %w", err)
		}
		if user == nil || strings.TrimSpace(user.Email) == "" {
			return breverrors.NewValidationError("could not determine your email; use --authorize or --public")
		}
		authorizedEmails = []string{strings.TrimSpace(user.Email)}
	}

	var openedPort *devplanev1.Port
	if target.Workspace != nil {
		hostname, err := buildHTTPHostname(customHostname, portNumber, target.Workspace.ID)
		if err != nil {
			return err
		}
		client := register.NewEnvironmentServiceClient(portStore, config.GlobalConfig.GetBrevPublicAPIURL())
		resp, err := client.OpenHTTPPort(ctx, connect.NewRequest(&devplanev1.EnvironmentServiceOpenHTTPPortRequest{
			EnvironmentId:              target.Workspace.ID,
			PortNumber:                 portNumber,
			CustomHostname:             hostname,
			HttpProtocol:               httpProtocol,
			AuthorizedEmails:           authorizedEmails,
			AllowPublicUnauthenticated: allowPublicUnauthenticated,
		}))
		if err != nil {
			return fmt.Errorf("open HTTP port on instance %q: %w", nameOrID, err)
		}
		if resp != nil {
			openedPort = resp.Msg.GetPort()
		}
	} else if target.Node != nil {
		hostname, err := buildHTTPHostname(customHostname, portNumber, target.Node.GetExternalNodeId())
		if err != nil {
			return err
		}
		client := register.NewNodeServiceClient(portStore, config.GlobalConfig.GetBrevPublicAPIURL())
		resp, err := client.OpenHTTPPort(ctx, connect.NewRequest(&devplanev1.OpenHTTPPortRequest{
			ExternalNodeId:             target.Node.GetExternalNodeId(),
			PortNumber:                 portNumber,
			CustomHostname:             hostname,
			HttpProtocol:               httpProtocol,
			AuthorizedEmails:           authorizedEmails,
			AllowPublicUnauthenticated: allowPublicUnauthenticated,
		}))
		if err != nil {
			return fmt.Errorf("open HTTP port on external node %q: %w", nameOrID, err)
		}
		if resp != nil {
			openedPort = resp.Msg.GetPort()
		}
	}

	if openedPort == nil {
		return fmt.Errorf("open HTTP port on %q: API returned no port", nameOrID)
	}
	return writeOpenResult(out, nameOrID, openedPort, jsonOutput)
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
	target, err := cmdutil.ResolveWorkspaceOrNodeWithContext(ctx, portStore, nameOrID)
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
			fmt.Errorf("invalid protocol %q: must be tcp, udp, ssh, http, or https", value)
	}
}

func isHTTPProtocol(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "http" || value == "https"
}

func parseHTTPProtocol(value string) (devplanev1.HttpPortProtocol, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "http":
		return devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_HTTP, nil
	case "https":
		return devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_HTTPS, nil
	default:
		return devplanev1.HttpPortProtocol_HTTP_PORT_PROTOCOL_UNSPECIFIED,
			fmt.Errorf("invalid HTTP protocol %q: must be http or https", value)
	}
}

func normalizeAllowedSources(values []string) ([]string, error) {
	normalized, err := normalizeUniqueValues(values, "allowed source")
	if err != nil {
		return nil, err
	}
	for _, value := range normalized {
		if _, _, err := net.ParseCIDR(value); err != nil {
			return nil, breverrors.NewValidationError(fmt.Sprintf(
				"invalid allowed source %q: must be a valid CIDR", value,
			))
		}
	}
	return normalized, nil
}

func normalizeAuthorizedEmails(values []string) ([]string, error) {
	return normalizeUniqueValues(values, "authorized email")
}

func normalizeUniqueValues(values []string, label string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("%s cannot be empty", label)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

var httpHostnamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)

func validateHTTPHostname(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if len(value) > 63 {
		return breverrors.NewValidationError("hostname must be 63 characters or fewer")
	}
	if !httpHostnamePattern.MatchString(value) {
		return breverrors.NewValidationError("hostname must contain only lowercase letters, digits, and hyphens, and must start and end with a letter or digit")
	}
	return nil
}

func buildHTTPHostname(value string, portNumber int32, targetID string) (string, error) {
	hostname := strings.TrimSpace(value)
	if hostname == "" {
		hostname = strconv.Itoa(int(portNumber))
	}
	suffix := "-" + strings.ToLower(strings.TrimSpace(targetID))
	if targetID != "" && !strings.HasSuffix(hostname, suffix) {
		hostname += suffix
	}
	if err := validateHTTPHostname(hostname); err != nil {
		return "", err
	}
	return hostname, nil
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

	_, err := fmt.Fprintf(out, "Created %s port %d on %s.\n", portInfo.Protocol, port.GetServerPort(), nameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return displayTables(out, nameOrID, []PortInfo{portInfo})
}
