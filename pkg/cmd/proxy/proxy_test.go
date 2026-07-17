package proxy

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type privateKeyProxyStore struct {
	privateKey        string
	writtenPrivateKey string
}

func (s *privateKeyProxyStore) GetAuthTokens() (*entity.AuthTokens, error) {
	return nil, nil
}

func (s *privateKeyProxyStore) GetWorkspace(string) (*entity.Workspace, error) {
	return nil, nil
}

func (s *privateKeyProxyStore) GetCurrentUserSSHPrivateKey() (string, error) {
	return s.privateKey, nil
}

func (s *privateKeyProxyStore) WritePrivateKey(privateKey string) error {
	s.writtenPrivateKey = privateKey
	return nil
}

func TestVersionParsing(t *testing.T) {
	_, err := version.NewVersion("abadfjladsf")
	assert.NotNil(t, err)
}

func TestWriteUserPrivateKeyWritesCurrentUsersPrivateKey(t *testing.T) {
	store := &privateKeyProxyStore{privateKey: "private"}

	err := WriteUserPrivateKey(store)

	require.NoError(t, err)
	require.Equal(t, "private", store.writtenPrivateKey)
}
