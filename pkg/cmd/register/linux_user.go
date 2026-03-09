package register

import (
	"github.com/brevdev/brev-cli/pkg/files"
)

// GetCachedLinuxUser returns the previously cached linux username, or "" if none exists.
func GetCachedLinuxUser() string {
	return files.ReadDefaultUserCache().LinuxUser
}

// SaveCachedLinuxUser writes the linux username to the user cache.
func SaveCachedLinuxUser(username string) {
	c := files.ReadDefaultUserCache()
	c.LinuxUser = username
	files.WriteDefaultUserCache(c)
}
