package store

import (
	"path/filepath"
	"testing"

	authpkg "github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testAPIKey = authpkg.BrevAPIKeyPrefix + "test-key"

func newAuthTokenTestStore(t *testing.T) (*FileStore, afero.Fs, string) {
	t.Helper()
	fs := afero.NewOsFs()
	home := t.TempDir()
	s := NewBasicStore().WithFileSystem(fs).WithUserHomeDirGetter(
		func() (string, error) { return home, nil },
	)
	return s, fs, home
}

func TestFileStore_GetAuthTokens_LegacyCredentialsDefaultToJWT(t *testing.T) {
	s, fs, home := newAuthTokenTestStore(t)
	credentialsPath := filepath.Join(home, ".brev", "credentials.json")
	require.NoError(t, fs.MkdirAll(filepath.Dir(credentialsPath), 0o755))
	require.NoError(t, afero.WriteFile(fs, credentialsPath, []byte(`{
 "access_token": "jwt-token",
 "refresh_token": "refresh-token"
}`), 0o600))

	got, err := s.GetAuthTokens()

	require.NoError(t, err)
	assert.Equal(t, "jwt-token", got.AccessToken)
	assert.Equal(t, "refresh-token", got.RefreshToken)
}

func TestFileStore_SaveAuthTokens_WritesAPIKey(t *testing.T) {
	s, _, _ := newAuthTokenTestStore(t)

	err := s.SaveAuthTokens(entity.AuthTokens{
		AccessToken: "jwt-token",
		APIKey:      testAPIKey,
		APIKeyOrgID: "org-test",
	})
	require.NoError(t, err)

	got, err := s.GetAuthTokens()
	require.NoError(t, err)
	assert.Equal(t, "jwt-token", got.AccessToken)
	assert.Equal(t, testAPIKey, got.APIKey)
	assert.Equal(t, "org-test", got.APIKeyOrgID)
}
