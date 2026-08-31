package launch

import (
	"context"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/managedsecret"
	"github.com/brevdev/brev-cli/pkg/store"
)

type managedSecretResolver interface {
	LatestVersion(ctx context.Context, secretID string) (string, error)
	Value(ctx context.Context, ref store.ManagedSecretReference) (string, error)
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

func (r devplaneManagedSecretResolver) LatestVersion(ctx context.Context, secretID string) (string, error) {
	return r.client.LatestVersion(ctx, secretID)
}

func (r devplaneManagedSecretResolver) Value(ctx context.Context, ref store.ManagedSecretReference) (string, error) {
	return r.client.Value(ctx, ref.SecretID, ref.VersionID)
}
