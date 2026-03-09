package files

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

// UserCache holds cached user preferences persisted to ~/.brev/user_cache.json.
type UserCache struct {
	Email     string `json:"email,omitempty"`
	LinuxUser string `json:"linux_user,omitempty"`
}

// UserCachePath returns the path to the user cache file within the given
// brev home directory (e.g. ~/.brev).
func UserCachePath(brevHome string) string {
	return filepath.Join(brevHome, userCacheFileName)
}

// ReadUserCache reads the cache from the given filesystem, returning an empty
// cache if the file doesn't exist or is malformed.
func ReadUserCache(fs afero.Fs, path string) *UserCache {
	data, err := afero.ReadFile(fs, path)
	if err != nil {
		return &UserCache{}
	}
	var cache UserCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return &UserCache{}
	}
	return &cache
}

// WriteUserCache writes the cache to the given filesystem.
func WriteUserCache(fs afero.Fs, path string, cache *UserCache) error {
	if err := fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling user cache: %w", err)
	}
	if err := afero.WriteFile(fs, path, data, 0o600); err != nil {
		return fmt.Errorf("writing user cache: %w", err)
	}
	return nil
}

// ReadDefaultUserCache reads the user cache from the default OS filesystem
// and home directory. Returns an empty cache on any error.
func ReadDefaultUserCache() *UserCache {
	home, err := os.UserHomeDir()
	if err != nil {
		return &UserCache{}
	}
	return ReadUserCache(AppFs, UserCachePath(GetBrevHome(home)))
}

// WriteDefaultUserCache writes to the default OS filesystem and home directory.
func WriteDefaultUserCache(cache *UserCache) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	_ = WriteUserCache(AppFs, UserCachePath(GetBrevHome(home)), cache)
}
