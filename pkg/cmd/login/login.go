// Package get is for the get command
package login

import (
	"context"
	"fmt"
	"time"

	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/auth0/auth0-cli/internal/ansi"

	"github.com/spf13/cobra"
		"github.com/pkg/browser"
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
			cmdutil.CheckErr(opts.RunSSH(cmd, args))
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

func (o *SshOptions) login(cmd *cobra.Command, args []string) (tenant, error) {
	ctx := context.Background()
	authenticator := auth.Authenticator{
		Audience:           "foo",
		ClientID:           "Bar",
		DeviceCodeEndpoint: "bar",
		OauthTokenEndpoint: "fadsf",
	}
	state, err := authenticator.Start(ctx)
	if err != nil {
		return tenant{}, fmt.Errorf("Could not start the authentication process: %w.", err)
	}

	// todo color library
	// fmt.Printf("Your Device Confirmation code is: %s\n\n", ansi.Bold(state.UserCode))
	// cli.renderer.Infof("%s to open the browser to log in or %s to quit...", ansi.Green("Press Enter"), ansi.Red("^C"))
	// fmt.Scanln()

	err = browser.OpenURL(state.VerificationURI)

	if err != nil {
		cli.renderer.Warnf("Couldn't open the URL, please do it manually: %s.", state.VerificationURI)
	}

	var res auth.Result
	err = ansi.Spinner("Waiting for login to complete in browser", func() error {
		res, err = cli.authenticator.Wait(ctx, state)
		return err
	})

	if err != nil {
		return tenant{}, fmt.Errorf("login error: %w", err)
	}

	fmt.Print("\n")
	cli.renderer.Infof("Successfully logged in.")
	cli.renderer.Infof("Tenant: %s\n", res.Domain)

	// store the refresh token
	secretsStore := &auth.Keyring{}
	err = secretsStore.Set(auth.SecretsNamespace, res.Domain, res.RefreshToken)
	if err != nil {
		// log the error but move on
		cli.renderer.Warnf("Could not store the refresh token locally, please expect to login again once your access token expired. See https://github.com/auth0/auth0-cli/blob/main/KNOWN-ISSUES.md.")
	}

	t := tenant{
		Name:        res.Tenant,
		Domain:      res.Domain,
		AccessToken: res.AccessToken,
		ExpiresAt: time.Now().Add(
			time.Duration(res.ExpiresIn) * time.Second,
		),
		Scopes: auth.RequiredScopes(),
	}
	err = cli.addTenant(t)
	if err != nil {
		return tenant{}, fmt.Errorf("Could not add tenant to config: %w", err)
	}

	if err := checkInstallID(cli); err != nil {
		return tenant{}, fmt.Errorf("Could not update config: %w", err)
	}

	if cli.config.DefaultTenant != res.Domain {
		promptText := fmt.Sprintf("Your default tenant is %s. Do you want to change it to %s?", cli.config.DefaultTenant, res.Domain)
		if confirmed := prompt.Confirm(promptText); !confirmed {
			return tenant{}, nil
		}
		cli.config.DefaultTenant = res.Domain
		if err := cli.persistConfig(); err != nil {
			cli.renderer.Warnf("Could not set the default tenant, please try 'auth0 tenants use %s': %w", res.Domain, err)
		}
	}

	return t, nil
}
