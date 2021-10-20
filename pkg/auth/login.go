package auth

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gopkg.in/square/go-jose.v2"
	"gopkg.in/square/go-jose.v2/jwt"

	"github.com/brevdev/brev-cli/pkg/brev_errors"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/requests"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

const (
	cotterEndpoint        = "https://js.cotter.app/app"
	cotterBackendEndpoint = "https://www.cotter.app/api/v0"
	cotterTokenEndpoint   = "https://www.cotter.app/api/v0/token"
	cotterJwksEndpoint    = "https://www.cotter.app/api/v0/token/jwks"
	localPort             = "8395"
	localEndpoint         = "http://localhost:" + localPort

	brevCredentialsFile      = "credentials.json"
	globalActiveProjectsFile = "active_projects.json"
)

type cotterTokenRequestPayload struct {
	CodeVerifier      string `json:"code_verifier"`
	AuthorizationCode string `json:"authorization_code"`
	ChallengeId       int    `json:"challenge_id"`
	RedirectURL       string `json:"redirect_url"`
}

type cotterTokenResponseBody struct {
	OauthToken CotterOauthToken `json:"oauth_token"`
}

type CotterOauthToken struct {
	AccessToken  string `json:"access_token"`
	AuthMethod   string `json:"auth_method"`
	ExpiresIn    int    `json:"expires_in"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
}

//go:embed success.html
var successHTML embed.FS

// Login performs a full round trip to Cotter:
//   1. Generate a code verifier
//   2. Open the Brev+Cotter auth URL in the default browser
//   3. Start a local web server awaiting a redirect to localhost
//   4. Capture the Cotter token upon redirect
//   5. Write the Cotter token to a file in the hidden brev directory
func login(t *terminal.Terminal) error {
	cotterCodeVerifier := generateCodeVerifier()

	cotterURL, err := buildCotterAuthURL(cotterCodeVerifier)
	if err != nil {
		t.Errprint(err, "Failed to construct auth URL")
		return err
	}

	// TODO: pretty print URL?
	t.Print(cotterURL)

	err = openInDefaultBrowser(cotterURL)
	if err != nil {
		t.Errprint(err, "Failed to open default browser")
		return err
	}

	token, err := captureCotterToken(cotterCodeVerifier)
	if err != nil {
		t.Errprint(err, "Failed to capture auth token")
		return err
	}

	err = writeTokenToBrevConfigFile(token)
	if err != nil {
		t.Errprint(err, "Failed to write auth token to file")
		return err
	}

	t.Vprint(
		t.Green("\nYou're authenticated!\n") +
			t.Yellow("\tbrev init") + t.Green(" to make a new project or\n") +
			t.Yellow("\tbrev clone") + t.Green(" to clone your existing project"))

	return nil
}

func LoginAndInitialize(t *terminal.Terminal) error {

	err := login(t)
	if err != nil {
		return err
	}

	err = initializeActiveProjectsFile(t)
	if err != nil {
		return err
	}

	return nil
}

func initializeActiveProjectsFile(t *terminal.Terminal) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	brevActiveProjectsFile := home + "/" + files.GetBrevDirectory() + "/" + globalActiveProjectsFile
	exists, err := files.Exists(brevActiveProjectsFile, false)
	if err != nil {
		return err
	}
	if !exists {
		err = files.OverwriteJSON(brevActiveProjectsFile, []string{})
		if err != nil {
			t.Errprint(err, "Failed to initialize active projects file. Just run this and try again: echo '[]' > ~/.brev/active_projects.json ")
			return err
		}
	}

	return nil
}

// GetToken reads the previously-persisted token from the filesystem but may issue a round
// trip request to Cotter if the token is determined to have expired:
//   1. Read the Cotter token from the hidden brev directory
//   2. Determine if the token is valid
//   3. If valid, return
//   4. If invalid, issue a refresh request to Cotter
//   5. Write the refreshed Cotter token to a file in the hidden brev directory
func GetToken() (*CotterOauthToken, error) {
	token, err := getTokenFromBrevConfigFile()
	if err != nil {
		return nil, err
	}

	return token, nil

	// if token.isValid() {
	// 	return token, nil
	// }

	// refreshedToken, err := refreshCotterToken(token)
	// if err != nil {
	// 	return nil, err
	// }

	// err = writeTokenToBrevConfigFile(refreshedToken)
	// if err != nil {
	// 	return nil, err
	// }

	// return refreshedToken, nil
}

func (t *CotterOauthToken) isValid() bool {
	var claims map[string]interface{}

	jwtToken, err := jwt.ParseSigned(t.AccessToken)
	if err != nil {
		return false
	}

	jwks, err := fetchCotterPublicKeySet()
	if err != nil {
		return false
	}

	err = jwtToken.Claims(jwks, &claims)
	if err != nil {
		return false
	}

	now := time.Now().Unix()
	exp := int64(claims["exp"].(float64))
	if now > exp {
		return false
	}

	return true
}

func fetchCotterPublicKeySet() (*jose.JSONWebKeySet, error) {
	request := requests.RESTRequest{
		Method:   "GET",
		Endpoint: cotterJwksEndpoint,
	}
	response, err := request.SubmitStrict()
	if err != nil {
		return nil, err
	}

	var cotterJWKS jose.JSONWebKeySet
	err = response.UnmarshalPayload(&cotterJWKS)
	if err != nil {
		return nil, err
	}

	return &cotterJWKS, nil
}

func buildCotterAuthURL(codeVerifier string) (string, error) {
	state := generateStateValue()
	codeChallenge := generateCodeChallenge(codeVerifier)

	request := &requests.RESTRequest{
		Method:   "GET",
		Endpoint: cotterEndpoint,
		QueryParams: []requests.QueryParam{
			{"api_key", getCotterAPIKey()},
			{"redirect_url", localEndpoint},
			{"state", state},
			{"code_challenge", codeChallenge},
			{"type", "EMAIL"},
			{"auth_method", "MAGIC_LINK"},
		},
	}

	httpRequest, err := request.BuildHTTPRequest()
	if err != nil {
		return "", err
	}

	return httpRequest.URL.String(), nil
}

func openInDefaultBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		return errors.New(fmt.Sprintf("Unsupported runtime: %s", runtime.GOOS))
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.Command(cmd, args...).Start()
}

func captureCotterToken(codeVerifier string) (*CotterOauthToken, error) {
	m := http.NewServeMux()
	s := http.Server{Addr: ":" + localPort, Handler: m}

	var token *CotterOauthToken
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		errorParm := q.Get("error")
		if errorParm != "" {
			fmt.Println(errorParm)
			// panic!
		}

		code := q.Get("code")
		challengeID := q.Get("challenge_id")

		cotterToken, err := requestCotterToken(code, challengeID, codeVerifier)
		if err != nil {
			// panic!
		}

		token = cotterToken

		content, _ := successHTML.ReadFile("success.html")
		w.Write(content)

		err = s.Shutdown(context.Background())
		if err != nil {
			// panic!
		}
	})

	// blocks
	s.ListenAndServe()
	return token, nil
}

func writeTokenToBrevConfigFile(token *CotterOauthToken) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	brevCredentialsFile := home + "/" + files.GetBrevDirectory() + "/" + brevCredentialsFile

	err = files.OverwriteJSON(brevCredentialsFile, token)
	if err != nil {
		return err
	}

	return nil
}

func getTokenFromBrevConfigFile() (*CotterOauthToken, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	brevCredentialsFile := home + "/" + files.GetBrevDirectory() + "/" + brevCredentialsFile
	exists, err := files.Exists(brevCredentialsFile, false)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, &brev_errors.CredentialsFileNotFound{}
	}

	var token CotterOauthToken
	err = files.ReadJSON(brevCredentialsFile, &token)
	if err != nil {
		return nil, err
	}

	return &token, nil
}

func requestCotterToken(code string, challengeID string, codeVerifier string) (*CotterOauthToken, error) {
	challengeIDInt, err := strconv.Atoi(challengeID)
	if err != nil {
		return nil, err
	}

	request := &requests.RESTRequest{
		Method:   "POST",
		Endpoint: cotterBackendEndpoint + "/verify/get_identity",
		Headers: []requests.Header{
			{"API_KEY_ID", getCotterAPIKey()},
			{"Content-Type", "application/json"},
		},
		QueryParams: []requests.QueryParam{
			{"oauth_token", "true"},
		},
		Payload: cotterTokenRequestPayload{
			CodeVerifier:      codeVerifier,
			AuthorizationCode: code,
			ChallengeId:       challengeIDInt,
			RedirectURL:       localEndpoint,
		},
	}
	response, err := request.SubmitStrict()
	if err != nil {
		return nil, err
	}

	var tokenResponse cotterTokenResponseBody
	err = response.UnmarshalPayload(&tokenResponse)
	if err != nil {
		return nil, err
	}

	return &tokenResponse.OauthToken, nil
}

func refreshCotterToken(oldToken *CotterOauthToken) (*CotterOauthToken, error) {
	request := &requests.RESTRequest{
		Method:   "POST",
		Endpoint: cotterTokenEndpoint + "/" + getCotterAPIKey(),
		Headers: []requests.Header{
			{"API_KEY_ID", getCotterAPIKey()},
			{"Content-Type", "application/json"},
		},
		Payload: map[string]string{
			"grant_type":    "refresh_token",
			"refresh_token": oldToken.RefreshToken,
		},
	}
	response, err := request.Submit()
	if err != nil {
		return nil, err
	}
	if response.StatusCode == http.StatusBadRequest {
		return nil, &brev_errors.CotterClientError{}
	}
	if response.StatusCode == http.StatusInternalServerError {
		return nil, &brev_errors.CotterServerError{}
	}

	var token CotterOauthToken
	err = response.UnmarshalPayload(&token)
	if err != nil {
		return nil, err
	}
	return &token, nil
}

func generateStateValue() string {
	return randomAlphabetical(10)
}

func generateCodeVerifier() string {
	codeVerifierBytes := randomBytes(32)
	codeVerifierRaw := base64.URLEncoding.EncodeToString(codeVerifierBytes)
	codeVerifier := strings.TrimRight(codeVerifierRaw, "=")

	return codeVerifier
}

func generateCodeChallenge(codeVerifier string) string {
	sha256Hasher := sha256.New()
	sha256Hasher.Write([]byte(codeVerifier))
	challengeBytes := sha256Hasher.Sum(nil)
	challengeRaw := base64.URLEncoding.EncodeToString(challengeBytes)
	challenge := strings.TrimRight(challengeRaw, "=")

	return challenge
}

func getCotterAPIKey() string {
	return config.GetCotterAPIKey()
}
