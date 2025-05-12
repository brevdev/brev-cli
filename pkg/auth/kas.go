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
	LoginURL   string `json:"loginUrl"`
	SessionKey string `json:"sessionKey"`
}

func (a KasAuthenticator) MakeLoginCall(id, email string) (LoginCallResponse, error) {
	url := fmt.Sprintf("%s/device/login", a.BaseURL)
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

	userLoginFlow(loginResp.LoginURL, "")

	return a.pollForTokens(loginResp.SessionKey, id)
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

func (a KasAuthenticator) pollForTokens(sessionKey, id string) (*LoginTokens, error) {
	// Try to retrieve tokens for up to 5 minutes
	timeout := time.After(a.PollTimeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-timeout:
			return nil, breverrors.WrapAndTrace(fmt.Errorf("timed out waiting for login"))
		case <-ticker.C:
			idToken, err := a.retrieveIDToken(sessionKey, id)
			if err == nil {
				return &LoginTokens{
					AuthTokens: entity.AuthTokens{
						AccessToken:  idToken,
						RefreshToken: fmt.Sprintf("%s:%s", sessionKey, id),
					},
					IDToken: idToken,
				}, nil
			}
			// Continue polling on error
		}
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
