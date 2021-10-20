// Package get is for the get command
package login

import (
	"context"
	"fmt"
	"time"

	"github.com/brevdev/brev-cli/pkg/auth"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

var sshLong = "dsfsdfds"

var sshExample = "dsfsdfsfd"

type SshOptions struct{}

var (
	BrevAuthDomain = "https://brev.dev/oauth/device/code"
	BrevClientID   = "foo"
)

func NewCmdLogin() *cobra.Command {
	opts := SshOptions{}

	cmd := &cobra.Command{
		Use:                   "login",
		DisableFlagsInUseLine: true,
		Short:                 "log into brev",
		Long:                  "log into brev",
		Example:               "brev login",
		Args:                  cobra.NoArgs,
		// ValidArgsFunction: util.ResourceNameCompletionFunc(f, "pod"),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.Complete(cmd, args))
			cmdutil.CheckErr(opts.Validate(cmd, args))
			cmdutil.CheckErr(opts.RunLogin(cmd, args))
		},
	}
	return cmd
}

func (o *SshOptions) Complete(cmd *cobra.Command, args []string) error {
	// return fmt.Errorf("not implemented")
	return nil
}

func (o *SshOptions) Validate(cmd *cobra.Command, args []string) error {
	// return fmt.Errorf("not implemented")
	return nil
}

type app struct {
	FirstRuns map[string]bool `json:"first_runs"`
}


// tenant is the cli's concept of an auth0 tenant. The fields are tailor fit
// specifically for interacting with the management API.
type tenant struct {
	Name         string         `json:"name"`
	Domain       string         `json:"domain"`
	AccessToken  string         `json:"access_token,omitempty"`
	Scopes       []string       `json:"scopes,omitempty"`
	ExpiresAt    time.Time      `json:"expires_at"`
	Apps         map[string]app `json:"apps,omitempty"`
	DefaultAppID string         `json:"default_app_id,omitempty"`

	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}


func (o *SshOptions) RunLogin(cmd *cobra.Command, args []string) (error) {
	ctx := context.Background()

	// TODO env vars
	authenticator := auth.Authenticator{
		Audience:           "https://brevdev.us.auth0.com/api/v2/",
		ClientID:           "JaqJRLEsdat5w7Tb0WqmTxzIeqwqepmk",
		DeviceCodeEndpoint: "https://brevdev.us.auth0.com/oauth/device/code",
		OauthTokenEndpoint: "https://brevdev.us.auth0.com/oauth/token",
	}
	state, err := authenticator.Start(ctx)
	if err != nil {
		return  fmt.Errorf("Could not start the authentication process: %w.", err)
	}

	// todo color library
	// fmt.Printf("Your Device Confirmation code is: %s\n\n", ansi.Bold(state.UserCode))
	// cli.renderer.Infof("%s to open the browser to log in or %s to quit...", ansi.Green("Press Enter"), ansi.Red("^C"))
	// fmt.Scanln()
	fmt.Println("Your Devicce Confirmation Code is %s", state.UserCode)

	err = browser.OpenURL(state.VerificationURI)

	if err != nil {
		fmt.Println("please open: %s", state.VerificationURI)
	}

	fmt.Println("waiting for auth to complete")
	var res auth.Result

	res, err = authenticator.Wait(ctx, state)

	if err != nil {
		return  fmt.Errorf("login error: %w", err)
	}

	fmt.Print("\n")
	fmt.Println("Successfully logged in.")
	fmt.Println("Tenant: %s\n", res.Domain)
	creds := &auth.Credentials{
		AccessToken: res.AccessToken,
		RefreshToken: res.RefreshToken,
		ExpiresIn: int(res.ExpiresIn),
	}
	// store the refresh token
	auth.WriteTokenToBrevConfigFile(creds)
	return nil
}
