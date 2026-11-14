package ports

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

// NewCmdGetPort creates the `brev ports get` command.
func NewCmdGetPort(portStore Store) *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"networking": ""},
		Use:                   "get <port-id>",
		DisableFlagsInUseLine: true,
		Short:                 "Get a Brev-managed port by ID",
		Example: `
  brev ports get nport-abc123
  brev ports get nport-abc123 --json`,
		Args: cmderrors.TransformToValidationError(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return breverrors.WrapAndTrace(Get(
				cmd.Context(), cmd.OutOrStdout(), portStore, args[0], jsonOutput,
			))
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	return cmd
}

// Get resolves a port by its exact ID and displays all of its data.
func Get(
	ctx context.Context,
	out io.Writer,
	portStore Store,
	portID string,
	jsonOutput bool,
) error {
	_, port, err := resolvePortID(ctx, portStore, portID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	portInfo := toPortInfos([]*devplanev1.Port{port})[0]
	if jsonOutput {
		return writePortJSON(out, portInfo)
	}
	displayPortDetails(out, portInfo)
	return nil
}

func writePortJSON(out io.Writer, portInfo PortInfo) error {
	encoded, err := json.MarshalIndent(portInfo, "", "  ")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	_, err = fmt.Fprintln(out, string(encoded))
	return breverrors.WrapAndTrace(err)
}

func displayPortDetails(out io.Writer, port PortInfo) {
	tw := newTable(out)
	tw.AppendHeader(table.Row{"FIELD", "VALUE"})
	tw.AppendRows([]table.Row{
		{"ID", valueOrDash(port.PortID)},
		{"ENDPOINT", valueOrDash(port.Endpoint)},
		{"PUBLIC PORT", portNumberLabel(port.PublicPort)},
		{"DESTINATION PORT", portNumberLabel(port.DestinationPort)},
		{"PROTOCOL", port.Protocol},
		{"ALLOWED SOURCES", allowedSourcesLabel(port.AllowedSources)},
		{"AUTHORIZED EMAILS", stringSliceLabel(port.AuthorizedEmails)},
		{"PUBLIC UNAUTHENTICATED", strconv.FormatBool(port.AllowPublicUnauthenticated)},
		{"TYPE", port.Type},
	})
	tw.Render()
}

func stringSliceLabel(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ", ")
}
