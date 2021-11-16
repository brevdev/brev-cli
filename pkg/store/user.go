package store

import "github.com/brevdev/brev-cli/pkg/brevapi"

func (s AuthHTTPStore) GetCurrentUser() (*brevapi.User, error) {
	return s.authHTTPClient.toDeprecateClient.GetMe() // todo check cache first
}
