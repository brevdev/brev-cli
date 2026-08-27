package store

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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
	return &NoAuthHTTPStore{*f, c, f.b}
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
	return n.GetWSLHostHomeDir()
}

func (s *AuthHTTPStore) GetWindowsDir() (string, error) {
	return s.GetWSLHostHomeDir()
}

// GetAccessToken returns a fresh access token, refreshing if needed.
func (s *AuthHTTPStore) GetAccessToken() (string, error) {
	token, err := s.authHTTPClient.auth.GetAccessToken()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return token, nil
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

func (n *NoAuthHTTPStore) WithAuth(auth Auth, options ...Option) *AuthHTTPStore {
	return n.WithAuthHTTPClient(NewAuthHTTPClient(auth, n.noAuthHTTPClient.restyClient.BaseURL, options...))
}

// Used if need new instance to customize settings
func (s AuthHTTPStore) NewAuthHTTPStore(options ...Option) *AuthHTTPStore {
	return s.WithAuth(s.authHTTPClient.auth, options...)
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

type Options struct {
	Debug bool
}

type Option func(*Options)

func WithDebug(debug bool) Option {
	return func(o *Options) {
		o.Debug = debug
	}
}

// quietRestyLogger swallows resty's retry WARN/ERROR chatter for expected,
// user-driven errors (declined login). Everything else logs as before.
type quietRestyLogger struct {
	next resty.Logger
}

func (q quietRestyLogger) Errorf(format string, v ...interface{}) {
	if isDeclinedLoginMsg(format, v...) {
		return
	}
	q.next.Errorf(format, v...)
}

func (q quietRestyLogger) Warnf(format string, v ...interface{}) {
	if isDeclinedLoginMsg(format, v...) {
		return
	}
	q.next.Warnf(format, v...)
}

func (q quietRestyLogger) Debugf(format string, v ...interface{}) {
	q.next.Debugf(format, v...)
}

func isDeclinedLoginMsg(format string, v ...interface{}) bool {
	if !strings.Contains(format, "%v") { // formatted messages embed the error
		return false
	}
	msg := fmt.Sprintf(format, v...)
	return strings.Contains(msg, breverrors.DeclineToLoginMessage)
}

// stderrLogger mirrors resty's default logger (stderr, date+microseconds,
// "WARN RESTY"/"ERROR RESTY" prefixes) so quietRestyLogger has a real sink.
type stderrLogger struct {
	l *log.Logger
}

func newStderrLogger() stderrLogger {
	return stderrLogger{l: log.New(os.Stderr, "", log.Ldate|log.Lmicroseconds)}
}

func (s stderrLogger) Errorf(format string, v ...interface{}) {
	s.outputf("ERROR RESTY "+format, v...)
}

func (s stderrLogger) Warnf(format string, v ...interface{}) {
	s.outputf("WARN RESTY "+format, v...)
}

func (s stderrLogger) Debugf(format string, v ...interface{}) {
	s.outputf("DEBUG RESTY "+format, v...)
}

func (s stderrLogger) outputf(format string, v ...interface{}) {
	_ = s.l.Output(2, fmt.Sprintf(format, v...))
}

func NewAuthHTTPClient(auth Auth, brevAPIURL string, options ...Option) *AuthHTTPClient {
	opts := &Options{}
	for _, o := range options {
		o(opts)
	}
	restyClient := NewRestyClient(brevAPIURL)
	restyClient.Debug = opts.Debug
	// quietRestyLogger wraps a real stderr logger (matching resty's default
	// format) and swallows only declined-login retry chatter. Everything else
	// — genuine HTTP errors, debug output when Debug is on — still logs.
	restyClient.SetLogger(quietRestyLogger{next: newStderrLogger()})
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
