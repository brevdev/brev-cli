package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/google/uuid"
)

var _ OAuth = KasAuthenticator{}

type KasAuthenticator struct {
	BaseURL              string
	IDTokenExpiryMinutes float64
	SessionIDExpiryHours float64
}

func (a KasAuthenticator) GetNewAuthTokensWithRefresh(refreshToken string) (*entity.AuthTokens, error) {
	return nil, nil
}

type LoginCallResponse struct {
	LoginUrl   string `json:"loginUrl"`
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

	var response LoginCallResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return LoginCallResponse{}, breverrors.WrapAndTrace(err)
	}
	return response, nil
}

func (a KasAuthenticator) DoDeviceAuthFlow(onStateRetrieved func(url string, code string)) (*LoginTokens, error) {
	id := uuid.New()
	email := "kpinkerton@nvidia.com" // TODO: ask user for email

	loginResp, err := a.MakeLoginCall(id.String(), email)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	onStateRetrieved(loginResp.LoginUrl, id.String())

	idToken, err := a.retrieveIDToken(loginResp.SessionKey, id.String())
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &LoginTokens{
		AuthTokens: entity.AuthTokens{
			AccessToken:  idToken,
			RefreshToken: fmt.Sprintf("%s:%s", loginResp.SessionKey, id.String()),
		},
		IDToken: idToken,
	}, nil
}

type RetrieveIDTokenResponse struct {
	IDToken string `json:"token"`
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

	var tokenResponse RetrieveIDTokenResponse
	if err := json.Unmarshal(tokenBody, &tokenResponse); err != nil {
		return "", fmt.Errorf("error parsing token JSON response: %v", err)
	}

	return tokenResponse.IDToken, nil
}

// getIDToken reads the sessionConfig file and retrieves the idToken.
// If the idToken is expired (15 minutes after creation), a new token is fetched and set in the sessionConfig file.
func (a KasAuthenticator) getIDToken() (string, error) {
	session, err := readSessionConfig()
	if err != nil {
		fmt.Println(err)
		return "", err
	}

	idToken, ok := session["idToken"]
	if !ok || idToken == "" {
		return "", fmt.Errorf("idToken not found in session data. Please ensure you are logged in")
	}

	idTokenCreatedAtStr, ok := session["idTokenCreatedAt"]
	if !ok || idTokenCreatedAtStr == "" {
		return "", fmt.Errorf("idTokenCreatedAt not found in session data")
	}

	idTokenCreatedAt, err := time.Parse(time.RFC3339, idTokenCreatedAtStr)
	if err != nil {
		return "", fmt.Errorf("error parsing idTokenCreatedAt timestamp: %v", err)
	}

	if time.Since(idTokenCreatedAt).Minutes() >= a.IDTokenExpiryMinutes {
		fmt.Println("ID token is expired. Creating a new ID token using the sessionKey.")

		newIDToken, newIDTokenCreatedAt, err := a.retrieveIDToken()
		if err != nil {
			fmt.Println("error retrieving new ID token", err)
			return "", fmt.Errorf("error retrieving new ID token: %v", err)
		}

		session["idToken"] = newIDToken
		session["idTokenCreatedAt"] = newIDTokenCreatedAt

		return newIDToken, updateSessionConfig(session)
	}

	return idToken, nil
}
