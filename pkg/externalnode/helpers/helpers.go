package helpers

import (
	"context"
	"fmt"
	"strings"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

// SelectOrganizationInteractive prompts the user to select an organization from the list.
func SelectOrganizationInteractive(t *terminal.Terminal, organizations []entity.Organization, selector terminal.Selector) (*entity.Organization, error) {
	if len(organizations) == 0 {
		return nil, fmt.Errorf("no organization found; please create or join an organization first")
	}

	labels := make([]string, len(organizations))
	for i := range organizations {
		labels[i] = organizations[i].Name
	}

	t.Vprint("")
	chosenLabel := selector.Select("Select organization", labels)

	var selected *entity.Organization
	for i := range organizations {
		if organizations[i].Name == chosenLabel {
			selected = &organizations[i]
			break
		}
	}

	if selected == nil {
		return nil, fmt.Errorf("selected organization not found")
	}
	return selected, nil
}

type OrganizationGetter interface {
	GetOrganizationsByName(name string) ([]entity.Organization, error)
}

// ResolveOrgByName returns the single organization matching the given name.
func ResolveOrgByName(getter OrganizationGetter, orgName string) (*entity.Organization, error) {
	orgs, err := getter.GetOrganizationsByName(orgName)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if len(orgs) == 0 {
		return nil, fmt.Errorf("no organization found with name %q", orgName)
	}
	if len(orgs) > 1 {
		return nil, fmt.Errorf("multiple organizations found with name %q", orgName)
	}
	return &orgs[0], nil
}

// ResolveNodeByName returns the full node for the organization that matches the given name (case-insensitive).
// Use in non-interactive flows when the node is specified by name (e.g. --node my-node).
func ResolveNodeByName(ctx context.Context, nodeClient devplaneapiv1connect.ExternalNodeServiceClient, orgID string, nodeName string) (*nodev1.ExternalNode, error) {
	// Fetch the list of nodes in the organization
	resp, err := nodeClient.ListNodes(ctx, connect.NewRequest(&nodev1.ListNodesRequest{
		OrganizationId: orgID,
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var selected *nodev1.ExternalNode
	for _, n := range resp.Msg.GetItems() {
		if strings.EqualFold(n.GetName(), nodeName) {
			selected = n
			break
		}
	}

	if selected == nil {
		return nil, fmt.Errorf("no node found with name %q", nodeName)
	}
	return selected, nil
}
