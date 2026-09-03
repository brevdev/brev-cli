package store

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"

	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

func (s *AuthHTTPStore) newManagedSecretServiceClient() nodev1connect.ManagedSecretServiceClient {
	return nodev1connect.NewManagedSecretServiceClient(
		&http.Client{Transport: &authHTTPStoreTransport{store: s, base: http.DefaultTransport}},
		config.GlobalConfig.GetBrevPublicAPIURL(),
	)
}

func (s *AuthHTTPStore) CreateManagedSecret(organizationID, name, value string) (*nodev1.ManagedSecret, error) {
	res, err := s.newManagedSecretServiceClient().CreateSecret(context.Background(), connect.NewRequest(
		&nodev1.ManagedSecretServiceCreateSecretRequest{
			Name:           name,
			Value:          value,
			OrganizationId: organizationID,
		}))
	if err != nil {
		return nil, breverrors.WrapAndTrace(connectErrToBrevErr(err))
	}
	return res.Msg.GetSecret(), nil
}

func (s *AuthHTTPStore) ListManagedSecrets(organizationID string) ([]*nodev1.ManagedSecret, error) {
	client := s.newManagedSecretServiceClient()
	var secrets []*nodev1.ManagedSecret
	var pageToken string
	for {
		res, err := client.ListSecrets(context.Background(), connect.NewRequest(
			&nodev1.ManagedSecretServiceListSecretsRequest{
				OrganizationId: organizationID,
				PageParams: &nodev1.PageParams{
					PageSize:  1000,
					PageToken: pageToken,
				},
			}))
		if err != nil {
			return nil, breverrors.WrapAndTrace(connectErrToBrevErr(err))
		}
		secrets = append(secrets, res.Msg.GetItems()...)
		pageToken = res.Msg.GetNextPageToken()
		if pageToken == "" {
			return secrets, nil
		}
	}
}

func (s *AuthHTTPStore) GetManagedSecretByName(organizationID, name string) (*nodev1.ManagedSecret, error) {
	secrets, err := s.ListManagedSecrets(organizationID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	for _, secret := range secrets {
		if secret.GetName() == name {
			return secret, nil
		}
	}
	return nil, breverrors.NewValidationError(fmt.Sprintf("secret %q not found", name))
}

func (s *AuthHTTPStore) GetManagedSecret(secretID string) (*nodev1.ManagedSecret, error) {
	res, err := s.newManagedSecretServiceClient().GetSecret(context.Background(), connect.NewRequest(
		&nodev1.ManagedSecretServiceGetSecretRequest{
			SecretId: secretID,
		}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, breverrors.NewValidationError(fmt.Sprintf("secret %q not found", secretID))
		}
		return nil, breverrors.WrapAndTrace(err)
	}
	return res.Msg.GetSecret(), nil
}

func (s *AuthHTTPStore) ListManagedSecretVersions(secretID string) ([]*nodev1.ManagedSecretVersion, error) {
	client := s.newManagedSecretServiceClient()
	var versions []*nodev1.ManagedSecretVersion
	var pageToken string
	for {
		res, err := client.ListSecretVersions(context.Background(), connect.NewRequest(
			&nodev1.ManagedSecretServiceListSecretVersionsRequest{
				SecretId: secretID,
				PageParams: &nodev1.PageParams{
					PageSize:  1000,
					PageToken: pageToken,
				},
			}))
		if err != nil {
			return nil, breverrors.WrapAndTrace(connectErrToBrevErr(err))
		}
		versions = append(versions, res.Msg.GetItems()...)
		pageToken = res.Msg.GetNextPageToken()
		if pageToken == "" {
			return versions, nil
		}
	}
}

func (s *AuthHTTPStore) GetManagedSecretValue(secretID, versionID string) (string, error) {
	res, err := s.newManagedSecretServiceClient().GetSecretValue(context.Background(), connect.NewRequest(
		&nodev1.ManagedSecretServiceGetSecretValueRequest{
			SecretId:  secretID,
			VersionId: versionID,
		}))
	if err != nil {
		return "", breverrors.WrapAndTrace(connectErrToBrevErr(err))
	}
	return res.Msg.GetValue(), nil
}

func (s *AuthHTTPStore) SetManagedSecretValue(secretID, value string) (*nodev1.ManagedSecretVersion, error) {
	res, err := s.newManagedSecretServiceClient().SetSecretValue(context.Background(), connect.NewRequest(
		&nodev1.ManagedSecretServiceSetSecretValueRequest{
			SecretId: secretID,
			Value:    value,
		}))
	if err != nil {
		return nil, breverrors.WrapAndTrace(connectErrToBrevErr(err))
	}
	return res.Msg.GetVersion(), nil
}

func (s *AuthHTTPStore) DeleteManagedSecret(secretID string) error {
	_, err := s.newManagedSecretServiceClient().DeleteSecret(context.Background(), connect.NewRequest(
		&nodev1.ManagedSecretServiceDeleteSecretRequest{
			SecretId: secretID,
		}))
	if err != nil {
		return breverrors.WrapAndTrace(connectErrToBrevErr(err))
	}
	return nil
}

// connectErrToBrevErr converts connect to validation errors, which print cleanly without a stack trace.
func connectErrToBrevErr(err error) error {
	var connectErr *connect.Error
	if errors.As(err, &connectErr) && connectErr.Code() == connect.CodeNotFound {
		return breverrors.NewValidationError(connectErr.Message())
	}
	return err
}
