package store

import (
	"fmt"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	resty "github.com/go-resty/resty/v2"
)

type NoAuthHTTPStore struct {
	FileStore
	noAuthHTTPClient *NoAuthHTTPClient
}

func (f *FileStore) WithNoAuthHTTPClient(c *NoAuthHTTPClient) *NoAuthHTTPStore {
	return &NoAuthHTTPStore{*f, c}
}

type NoAuthHTTPClient struct {
	restyClient *resty.Client
}

func NewNoAuthHTTPClient(brevAPIURL string) *NoAuthHTTPClient {
	restyClient := NewRestyClient(brevAPIURL)
	return &NoAuthHTTPClient{restyClient}
}

func NewRestyClient(brevAPIURL string) *resty.Client {
	restyClient := resty.New()
	restyClient.SetBaseURL(brevAPIURL)
	restyClient.SetQueryParam("utm_source", "cli")
	return restyClient
}

type AuthHTTPStore struct {
	NoAuthHTTPStore
	authHTTPClient *AuthHTTPClient
}

func (f *FileStore) WithAuthHTTPClient(c *AuthHTTPClient) *AuthHTTPStore {
	na := f.WithNoAuthHTTPClient(NewNoAuthHTTPClient(c.restyClient.BaseURL))
	return &AuthHTTPStore{*na, c}
}

func (n *NoAuthHTTPStore) WithAuthHTTPClient(c *AuthHTTPClient) *AuthHTTPStore {
	return &AuthHTTPStore{*n, c}
}

func (n *NoAuthHTTPStore) WithAuth(auth Auth) *AuthHTTPStore {
	return n.WithAuthHTTPClient(NewAuthHTTPClient(auth, n.noAuthHTTPClient.restyClient.BaseURL))
}

type AuthHTTPClient struct {
	restyClient *resty.Client
	auth        Auth
}

type Auth interface {
	GetAccessToken() (string, error)
}

func NewAuthHTTPClient(auth Auth, brevAPIURL string) *AuthHTTPClient {
	restyClient := NewRestyClient(brevAPIURL)
	restyClient.OnBeforeRequest(func(c *resty.Client, r *resty.Request) error {
		token, err := auth.GetAccessToken()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		r.SetAuthToken(token)
		return nil
	})
	return &AuthHTTPClient{restyClient, auth}
}

type HTTPResponseError struct {
	response *resty.Response
}

func NewHTTPResponseError(response *resty.Response) *HTTPResponseError {
	return &HTTPResponseError{
		response: response,
	}
}

func (e HTTPResponseError) Error() string {
	return fmt.Sprintf("%s %s", e.response.Request.URL, e.response.Status())
}
