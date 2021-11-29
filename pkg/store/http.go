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

// Used if need new instance to customize settings
func (n NoAuthHTTPStore) NewNoAuthHTTPStore() *NoAuthHTTPStore {
	return n.WithNoAuthHTTPClient(NewNoAuthHTTPClient(n.noAuthHTTPClient.restyClient.BaseURL))
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
	authHTTPClient           *AuthHTTPClient
	isRefreshTokenHandlerSet bool
}

func (f *FileStore) WithAuthHTTPClient(c *AuthHTTPClient) *AuthHTTPStore {
	na := f.WithNoAuthHTTPClient(NewNoAuthHTTPClient(c.restyClient.BaseURL))
	return &AuthHTTPStore{NoAuthHTTPStore: *na, authHTTPClient: c}
}

func (n *NoAuthHTTPStore) WithAuthHTTPClient(c *AuthHTTPClient) *AuthHTTPStore {
	return &AuthHTTPStore{NoAuthHTTPStore: *n, authHTTPClient: c}
}

func (n *NoAuthHTTPStore) WithAuth(auth Auth) *AuthHTTPStore {
	return n.WithAuthHTTPClient(NewAuthHTTPClient(auth, n.noAuthHTTPClient.restyClient.BaseURL))
}

// Used if need new instance to customize settings
func (s AuthHTTPStore) NewAuthHTTPStore() *AuthHTTPStore {
	return s.WithAuth(s.authHTTPClient.auth)
}

func (s *AuthHTTPStore) SetRefreshTokenHandler(handler func() (string, error)) error {
	if s.isRefreshTokenHandlerSet { // need to create a way to idempotently set this
		return fmt.Errorf("refresh token handler alreay set")
	}
	attemptsThresh := 1
	s.authHTTPClient.restyClient.OnAfterResponse(func(c *resty.Client, r *resty.Response) error {
		if r.StatusCode() == 403 && r.Request.Attempt < attemptsThresh+1 {
			token, err := handler()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			c.SetAuthToken(token)
		}
		return nil
	})
	s.authHTTPClient.restyClient.AddRetryCondition(
		func(r *resty.Response, e error) bool {
			return r.StatusCode() == 403
		})
	s.authHTTPClient.restyClient.SetRetryCount(attemptsThresh)

	s.isRefreshTokenHandlerSet = true
	return nil
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
