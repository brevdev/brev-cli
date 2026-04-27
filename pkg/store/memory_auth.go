package store

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

// MemoryAuthStore implements auth.AuthStore with in-memory token storage.
// Tokens live only for the lifetime of the process — nothing is written to disk.
type MemoryAuthStore struct {
	tokens *entity.AuthTokens
}

func NewMemoryAuthStore() *MemoryAuthStore {
	return &MemoryAuthStore{}
}

func (m *MemoryAuthStore) SaveAuthTokens(tokens entity.AuthTokens) error {
	if tokens.AccessToken == "" && tokens.APIKey == "" {
		return fmt.Errorf("access token and api key are empty")
	}
	m.tokens = &tokens
	return nil
}

func (m *MemoryAuthStore) GetAuthTokens() (*entity.AuthTokens, error) {
	if m.tokens == nil {
		return nil, &breverrors.CredentialsFileNotFound{}
	}
	return m.tokens, nil
}

func (m *MemoryAuthStore) DeleteAuthTokens() error {
	m.tokens = nil
	return nil
}

// Seed pre-fills the in-memory store, allowing the auth layer to skip login.
// Intended for future use where the user can supply a token
func (m *MemoryAuthStore) Seed(tokens entity.AuthTokens) {
	m.tokens = &tokens
}
