package store

import (
	"os"
	"path/filepath"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/spf13/afero"
)

const cachedEmailFile = "cached-email"

// GetCachedEmail returns the previously cached login email, or "" if none exists.
func (f FileStore) GetCachedEmail() (string, error) {
	brevHome, err := f.GetBrevHomePath()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	data, err := afero.ReadFile(f.fs, filepath.Join(brevHome, cachedEmailFile))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", breverrors.WrapAndTrace(err)
	}
	return strings.TrimSpace(string(data)), nil
}

// SaveCachedEmail writes the login email to ~/.brev/cached-email (0600).
func (f FileStore) SaveCachedEmail(email string) error {
	brevHome, err := f.GetBrevHomePath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	path := filepath.Join(brevHome, cachedEmailFile)
	err = f.fs.MkdirAll(brevHome, 0o755)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = afero.WriteFile(f.fs, path, []byte(email), 0o600)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
