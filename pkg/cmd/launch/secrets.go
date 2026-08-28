package launch

import (
	"context"
	"fmt"

	devplanev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	devplanev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/store"
)

type managedSecretResolver interface {
	LatestVersion(ctx context.Context, secretID string) (string, error)
	Value(ctx context.Context, ref store.ManagedSecretReference) (string, error)
}

type devplaneManagedSecretResolver struct {
	client devplanev1connect.ManagedSecretServiceClient
}

func newDevplaneManagedSecretResolver(provider Store) managedSecretResolver {
	return devplaneManagedSecretResolver{
		client: register.NewManagedSecretServiceClient(provider, config.GlobalConfig.GetBrevPublicAPIURL()),
	}
}

func (r devplaneManagedSecretResolver) LatestVersion(ctx context.Context, secretID string) (string, error) {
	response, err := r.client.GetSecret(ctx, connect.NewRequest(&devplanev1.ManagedSecretServiceGetSecretRequest{
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

func (r devplaneManagedSecretResolver) Value(ctx context.Context, ref store.ManagedSecretReference) (string, error) {
	response, err := r.client.GetSecretValue(ctx, connect.NewRequest(&devplanev1.ManagedSecretServiceGetSecretValueRequest{
		SecretId:  ref.SecretID,
		VersionId: ref.VersionID,
	}))
	if err != nil {
		return "", fmt.Errorf("get managed secret value: %w", err)
	}
	return response.Msg.GetValue(), nil
}
