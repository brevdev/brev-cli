// user_cache.go wraps the shared files.UserCache helpers so that FileStore
// callers (notably cmd.go) go through the injected afero.Fs. This lets tests
// swap in a MemMapFs instead of hitting the real filesystem. The actual
// read/write logic lives in pkg/files/user_cache.go.
package store

import (
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
)

// GetUserCache reads the cache file, returning an empty cache if it doesn't exist.
func (f FileStore) GetUserCache() (*files.UserCache, error) {
	brevHome, err := f.GetBrevHomePath()
	if err != nil {
		return &files.UserCache{}, breverrors.WrapAndTrace(err)
	}
	return files.ReadUserCache(f.fs, files.UserCachePath(brevHome)), nil
}

// SaveUserCache writes the cache to ~/.brev/user_cache.json.
func (f FileStore) SaveUserCache(cache *files.UserCache) error {
	brevHome, err := f.GetBrevHomePath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if err := files.WriteUserCache(f.fs, files.UserCachePath(brevHome), cache); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

// GetCachedEmail returns the cached email, or "" if none exists.
func (f FileStore) GetCachedEmail() (string, error) {
	cache, err := f.GetUserCache()
	if err != nil {
		return "", err
	}
	return cache.Email, nil
}

// SaveCachedEmail updates the email field in the cache.
func (f FileStore) SaveCachedEmail(email string) error {
	cache, err := f.GetUserCache()
	if err != nil {
		cache = &files.UserCache{}
	}
	cache.Email = email
	return f.SaveUserCache(cache)
}

// GetCachedLinuxUser returns the cached linux username, or "" if none exists.
func (f FileStore) GetCachedLinuxUser() (string, error) {
	cache, err := f.GetUserCache()
	if err != nil {
		return "", err
	}
	return cache.LinuxUser, nil
}

// SaveCachedLinuxUser updates the linux user field in the cache.
func (f FileStore) SaveCachedLinuxUser(linuxUser string) error {
	cache, err := f.GetUserCache()
	if err != nil {
		cache = &files.UserCache{}
	}
	cache.LinuxUser = linuxUser
	return f.SaveUserCache(cache)
}
