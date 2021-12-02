package store

import (
	"fmt"
	"io"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/spf13/afero"
)

type SSHConfigFileStore struct {
	AuthHTTPStore
}

func (b *AuthHTTPStore) WithSSHConfigFile() *SSHConfigFileStore {
	return &SSHConfigFileStore{*b}
}

// !! need something to resolve file path of user ssh
func (f SSHConfigFileStore) GetSSHConfig() (string, error) {
	file, err := files.GetOrCreateSSHConfigFile(f.fs)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	buf := new(strings.Builder)
	_, err = io.Copy(buf, file)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return buf.String(), nil
}

func (f SSHConfigFileStore) WriteSSHConfig(config string) error {
	csp, err := files.GetUserSSHConfigPath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = afero.WriteFile(f.fs, *csp, []byte(config), 0o644)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (f SSHConfigFileStore) CreateNewSSHConfigBackup() error {
	backupFilePath, err := files.GetNewBackupSSHConfigFilePath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	csp, err := files.GetUserSSHConfigPath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	file, err := f.fs.Open(*csp)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	buf := new(strings.Builder)
	_, err = io.Copy(buf, file)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = afero.WriteFile(f.fs, *backupFilePath, []byte(buf.String()), 0o644)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fmt.Printf("Editing ssh config, backed up at path %s\n", *backupFilePath)
	return nil
}

func (f SSHConfigFileStore) WritePrivateKey(pem string) error {
	err := files.WriteSSHPrivateKey(f.fs, pem)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (f SSHConfigFileStore) GetPrivateKeyFilePath() string {
	return files.GetSSHPrivateKeyFilePath()
}
