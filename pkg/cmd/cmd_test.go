package cmd

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeJWT builds an unsigned JWT with the given claims (header.payload.signature).
func fakeJWT(t *testing.T, claims map[string]interface{}) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload, err := json.Marshal(claims)
	require.NoError(t, err)
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + "."
}

func newTestFileStore(t *testing.T) *store.FileStore {
	t.Helper()
	fs := afero.NewMemMapFs()
	err := fs.MkdirAll("/home/testuser/.brev", 0o755)
	require.NoError(t, err)
	return store.NewBasicStore().WithFileSystem(fs).WithUserHomeDirGetter(
		func() (string, error) { return "/home/testuser", nil },
	)
}

func TestEmailCachingAuthStore_SaveCachesEmail(t *testing.T) {
	fs := newTestFileStore(t)
	s := &emailCachingAuthStore{
		MemoryAuthStore: store.NewMemoryAuthStore(),
		fileStore:       fs,
	}

	token := fakeJWT(t, map[string]interface{}{"email": "user@example.com"})
	err := s.SaveAuthTokens(entity.AuthTokens{AccessToken: token})
	require.NoError(t, err)

	cached, err := fs.GetCachedEmail()
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", cached)
}

func TestEmailCachingAuthStore_NoEmailInToken(t *testing.T) {
	fs := newTestFileStore(t)
	s := &emailCachingAuthStore{
		MemoryAuthStore: store.NewMemoryAuthStore(),
		fileStore:       fs,
	}

	token := fakeJWT(t, map[string]interface{}{"sub": "12345"})
	err := s.SaveAuthTokens(entity.AuthTokens{AccessToken: token})
	require.NoError(t, err)

	cached, err := fs.GetCachedEmail()
	require.NoError(t, err)
	assert.Equal(t, "", cached)
}

func TestEmailCachingAuthStore_EmptyAccessToken(t *testing.T) {
	fs := newTestFileStore(t)
	s := &emailCachingAuthStore{
		MemoryAuthStore: store.NewMemoryAuthStore(),
		fileStore:       fs,
	}

	err := s.SaveAuthTokens(entity.AuthTokens{AccessToken: ""})
	require.Error(t, err)
}
