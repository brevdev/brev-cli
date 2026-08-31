package cmd

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/brevdev/brev-cli/pkg/auth"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
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
	home := t.TempDir() // hoist: TempDir inside the getter closure misbehaves
	fs := afero.NewOsFs()
	return store.NewBasicStore().WithFileSystem(fs).WithUserHomeDirGetter(
		func() (string, error) { return home, nil },
	)
}

func newEmailCachingAuthStore(fs *store.FileStore) *emailCachingAuthStore {
	return &emailCachingAuthStore{
		MemoryAuthStore: store.NewMemoryAuthStore(),
		fileStore:       fs,
	}
}

func TestAccessCommandsExcludesHiddenCommands(t *testing.T) {
	root := &cobra.Command{Use: "brev"}
	visible := &cobra.Command{
		Use:         "visible",
		Annotations: map[string]string{"access": ""},
		Run:         func(_ *cobra.Command, _ []string) {},
	}
	hidden := &cobra.Command{
		Use:         "hidden",
		Annotations: map[string]string{"access": ""},
		Hidden:      true,
		Run:         func(_ *cobra.Command, _ []string) {},
	}
	root.AddCommand(visible, hidden)

	commands := accessCommands(root)
	require.Len(t, commands, 1)
	assert.Same(t, visible, commands[0])
}

func TestEmailCachingAuthStore_SaveCachesEmail(t *testing.T) {
	fs := newTestFileStore(t)
	s := newEmailCachingAuthStore(fs)

	token := fakeJWT(t, map[string]interface{}{"email": "user@example.com"})
	err := s.SaveAuthTokens(entity.AuthTokens{AccessToken: token})
	require.NoError(t, err)

	cached, err := fs.GetCachedEmail()
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", cached)
}

func TestEmailCachingAuthStore_NoEmailInToken(t *testing.T) {
	fs := newTestFileStore(t)
	s := newEmailCachingAuthStore(fs)

	token := fakeJWT(t, map[string]interface{}{"sub": "12345"})
	err := s.SaveAuthTokens(entity.AuthTokens{AccessToken: token})
	require.NoError(t, err)

	cached, err := fs.GetCachedEmail()
	require.NoError(t, err)
	assert.Equal(t, "", cached)
}

func TestEmailCachingAuthStore_EmptyAccessToken(t *testing.T) {
	fs := newTestFileStore(t)
	s := newEmailCachingAuthStore(fs)

	err := s.SaveAuthTokens(entity.AuthTokens{AccessToken: ""})
	require.Error(t, err)
}

func TestExternalNodeAuth_UsesEnvAPIKey(t *testing.T) {
	fs := newTestFileStore(t)
	t.Setenv(auth.APIKeyEnvVar, auth.BrevAPIKeyPrefix+"env-key")
	nodeAuth := externalNodeAuth{
		memLoginAuth: auth.NewLoginAuth(newEmailCachingAuthStore(fs), mockNodeAuthOAuth{}),
	}

	token, err := nodeAuth.GetAccessToken()
	require.NoError(t, err)
	assert.Equal(t, auth.BrevAPIKeyPrefix+"env-key", token)
}

func TestExternalNodeAuth_FallsBackToEphemeralLogin(t *testing.T) {
	fs := newTestFileStore(t)
	t.Setenv(auth.APIKeyEnvVar, "")
	nodeAuth := externalNodeAuth{
		memLoginAuth: auth.NewLoginAuth(newEmailCachingAuthStore(fs), mockNodeAuthOAuth{
			loginTokens: &auth.LoginTokens{AuthTokens: entity.AuthTokens{
				AccessToken: fakeJWT(t, map[string]interface{}{"email": "user@example.com"}),
			}},
		}),
	}
	nodeAuth.memLoginAuth.WithShouldLogin(func() (bool, error) { return true, nil })

	token, err := nodeAuth.GetAccessToken()
	require.NoError(t, err)
	assert.Equal(t, fakeJWT(t, map[string]interface{}{"email": "user@example.com"}), token)

	// The ephemeral login tokens are NOT written to credentials.json.
	_, err = fs.GetAuthTokens()
	assert.Error(t, err, "ephemeral login must not persist tokens to the file store")

	// The login email IS cached for future pre-fill.
	cached, err := fs.GetCachedEmail()
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", cached)
}

// StandardLogin pre-fills the Kas authenticator with the cached email; the
// cast mirrors the wiring in cmd.go which then sets ShouldPromptEmail.
func TestExternalNodeAuth_EphemeralLoginPreFillsEmailPrompt(t *testing.T) {
	authenticator := auth.StandardLogin("", "cached@example.com", nil)
	kas, ok := authenticator.(auth.KasAuthenticator)
	require.True(t, ok, "StandardLogin with a cached email must return a KasAuthenticator for pre-fill")
	assert.Equal(t, "cached@example.com", kas.Email)
}

type mockNodeAuthOAuth struct {
	loginTokens *auth.LoginTokens
}

func (m mockNodeAuthOAuth) GetCredentialProvider() entity.CredentialProvider {
	return "mock"
}

func (m mockNodeAuthOAuth) IsTokenValid(string) bool {
	return true
}

func (m mockNodeAuthOAuth) DoDeviceAuthFlow(_ func(string, string)) (*auth.LoginTokens, error) {
	return m.loginTokens, nil
}

func (m mockNodeAuthOAuth) GetNewAuthTokensWithRefresh(string) (*entity.AuthTokens, error) {
	return nil, nil
}
