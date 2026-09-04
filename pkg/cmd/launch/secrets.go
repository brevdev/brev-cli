package launch

import (
	"context"

	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
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
	id, err := r.client.LatestVersion(ctx, secretID)
	if err != nil {
		breverrors.WrapAndTrace(err)
	}
	return id, nil
}

func (r devplaneManagedSecretResolver) GetVersionIDForVersionNumber(ctx context.Context, secretID string, versionNumber int64) (string, error) {
	id, err := r.client.GetVersionIDForVersionNumber(ctx, secretID, versionNumber)
	if err != nil {
		breverrors.WrapAndTrace(err)
	}
	return id, nil
}

func (r devplaneManagedSecretResolver) GetValue(ctx context.Context, ref store.ManagedSecretReference) (string, error) {
	value, err := r.client.Value(ctx, ref.SecretID, ref.VersionID)
	if err != nil {
		breverrors.WrapAndTrace(err)
	}
	return value, nil
}
