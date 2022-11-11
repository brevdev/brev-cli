package store

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/version"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	resty "github.com/go-resty/resty/v2"
)

type NoAuthHTTPStore struct {
	FileStore
	noAuthHTTPClient *NoAuthHTTPClient
	BasicStore
}

func (f *FileStore) WithNoAuthHTTPClient(c *NoAuthHTTPClient) *NoAuthHTTPStore {
	return &NoAuthHTTPStore{*f, c, f.BasicStore}
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
	restyClient.SetQueryParam("cli_version", version.Version)
	restyClient.SetQueryParam("os", runtime.GOOS)
	return restyClient
}

type AuthHTTPStore struct {
	NoAuthHTTPStore
	authHTTPClient           *AuthHTTPClient
	isRefreshTokenHandlerSet bool
	BasicStore
}

func (n *NoAuthHTTPStore) GetWindowsDir() (string, error) {
	return n.FileStore.GetWSLHostHomeDir()
}

func (s *AuthHTTPStore) GetWindowsDir() (string, error) {
	return s.FileStore.GetWSLHostHomeDir()
}

func (f *FileStore) WithAuthHTTPClient(c *AuthHTTPClient) *AuthHTTPStore {
	// err never returned from GetCurrentWorkspaceID
	id, _ := f.GetCurrentWorkspaceID()
	if id == "" {
		c.restyClient.SetQueryParam("local", "true")
	}
	na := f.WithNoAuthHTTPClient(NewNoAuthHTTPClient(c.restyClient.BaseURL))
	return &AuthHTTPStore{NoAuthHTTPStore: *na, authHTTPClient: c}
}

func (n *NoAuthHTTPStore) WithAuthHTTPClient(c *AuthHTTPClient) *AuthHTTPStore {
	// err never returned from GetCurrentWorkspaceID
	id, _ := n.GetCurrentWorkspaceID()
	if id == "" {
		c.restyClient.SetQueryParam("local", "true")
	}
	return &AuthHTTPStore{NoAuthHTTPStore: *n, authHTTPClient: c}
}

func (n *NoAuthHTTPStore) WithAuth(auth Auth) *AuthHTTPStore {
	return n.WithAuthHTTPClient(NewAuthHTTPClient(auth, n.noAuthHTTPClient.restyClient.BaseURL))
}

// Used if need new instance to customize settings
func (s AuthHTTPStore) NewAuthHTTPStore() *AuthHTTPStore {
	return s.WithAuth(s.authHTTPClient.auth)
}

func (s *AuthHTTPStore) SetForbiddenStatusRetryHandler(handler func() error) error {
	if s.isRefreshTokenHandlerSet { // need to create a way to idempotently set this
		return fmt.Errorf("refresh token handler alreay set")
	}
	attemptsThresh := 1
	s.authHTTPClient.restyClient.OnAfterResponse(func(c *resty.Client, r *resty.Response) error {
		if r.StatusCode() == http.StatusForbidden && r.Request.Attempt < attemptsThresh+1 {
			err := handler()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
		}
		return nil
	})
	s.authHTTPClient.restyClient.AddRetryCondition(
		func(r *resty.Response, e error) bool {
			if e != nil {
				return false
			}
			return r.StatusCode() == http.StatusForbidden
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

func (s *AuthHTTPStore) WithStaticHeader(header string, value string) *AuthHTTPStore {
	s.authHTTPClient.restyClient.SetHeader(header, value)
	return s
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

type BrevDeployErrorList struct {
	Errors []BrevDeployError
}

type BrevDeployError struct {
	Kind    string `json:"type"`
	Message string `json:"message"`
}

type HTTPResponseError struct {
	Response *resty.Response
}

func NewHTTPResponseError(response *resty.Response) *HTTPResponseError {
	return &HTTPResponseError{
		Response: response,
	}
}

func (e HTTPResponseError) Error() string {
	body := e.Response.Body()
	if featureflag.Debug() {
		return fmt.Sprintf("%s %s %s", e.Response.Request.URL, e.Response.Status(), body)
	}
	errors := &BrevDeployErrorList{}
	err := json.Unmarshal(body, errors)
	if err != nil {
		return fmt.Sprintf("%s %s %s", e.Response.Request.URL, e.Response.Status(), body)
	}
	msg := ""
	for _, e := range errors.Errors {
		msg = msg + e.Message + "\n"
	}
	if strings.TrimSpace(msg) == "" {
		return fmt.Sprintf("%s %s %s", e.Response.Request.URL, e.Response.Status(), body)
	}
	return msg
}

func IsNetwork404Or403Error(err error) bool {
	return IsNetworkErrorWithStatus(err, []int{404, 403})
}

func IsNetworkErrorWithStatus(err error, statusCodes []int) bool {
	switch err := err.(type) {
	case *HTTPResponseError:
		statusCode := err.Response.StatusCode()
		for _, c := range statusCodes {
			if c == statusCode {
				return true
			}
		}
		return false
	default:
		return false
	}
}
