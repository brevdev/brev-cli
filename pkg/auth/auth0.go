package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

const (
	audiencePath           = "/api/v2/"
	waitThresholdInSeconds = 1
	// namespace used to set/get values from the keychain.
	SecNamespace = "auth0-cli"
)

var requiredScopes = []string{
	"openid",
	"profile",
	"email",
	"offline_access", // <-- to get a refresh token.
	"create:clients", "delete:clients", "read:clients", "update:clients",
	"create:resource_servers", "delete:resource_servers", "read:resource_servers", "update:resource_servers",
	"create:roles", "delete:roles", "read:roles", "update:roles",
	"create:rules", "delete:rules", "read:rules", "update:rules",
	"create:users", "delete:users", "read:users", "update:users",
	"read:branding", "update:branding",
	"read:email_templates", "update:email_templates",
	"read:connections", "update:connections",
	"read:client_keys", "read:logs", "read:tenant_settings",
	"read:custom_domains", "create:custom_domains", "update:custom_domains", "delete:custom_domains",
	"read:anomaly_blocks", "delete:anomaly_blocks",
	"create:log_streams", "delete:log_streams", "read:log_streams", "update:log_streams",
	"create:actions", "delete:actions", "read:actions", "update:actions",
	"create:organizations", "delete:organizations", "read:organizations", "update:organizations",
}

type Authenticator struct {
	Audience           string
	ClientID           string
	DeviceCodeEndpoint string
	OauthTokenEndpoint string
}

var _ OAuth = Authenticator{}

type Result struct {
	Tenant       string
	Domain       string
	RefreshToken string
	AccessToken  string
	IDToken      string
	ExpiresIn    int64
}

type State struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri_complete"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// RequiredScopes returns the scopes used for login.

// RequiredScopesMin returns minimum scopes used for login in integration tests.

func (s *State) IntervalDuration() time.Duration {
	return time.Duration(s.Interval+waitThresholdInSeconds) * time.Second
}

func (a Authenticator) DoDeviceAuthFlow(onStateRetrieved func(url string, code string)) (*LoginTokens, error) {
	ctx := context.Background()

	state, err := a.Start(ctx)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	onStateRetrieved(state.VerificationURI, state.UserCode)

	res, err := a.Wait(ctx, state)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return &LoginTokens{
		AuthTokens: entity.AuthTokens{
			AccessToken:  res.AccessToken,
			RefreshToken: res.RefreshToken,
		},
		IDToken: res.IDToken,
	}, nil
}

// Start kicks-off the device authentication flow
// by requesting a device code from Auth0,
// The returned state contains the URI for the next step of the flow.
func (a *Authenticator) Start(ctx context.Context) (State, error) {
	s, err := a.getDeviceCode(ctx)
	if err != nil {
		return State{}, breverrors.WrapAndTrace(breverrors.Errorf("cannot get device code %w", err))
	}
	return s, nil
}

// postFormWithContext wraps the process of creating a POST request with form data and a context
func postFormWithContext(ctx context.Context, url string, data url.Values) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return resp, nil
}

// Wait waits until the user is logged in on the browser.
func (a *Authenticator) Wait(ctx context.Context, state State) (Result, error) {
	t := time.NewTicker(state.IntervalDuration())
	for {
		select {
		case <-ctx.Done():
			return Result{}, breverrors.WrapAndTrace(ctx.Err())
		case <-t.C:
			data := url.Values{
				"client_id":   {a.ClientID},
				"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
				"device_code": {state.DeviceCode},
			}
			r, err := postFormWithContext(ctx, a.OauthTokenEndpoint, data)
			if err != nil {
				return Result{}, breverrors.WrapAndTrace(breverrors.Errorf("%w %w", err, breverrors.NetworkErrorMessage))
			}
			err = ErrorIfBadHTTP(r, http.StatusForbidden)
			if err != nil {
				return Result{}, breverrors.WrapAndTrace(err)
			}

			var res struct {
				AccessToken      string  `json:"access_token"`
				IDToken          string  `json:"id_token"`
				RefreshToken     string  `json:"refresh_token"`
				Scope            string  `json:"scope"`
				ExpiresIn        int64   `json:"expires_in"`
				TokenType        string  `json:"token_type"`
				Error            *string `json:"error,omitempty"`
				ErrorDescription string  `json:"error_description,omitempty"`
			}

			err = json.NewDecoder(r.Body).Decode(&res)
			if err != nil {
				return Result{}, breverrors.Wrap(err, "cannot decode response")
			}

			if res.Error != nil {
				if *res.Error == "authorization_pending" {
					continue
				}
				return Result{}, breverrors.WrapAndTrace(errors.New(res.ErrorDescription))
			}

			ten, domain, err := parseTenant(res.AccessToken)
			if err != nil {
				return Result{}, breverrors.Wrap(err, "cannot parse tenant from the given access token")
			}

			if err = r.Body.Close(); err != nil {
				return Result{}, breverrors.WrapAndTrace(err)
			}
			return Result{
				RefreshToken: res.RefreshToken,
				AccessToken:  res.AccessToken,
				ExpiresIn:    res.ExpiresIn,
				Tenant:       ten,
				Domain:       domain,
				IDToken:      res.IDToken,
			}, nil
		}
	}
}

func (a *Authenticator) getDeviceCode(ctx context.Context) (State, error) {
	data := url.Values{
		"client_id": {a.ClientID},
		"scope":     {strings.Join(requiredScopes, " ")},
		"audience":  {a.Audience},
	}
	r, err := postFormWithContext(ctx, a.DeviceCodeEndpoint, data)
	if err != nil {
		return State{}, breverrors.Wrap(err, breverrors.NetworkErrorMessage)
	}
	err = ErrorIfBadHTTP(r)
	if err != nil {
		return State{}, breverrors.WrapAndTrace(err)
	}

	var res State
	err = json.NewDecoder(r.Body).Decode(&res)
	if err != nil {
		return State{}, breverrors.Wrap(err, "cannot decode response")
	}
	// TODO 9 if status code > 399 handle errors
	// {"error":"unauthorized_client","error_description":"Grant type 'urn:ietf:params:oauth:grant-type:device_code' not allowed for the client.","error_uri":"https://auth0.com/docs/clients/client-grant-types"}

	if err = r.Body.Close(); err != nil {
		return State{}, breverrors.WrapAndTrace(err)
	}
	return res, nil
}

func parseTenant(accessToken string) (tenant, domain string, err error) {
	parts := strings.Split(accessToken, ".")
	v, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", breverrors.WrapAndTrace(err)
	}
	var payload struct {
		AUDs []string `json:"aud"`
	}
	if err := json.Unmarshal(v, &payload); err != nil {
		return "", "", breverrors.WrapAndTrace(err)
	}

	for _, aud := range payload.AUDs {
		u, err := url.Parse(aud)
		if err != nil {
			return "", "", breverrors.WrapAndTrace(err)
		}
		if u.Path == audiencePath {
			parts := strings.Split(u.Host, ".")
			return parts[0], u.Host, nil
		}
	}
	return "", "", breverrors.WrapAndTrace(fmt.Errorf("audience not found for %s", audiencePath))
}

type AuthError struct {
	// https://auth0.com/docs/api/authentication#standard-error-responses
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// https://auth0.com/docs/get-started/authentication-and-authorization-flow/call-your-api-using-the-authorization-code-flow#example-post-to-token-url
func (a Authenticator) GetNewAuthTokensWithRefresh(refreshToken string) (*entity.AuthTokens, error) {
	payload := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {a.ClientID},
		"refresh_token": {refreshToken},
	}

	r, err := postFormWithContext(context.TODO(), a.OauthTokenEndpoint, payload)
	if err != nil {
		return nil, breverrors.Wrap(err, breverrors.NetworkErrorMessage)
	}
	err = ErrorIfBadHTTP(r, http.StatusForbidden)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if r.StatusCode == http.StatusForbidden {
		var authError AuthError
		err = json.NewDecoder(r.Body).Decode(&authError)
		if err != nil {
			// more readable error
			err = ErrorIfBadHTTP(r)
			if err != nil {
				return nil, breverrors.WrapAndTrace(err)
			}
		}
		if strings.Contains(authError.ErrorDescription, "invalid refresh token") {
			fmt.Printf("warning: invalid refresh token | %s\n", authError.ErrorDescription)
			return nil, nil
		}

		err = ErrorIfBadHTTP(r)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
	}

	var authTokens entity.AuthTokens

	err = json.NewDecoder(r.Body).Decode(&authTokens)
	if err != nil {
		return nil, breverrors.Wrap(err, "cannot decode response")
	}

	_, _, err = parseTenant(authTokens.AccessToken)
	if err != nil {
		return nil, breverrors.Wrap(err, "cannot parse tenant from the given access token")
	}

	if err = r.Body.Close(); err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &authTokens, nil
}

func ErrorIfBadHTTP(r *http.Response, exceptStatus ...int) error {
	shouldExcept := false
	for _, s := range exceptStatus {
		if r.StatusCode == s {
			shouldExcept = true
			break
		}
	}

	if IsError(r.StatusCode) && !shouldExcept {
		return NewHTTPResponseError(r)
	}
	return nil
}

// IsError method returns true if HTTP status `code >= 400` otherwise false.
func IsError(statusCode int) bool {
	return statusCode > 399
}

type StdHTTPResponseError struct {
	response *http.Response
}

func NewHTTPResponseError(response *http.Response) *StdHTTPResponseError {
	return &StdHTTPResponseError{
		response: response,
	}
}

func (e StdHTTPResponseError) Error() string {
	return fmt.Sprintf("%s %s", e.response.Request.URL, e.response.Status)
}
