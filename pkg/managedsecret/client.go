// Package managedsecret provides an authenticated client for DevPlane managed secrets.
package managedsecret

import (
	"context"
	"fmt"

	devplanev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
)

// Client exposes the DevPlane managed-secret API and common read operations.
type Client struct {
	devplanev1connect.ManagedSecretServiceClient
}

// NewClient adds managed-secret helpers to an API client.
func NewClient(client devplanev1connect.ManagedSecretServiceClient) Client {
	return Client{ManagedSecretServiceClient: client}
}

func (c Client) LatestVersion(ctx context.Context, secretID string) (string, error) {
	response, err := c.GetSecret(ctx, connect.NewRequest(&devplanev1.ManagedSecretServiceGetSecretRequest{
		SecretId: secretID,
	}))
	if err != nil {
		return "", fmt.Errorf("get managed secret metadata: %w", err)
	}
	if response.Msg.GetSecret() == nil || response.Msg.GetSecret().GetLatestVersionId() == "" {
		return "", fmt.Errorf("managed secret has no latest version")
	}
	return response.Msg.GetSecret().GetLatestVersionId(), nil
}

func (c Client) Value(ctx context.Context, secretID string, versionID string) (string, error) {
	response, err := c.GetSecretValue(ctx, connect.NewRequest(&devplanev1.ManagedSecretServiceGetSecretValueRequest{
		SecretId:  secretID,
		VersionId: versionID,
	}))
	if err != nil {
		return "", fmt.Errorf("get managed secret value: %w", err)
	}
	return response.Msg.GetValue(), nil
}
