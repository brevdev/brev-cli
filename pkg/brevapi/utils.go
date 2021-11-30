package brevapi

import (
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/deprecatedauth"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

type DeprecatedClient struct {
	Key *deprecatedauth.OauthToken
}

func NewDeprecatedClient() (*DeprecatedClient, error) {
	// get token and use it to create a client
	token, err := deprecatedauth.GetToken()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	client := DeprecatedClient{
		Key: token,
	}
	return &client, nil
}

func buildBrevEndpoint(resource string) string {
	baseEndpoint := config.GlobalConfig.GetBrevAPIURl()
	return baseEndpoint + resource
}
