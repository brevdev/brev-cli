package store

import (
	"net/http"

	devplanev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"

	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

// DevPlaneClient bundles dev-plane's ConnectRPC service clients over a single HTTP client
type DevPlaneClient struct {
	Instances      devplanev1connect.InstanceServiceClient
	Environments   devplanev1connect.EnvironmentServiceClient
	Organizations  devplanev1connect.OrganizationServiceClient
	ExternalNodes  devplanev1connect.ExternalNodeServiceClient
	ManagedSecrets devplanev1connect.ManagedSecretServiceClient
}

// NewAuthenticatedHTTPClient builds the HTTP client shared by every authenticated
// dev-plane ConnectRPC service. It injects the bearer token from auth and the CLI
// attribution params on each request.
func NewAuthenticatedHTTPClient(auth Auth) *http.Client {
	return &http.Client{Transport: &authenticatedTransport{provider: auth, base: http.DefaultTransport}}
}

// NewPublicHTTPClient builds the HTTP client used by unauthenticated dev-plane
// public routes; it only adds the CLI attribution params.
func NewPublicHTTPClient() *http.Client {
	return &http.Client{Transport: attributionTransport{base: http.DefaultTransport}}
}

// NewDevPlaneClient builds an authenticated bundle over one shared HTTP client.
func NewDevPlaneClient(auth Auth, baseURL string) *DevPlaneClient {
	return newDevPlaneClient(NewAuthenticatedHTTPClient(auth), baseURL)
}

// NewPublicDevPlaneClient builds an unauthenticated bundle for dev-plane's public routes.
func NewPublicDevPlaneClient(baseURL string) *DevPlaneClient {
	return newDevPlaneClient(NewPublicHTTPClient(), baseURL)
}

func newDevPlaneClient(httpClient *http.Client, baseURL string) *DevPlaneClient {
	return &DevPlaneClient{
		Instances:      devplanev1connect.NewInstanceServiceClient(httpClient, baseURL),
		Environments:   devplanev1connect.NewEnvironmentServiceClient(httpClient, baseURL),
		Organizations:  devplanev1connect.NewOrganizationServiceClient(httpClient, baseURL),
		ExternalNodes:  devplanev1connect.NewExternalNodeServiceClient(httpClient, baseURL),
		ManagedSecrets: devplanev1connect.NewManagedSecretServiceClient(httpClient, baseURL),
	}
}

// DevPlane returns the store's authenticated dev-plane client bundle. It is
// created with the store so commands and store methods share one HTTP client.
func (s *AuthHTTPStore) DevPlane() *DevPlaneClient {
	if s.devPlaneClient != nil {
		return s.devPlaneClient
	}
	// Stores constructed as literals (e.g. tests) fall back to a fresh bundle.
	return NewDevPlaneClient(s, config.GlobalConfig.GetBrevPublicAPIURL())
}

type authenticatedTransport struct {
	provider Auth
	base     http.RoundTripper
}

func (t *authenticatedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.provider.GetAccessToken()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+token)
	AddCLIAttributionParams(req)
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return resp, nil
}
