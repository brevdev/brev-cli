package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/google/uuid"
)

var _ OAuth = KasAuthenticator{}

const CredentialProviderKAS entity.CredentialProvider = "kas"

type KasAuthenticator struct {
	Email             string
	ShouldPromptEmail bool
	BaseURL           string
	PollTimeout       time.Duration
	Issuer            string
	RedirectURI       string
}

func (a KasAuthenticator) GetCredentialProvider() entity.CredentialProvider {
	return CredentialProviderKAS
}

func (a KasAuthenticator) IsTokenValid(token string) bool {
	return IssuerCheck(token, a.Issuer)
}

func NewKasAuthenticator(email, baseURL, issuer string, shouldPromptEmail bool, redirectURI string) KasAuthenticator {
	return KasAuthenticator{
		Email:             email,
		ShouldPromptEmail: shouldPromptEmail,
		Issuer:            issuer,
		BaseURL:           baseURL,
		PollTimeout:       5 * time.Minute,
		RedirectURI:       redirectURI,
	}
}

func (a KasAuthenticator) GetNewAuthTokensWithRefresh(refreshToken string) (*entity.AuthTokens, error) {
	splitRefreshToken := strings.Split(refreshToken, ":")
	if len(splitRefreshToken) != 2 {
		// Write the invalid refresh token to the specified file
		homeDir, err := os.UserHomeDir()
		if err == nil {
			filePath := homeDir + "/.brev/.invalid-refresh-token"
			_ = os.WriteFile(filePath, []byte(refreshToken), 0o600)
			fmt.Println("WARN: malformed refresh token, logging out")
		}
		return nil, nil
	}

	sessionKey, deviceID := splitRefreshToken[0], splitRefreshToken[1]
	token, err := a.retrieveIDToken(sessionKey, deviceID)
	if err != nil && strings.Contains(err.Error(), "UNAUTHORIZED") {
		return nil, nil
	} else if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &entity.AuthTokens{
		AccessToken:  token,
		RefreshToken: refreshToken,
	}, nil
}

type LoginCallResponse struct {
	UserCode        string `json:"user_code"`
	SessionKey      string `json:"session_key"`
	VerificationURL string `json:"verification_url"`
}

func (a KasAuthenticator) MakeLoginCall(id, email string) (LoginCallResponse, error) {
	url := fmt.Sprintf("%s/v2/device/login", a.BaseURL)
	payload := map[string]string{
		"email":       email,
		"deviceId":    id,
		"redirectUri": a.RedirectURI,
	}
	// Marshal the payload into JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return LoginCallResponse{}, breverrors.WrapAndTrace(err)
	}

	// Create a new POST request with JSON payload
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData)) //nolint:noctx // fine
	if err != nil {
		return LoginCallResponse{}, breverrors.WrapAndTrace(err)
	}

	req.Header.Set("Content-Type", "application/json")
	// Create an HTTP client and send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return LoginCallResponse{}, breverrors.WrapAndTrace(err)
	}
	defer resp.Body.Close() //nolint:errcheck // fine

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return LoginCallResponse{}, breverrors.WrapAndTrace(err)
	}

	if resp.StatusCode >= 400 {
		return LoginCallResponse{}, fmt.Errorf("error making login call, status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var response LoginCallResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return LoginCallResponse{}, breverrors.WrapAndTrace(err)
	}
	return response, nil
}

func (a KasAuthenticator) DoDeviceAuthFlow(userLoginFlow func(url string, code string)) (*LoginTokens, error) {
	id := uuid.New().String()

	email, err := a.maybePromptForEmail()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	loginResp, err := a.MakeLoginCall(id, email)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	userLoginFlow(loginResp.VerificationURL, loginResp.UserCode)

	return a.pollForAuthentication(loginResp.SessionKey)
}

func (a KasAuthenticator) maybePromptForEmail() (string, error) {
	var email string
	if a.Email != "" && a.ShouldPromptEmail { //nolint:gocritic // fine
		fmt.Printf("Logging in with email %s\nPress enter to continue or type a different email: ", a.Email)
		var response string
		_, err := fmt.Scanln(&response)
		if err != nil && err.Error() != "unexpected newline" {
			return "", breverrors.WrapAndTrace(err)
		}
		if response == "" {
			email = a.Email
		} else {
			email = response
		}
	} else if a.Email != "" {
		return a.Email, nil
	} else {
		t := terminal.New()
		fmt.Print(t.Green("Enter your email: "))
		_, err := fmt.Scanln(&email)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
	}
	return email, nil
}

// pollForAuthentication waits until the user completes login in the browser.
// While the session is pending, HEAD /v2/ping returns 401; once the user
// authenticates, it returns 200 and the session_key becomes the bearer token.
func (a KasAuthenticator) pollForAuthentication(sessionKey string) (*LoginTokens, error) {
	// Try to authenticate for up to PollTimeout
	timeout := time.After(a.PollTimeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-timeout:
			return nil, breverrors.WrapAndTrace(fmt.Errorf("timed out waiting for login"))
		case <-ticker.C:
			authenticated, err := a.isSessionAuthenticated(sessionKey)
			if err != nil {
				// Transient errors (network blips, 5xx) are retried until timeout.
				continue
			}
			if authenticated {
				return &LoginTokens{
					AuthTokens: entity.AuthTokens{
						AccessToken: sessionKey,
					},
					IDToken: sessionKey,
				}, nil
			}
		}
	}
}

// isSessionAuthenticated reports whether the session_key is now a valid bearer
// token by calling HEAD /v2/ping. 200 means authenticated; 401 means the user
// has not completed login yet.
func (a KasAuthenticator) isSessionAuthenticated(sessionKey string) (bool, error) {
	pingURL := fmt.Sprintf("%s/v2/ping", a.BaseURL)
	req, err := http.NewRequest(http.MethodHead, pingURL, nil) //nolint:noctx // fine
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	req.Header.Set("Authorization", "Bearer "+sessionKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	defer resp.Body.Close() //nolint:errcheck // fine

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusUnauthorized:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected ping status code: %d", resp.StatusCode)
	}
}

type RetrieveIDTokenResponse struct {
	IDToken       string `json:"token"`
	RequestStatus struct {
		StatusCode        string `json:"statusCode"`
		StatusDescription string `json:"statusDescription"`
		RequestID         string `json:"requestId"`
	} `json:"requestStatus"`
}

// retrieveIDToken retrieves the ID token from BASE_API_URL + "/token".
// If the sessionKey is expired (24 hours after sessionKeyCreatedAt), it prompts the user
// to re-login using the "sample-device-login-golang login" command.
func (a KasAuthenticator) retrieveIDToken(sessionKey, deviceID string) (string, error) {
	tokenURL := fmt.Sprintf("%s/token", a.BaseURL)
	client := &http.Client{}
	tokenReq, err := http.NewRequest("GET", tokenURL, nil) //nolint:noctx // fine
	if err != nil {
		return "", fmt.Errorf("error creating token request: %v", err)
	}

	tokenReq.Header.Set("Authorization", "Bearer "+sessionKey)
	tokenReq.Header.Set("x-device-id", deviceID)

	tokenResp, err := client.Do(tokenReq)
	if err != nil {
		return "", fmt.Errorf("error sending token request: %v", err)
	}
	defer tokenResp.Body.Close() //nolint:errcheck // fine

	tokenBody, err := io.ReadAll(tokenResp.Body)
	if err != nil {
		return "", fmt.Errorf("error reading token response: %v", err)
	}

	if tokenResp.StatusCode >= 400 {
		return "", fmt.Errorf("error retrieving token, status code: %d, body: %s", tokenResp.StatusCode, string(tokenBody))
	}

	var tokenResponse RetrieveIDTokenResponse
	if err := json.Unmarshal(tokenBody, &tokenResponse); err != nil {
		return "", fmt.Errorf("error parsing token JSON response: %v", err)
	}

	return tokenResponse.IDToken, nil
}
