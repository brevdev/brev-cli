package launch

import (
	"context"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/managedsecret"
	"github.com/brevdev/brev-cli/pkg/store"
)

type managedSecretResolver interface {
	GetLatestVersionID(ctx context.Context, secretID string) (string, error)
	GetVersionIDForVersionNumber(ctx context.Context, secretID string, versionNumber int64) (string, error)
	GetValue(ctx context.Context, ref store.ManagedSecretReference) (string, error)
}

type devplaneManagedSecretResolver struct {
	client managedsecret.Client
}

func newDevplaneManagedSecretResolver(provider Store) managedSecretResolver {
	return devplaneManagedSecretResolver{
		client: managedsecret.NewClient(
			register.NewManagedSecretServiceClient(provider, config.GlobalConfig.GetBrevPublicAPIURL()),
		),
	}
}

func (r devplaneManagedSecretResolver) GetLatestVersionID(ctx context.Context, secretID string) (string, error) {
	return r.client.LatestVersion(ctx, secretID)
}

func (r devplaneManagedSecretResolver) GetVersionIDForVersionNumber(ctx context.Context, secretID string, versionNumber int64) (string, error) {
	return r.client.GetVersionIDForVersionNumber(ctx, secretID, versionNumber)
}

func (r devplaneManagedSecretResolver) GetValue(ctx context.Context, ref store.ManagedSecretReference) (string, error) {
	return r.client.Value(ctx, ref.SecretID, ref.VersionID)
}
