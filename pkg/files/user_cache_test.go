package files

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadUserCache_MissingFile(t *testing.T) {
	fs := afero.NewMemMapFs()
	cache := ReadUserCache(fs, "/home/test/.brev/user_cache.json")
	assert.Equal(t, "", cache.Email)
	assert.Equal(t, "", cache.LinuxUser)
}

func TestReadUserCache_MalformedJSON(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/home/test/.brev/user_cache.json"
	require.NoError(t, fs.MkdirAll("/home/test/.brev", 0o755))
	require.NoError(t, afero.WriteFile(fs, path, []byte("{invalid"), 0o600))

	cache := ReadUserCache(fs, path)
	assert.Equal(t, "", cache.Email)
	assert.Equal(t, "", cache.LinuxUser)
}

func TestWriteUserCache_CreatesDir(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/home/test/.brev/user_cache.json"

	err := WriteUserCache(fs, path, &UserCache{Email: "a@b.com"})
	require.NoError(t, err)

	cache := ReadUserCache(fs, path)
	assert.Equal(t, "a@b.com", cache.Email)
}

func TestUserCache_RoundTrip(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/home/test/.brev/user_cache.json"

	original := &UserCache{Email: "user@example.com", LinuxUser: "ubuntu"}
	require.NoError(t, WriteUserCache(fs, path, original))

	loaded := ReadUserCache(fs, path)
	assert.Equal(t, "user@example.com", loaded.Email)
	assert.Equal(t, "ubuntu", loaded.LinuxUser)
}

func TestUserCache_PartialUpdate_PreservesOtherField(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/home/test/.brev/user_cache.json"

	require.NoError(t, WriteUserCache(fs, path, &UserCache{Email: "user@example.com", LinuxUser: "ubuntu"}))

	// Update only email.
	cache := ReadUserCache(fs, path)
	cache.Email = "new@example.com"
	require.NoError(t, WriteUserCache(fs, path, cache))

	result := ReadUserCache(fs, path)
	assert.Equal(t, "new@example.com", result.Email)
	assert.Equal(t, "ubuntu", result.LinuxUser)
}

func TestUserCachePath(t *testing.T) {
	got := UserCachePath("/home/test/.brev")
	assert.Equal(t, "/home/test/.brev/user_cache.json", got)
}
