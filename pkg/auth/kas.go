package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/google/uuid"
)

var _ OAuth = KasAuthenticator{}

type KasAuthenticator struct {
	Email       string
	BaseURL     string
	PollTimeout time.Duration
}

func NewKasAuthenticator(email, baseURL string) KasAuthenticator {
	return KasAuthenticator{
		Email:       email,
		BaseURL:     baseURL,
		PollTimeout: 5 * time.Minute,
	}
}

func (a KasAuthenticator) GetNewAuthTokensWithRefresh(refreshToken string) (*entity.AuthTokens, error) {
	splitRefreshToken := strings.Split(refreshToken, ":")
	if len(splitRefreshToken) != 2 {
		return nil, fmt.Errorf("invalid refresh token")
	}
	sessionKey, deviceID := splitRefreshToken[0], splitRefreshToken[1]
	token, err := a.retrieveIDToken(sessionKey, deviceID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &entity.AuthTokens{
		AccessToken:        token,
		RefreshToken:       refreshToken,
		CredentialProvider: entity.CredentialProviderKAS,
	}, nil
}

type LoginCallResponse struct {
	LoginURL   string `json:"loginUrl"`
	SessionKey string `json:"sessionKey"`
}

func (a KasAuthenticator) MakeLoginCall(id, email string) (LoginCallResponse, error) {
	url := fmt.Sprintf("%s/device/login", a.BaseURL)
	payload := map[string]string{
		"email":    email,
		"deviceId": id,
	}
	// Marshal the payload into JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return LoginCallResponse{}, breverrors.WrapAndTrace(err)
	}

	// Create a new POST request with JSON payload
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
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
	defer resp.Body.Close()

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
	email := a.Email

	if a.Email == "" {
		fmt.Print("Enter your email: ")
		_, err := fmt.Scanln(&email)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
	}

	loginResp, err := a.MakeLoginCall(id, email)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	userLoginFlow(loginResp.LoginURL, "")

	return a.pollForTokens(loginResp.SessionKey, id)
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
				fmt.Println(idToken)
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
	tokenReq, err := http.NewRequest("GET", tokenURL, nil)
	if err != nil {
		return "", fmt.Errorf("error creating token request: %v", err)
	}

	tokenReq.Header.Set("Authorization", "Bearer "+sessionKey)
	tokenReq.Header.Set("x-device-id", deviceID)

	tokenResp, err := client.Do(tokenReq)
	if err != nil {
		return "", fmt.Errorf("error sending token request: %v", err)
	}
	defer tokenResp.Body.Close()

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
