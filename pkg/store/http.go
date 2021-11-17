package store

import (
	"fmt"

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
	client *resty.Client
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
	na := f.WithNoAuthHTTPClient(NewNoAuthHTTPClient(c.restyClient.BaseURL)) // TODO pull from auth client
	return &AuthHTTPStore{*na, c}
}

func (n *NoAuthHTTPStore) WithAuthHTTPClient(c *AuthHTTPClient) *AuthHTTPStore {
	return &AuthHTTPStore{*n, c}
}

func (n *NoAuthHTTPStore) WithAccessToken(accessToken string) *AuthHTTPStore {
	return n.WithAuthHTTPClient(NewAuthHTTPClient(accessToken, n.noAuthHTTPClient.client.BaseURL))
}

type AuthHTTPClient struct {
	restyClient *resty.Client
}

func NewAuthHTTPClient(accessToken string, brevAPIURL string) *AuthHTTPClient {
	restyClient := NewRestyClient(brevAPIURL)
	restyClient.SetAuthToken(accessToken)
	return &AuthHTTPClient{restyClient}
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
