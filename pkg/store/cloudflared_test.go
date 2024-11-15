package store

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/fatih/color"
	"github.com/stretchr/testify/assert"
)

func makeCloudflare() Cloudflared {
	conf := config.NewConstants()
	fs := files.AppFs
	authenticator := auth.Auth0Authenticator{
		Audience:           "https://brevdev.us.auth0.com/api/v2/",
		ClientID:           "JaqJRLEsdat5w7Tb0WqmTxzIeqwqepmk",
		DeviceCodeEndpoint: "https://brevdev.us.auth0.com/oauth/device/code",
		OauthTokenEndpoint: "https://brevdev.us.auth0.com/oauth/token",
	}
	// super annoying. this is needed to make the import stay
	_ = color.New(color.FgYellow, color.Bold).SprintFunc()

	fsStore := NewBasicStore().
		WithFileSystem(fs)
	loginAuth := auth.NewLoginAuth(fsStore, authenticator)

	loginCmdStore := fsStore.WithNoAuthHTTPClient(
		NewNoAuthHTTPClient(conf.GetBrevAPIURl()),
	).
		WithAuth(loginAuth, WithDebug(conf.GetDebugHTTP()))
	return Cloudflared{
		store: loginCmdStore,
	}
}

func TestTask_DownloadCloudflaredBinary(t *testing.T) {
	client := makeCloudflare()

	err := client.DownloadCloudflaredBinaryIfItDNE()
	assert.NoError(t, err)
}
