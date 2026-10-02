package store

import (
	"context"

	"github.com/brevdev/brev-cli/pkg/environment"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

func (s *AuthHTTPStore) newEnvironmentClient() environment.Client {
	devPlane := s.DevPlane()
	return environment.NewClient(devPlane.Environments, devPlane.Organizations)
}

// GetEnvironmentStatus reads live environment lifecycle state directly from dev-plane,
// which is authoritative over the instance-derived status exposed by brev-deploy.
func (s *AuthHTTPStore) GetEnvironmentStatus(environmentID string) (environment.Status, error) {
	status, err := s.newEnvironmentClient().EnvironmentStatus(context.Background(), environmentID)
	if err != nil {
		return environment.Status{}, breverrors.WrapAndTrace(err)
	}
	return status, nil
}
