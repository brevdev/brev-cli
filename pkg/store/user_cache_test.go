package store

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestFileStore(t *testing.T) FileStore {
	t.Helper()
	fs := afero.NewMemMapFs()
	err := fs.MkdirAll("/home/testuser/.brev", 0o755)
	require.NoError(t, err)
	return FileStore{
		b:                 BasicStore{},
		fs:                fs,
		userHomeDirGetter: func() (string, error) { return "/home/testuser", nil },
	}
}

func TestCachedEmail_RoundTrip(t *testing.T) {
	s := newTestFileStore(t)

	err := s.SaveCachedEmail("user@example.com")
	require.NoError(t, err)

	email, err := s.GetCachedEmail()
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", email)
}

func TestCachedEmail_MissingFile(t *testing.T) {
	s := newTestFileStore(t)

	email, err := s.GetCachedEmail()
	require.NoError(t, err)
	assert.Equal(t, "", email)
}

func TestCachedEmail_Overwrites(t *testing.T) {
	s := newTestFileStore(t)

	require.NoError(t, s.SaveCachedEmail("first@example.com"))
	require.NoError(t, s.SaveCachedEmail("second@example.com"))

	email, err := s.GetCachedEmail()
	require.NoError(t, err)
	assert.Equal(t, "second@example.com", email)
}

func TestCachedLinuxUser_RoundTrip(t *testing.T) {
	s := newTestFileStore(t)

	err := s.SaveCachedLinuxUser("ubuntu")
	require.NoError(t, err)

	user, err := s.GetCachedLinuxUser()
	require.NoError(t, err)
	assert.Equal(t, "ubuntu", user)
}

func TestCachedLinuxUser_MissingFile(t *testing.T) {
	s := newTestFileStore(t)

	user, err := s.GetCachedLinuxUser()
	require.NoError(t, err)
	assert.Equal(t, "", user)
}

func TestUserCache_BothFields(t *testing.T) {
	s := newTestFileStore(t)

	require.NoError(t, s.SaveCachedEmail("user@example.com"))
	require.NoError(t, s.SaveCachedLinuxUser("ubuntu"))

	email, err := s.GetCachedEmail()
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", email)

	user, err := s.GetCachedLinuxUser()
	require.NoError(t, err)
	assert.Equal(t, "ubuntu", user)
}

func TestUserCache_SaveOneDoesNotClobberOther(t *testing.T) {
	s := newTestFileStore(t)

	require.NoError(t, s.SaveCachedEmail("user@example.com"))
	require.NoError(t, s.SaveCachedLinuxUser("ubuntu"))

	// Overwrite email — linux user should survive.
	require.NoError(t, s.SaveCachedEmail("new@example.com"))

	email, err := s.GetCachedEmail()
	require.NoError(t, err)
	assert.Equal(t, "new@example.com", email)

	user, err := s.GetCachedLinuxUser()
	require.NoError(t, err)
	assert.Equal(t, "ubuntu", user)
}
