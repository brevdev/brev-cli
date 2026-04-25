// Package status implements the `brev status` command, which reports the
// caller's login state, current organization, and credential metadata.
package status

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cobra"
)

const (
	statusLong    = "Show your Brev login status, current organization, and credential metadata."
	statusExample = "  brev status"

	// auth0Issuer mirrors the hard-coded issuer used in auth.StandardLogin.
	auth0Issuer = "https://brevdev.us.auth0.com/"
)

type StatusStore interface {
	GetAuthTokens() (*entity.AuthTokens, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetCurrentWorkspaceID() (string, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
}

func NewCmdStatus(t *terminal.Terminal, statusStore StatusStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "status",
		DisableFlagsInUseLine: true,
		Short:                 "Show login status, organization, and credential info",
		Long:                  statusLong,
		Example:               statusExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			runShowStatus(t, statusStore)
			return nil
		},
	}
	return cmd
}

func runShowStatus(t *terminal.Terminal, statusStore StatusStore) {
	terminal.DisplayBrevLogo(t)
	t.Vprintf("\n")

	tokens, loggedIn := showAuthStatus(t, statusStore)
	if loggedIn {
		showUserAndOrg(t, statusStore, tokens)
	}
	showWorkspaceStatus(t, statusStore)
}

// showAuthStatus prints the login section. Returns the loaded tokens (may be nil)
// and whether the user is logged in.
func showAuthStatus(t *terminal.Terminal, statusStore StatusStore) (*entity.AuthTokens, bool) {
	tokens, err := statusStore.GetAuthTokens()
	if err != nil {
		var notFound *breverrors.CredentialsFileNotFound
		if errors.As(err, &notFound) {
			printLoggedOut(t)
			return nil, false
		}
		t.Vprintf("Status: %s (%s)\n", t.Red("unknown"), err.Error())
		return nil, false
	}
	if tokens == nil || (tokens.AccessToken == "" && tokens.RefreshToken == "") {
		printLoggedOut(t)
		return nil, false
	}

	t.Vprintf("Status:      %s\n", t.Green("Logged in"))

	provider, providerDetail := describeCredential(tokens)
	if providerDetail != "" {
		t.Vprintf("Provider:    %s (%s)\n", t.Yellow(provider), providerDetail)
	} else {
		t.Vprintf("Provider:    %s\n", t.Yellow(provider))
	}

	issuedAt, expiresAt, gotClaims := tokenIssuedAndExpiry(tokens.AccessToken)
	if gotClaims {
		if !issuedAt.IsZero() {
			t.Vprintf("Issued at:   %s\n", t.Yellow(issuedAt.Local().Format(time.RFC3339)))
		}
		if !expiresAt.IsZero() {
			remaining := time.Until(expiresAt)
			if remaining > 0 {
				t.Vprintf("Expires at:  %s (%s remaining)\n",
					t.Yellow(expiresAt.Local().Format(time.RFC3339)),
					t.Yellow(formatDuration(remaining)))
			} else {
				t.Vprintf("Expires at:  %s (%s; refresh token will be used on next call)\n",
					t.Yellow(expiresAt.Local().Format(time.RFC3339)),
					t.Red("expired"))
			}
		}
	} else if tokens.AccessToken != "" {
		t.Vprintf("Token:       %s\n", t.Yellow("opaque (no expiration metadata)"))
	}

	if tokens.RefreshToken != "" && tokens.RefreshToken != "auto-login" {
		t.Vprintf("Refresh:     %s\n", t.Yellow("present"))
	} else {
		t.Vprintf("Refresh:     %s\n", t.Yellow("absent"))
	}

	return tokens, true
}

func showUserAndOrg(t *terminal.Terminal, statusStore StatusStore, tokens *entity.AuthTokens) {
	tokenEmail := ""
	if tokens != nil {
		tokenEmail = auth.GetEmailFromToken(tokens.AccessToken)
	}

	user, userErr := statusStore.GetCurrentUser()
	if userErr == nil && user != nil {
		t.Vprintf("\nUser:        %s\n", t.Yellow(coalesce(user.Name, user.Username, user.Email)))
		t.Vprintf("\tID:        %s\n", t.Yellow(user.ID))
		if user.Email != "" {
			t.Vprintf("\tEmail:     %s\n", t.Yellow(user.Email))
		}
		if user.Username != "" && user.Username != user.Name {
			t.Vprintf("\tUsername:  %s\n", t.Yellow(user.Username))
		}
	} else {
		if tokenEmail != "" {
			t.Vprintf("\nUser:        %s\n", t.Yellow(tokenEmail))
		}
		if userErr != nil {
			t.Vprintf("\t(remote user lookup failed: %s)\n", t.Red(rootErrorMessage(userErr)))
		}
	}

	org, orgErr := statusStore.GetActiveOrganizationOrDefault()
	if orgErr == nil && org != nil {
		t.Vprintf("\nOrg:         %s\n", t.Yellow(org.Name))
		t.Vprintf("\tID:        %s\n", t.Yellow(org.ID))
	} else if orgErr != nil {
		t.Vprintf("\nOrg:         %s\n", t.Red("unknown"))
		t.Vprintf("\t(remote org lookup failed: %s)\n", t.Red(rootErrorMessage(orgErr)))
	} else {
		t.Vprintf("\nOrg:         %s\n", t.Yellow("none set"))
	}
}

func showWorkspaceStatus(t *terminal.Terminal, statusStore StatusStore) {
	wsID, err := statusStore.GetCurrentWorkspaceID()
	if err != nil || wsID == "" {
		return
	}
	ws, err := statusStore.GetWorkspace(wsID)
	if err != nil {
		t.Vprintf("\nInstance lookup failed: %s\n", t.Red(rootErrorMessage(err)))
		return
	}
	t.Vprintf("\nInstance:    %s\n", t.Yellow(ws.Name))
	t.Vprintf("\tID:        %s\n", t.Yellow(ws.ID))
	t.Vprintf("\tMachine:   %s\n", t.Yellow(util.GetInstanceString(*ws)))
}

// rootErrorMessage returns a single-line message from the deepest wrapped
// error, stripping the file/line trace that breverrors.WrapAndTrace prepends.
func rootErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	root := breverrors.Root(err)
	if root == nil {
		return strings.TrimSpace(err.Error())
	}
	return strings.TrimSpace(root.Error())
}

func printLoggedOut(t *terminal.Terminal) {
	t.Vprintf("Status:      %s\n", t.Red("Logged out"))
	t.Vprintf("Run %s to log in.\n", t.Yellow("brev login"))
}

// describeCredential infers the credential provider from the stored tokens.
// Returns (provider, optional detail string).
func describeCredential(tokens *entity.AuthTokens) (string, string) {
	if tokens.AccessToken == "" {
		return "unknown", ""
	}
	// FileStore.GetAuthTokens returns the Kubernetes service-account token
	// with an empty refresh token when running inside a Brev workspace pod.
	if tokens.RefreshToken == "" {
		return "service account", "Kubernetes pod token"
	}
	if tokens.RefreshToken == "auto-login" {
		return "manual access token", "set via `brev login --token`"
	}
	if tokens.AccessToken == "auto-login" {
		return "manual refresh token", "set via `brev login --token`"
	}
	if auth.IssuerCheck(tokens.AccessToken, auth0Issuer) {
		return "auth0", auth0Issuer
	}
	if kasIssuer := config.GlobalConfig.GetBrevAuthIssuerURL(); kasIssuer != "" && auth.IssuerCheck(tokens.AccessToken, kasIssuer) {
		return "kas (NVIDIA NGC)", kasIssuer
	}
	if iss := tokenIssuer(tokens.AccessToken); iss != "" {
		return "unknown", iss
	}
	return "unknown", ""
}

func tokenIssuer(token string) string {
	parser := jwt.Parser{}
	claims := jwt.MapClaims{}
	_, _, err := parser.ParseUnverified(token, &claims)
	if err != nil {
		return ""
	}
	iss, _ := claims["iss"].(string)
	return iss
}

// tokenIssuedAndExpiry parses a JWT and returns iat and exp times. ok=false if
// the token isn't a JWT or has neither claim.
func tokenIssuedAndExpiry(token string) (issuedAt, expiresAt time.Time, ok bool) {
	parser := jwt.Parser{}
	claims := jwt.MapClaims{}
	_, _, err := parser.ParseUnverified(token, &claims)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	if exp, e := claims.GetExpirationTime(); e == nil && exp != nil {
		expiresAt = exp.Time
	}
	if iat, e := claims.GetIssuedAt(); e == nil && iat != nil {
		issuedAt = iat.Time
	}
	return issuedAt, expiresAt, !expiresAt.IsZero() || !issuedAt.IsZero()
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		return fmt.Sprintf("%dd %dh", days, hours)
	}
}

func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
