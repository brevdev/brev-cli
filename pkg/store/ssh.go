package store

import (
	"fmt"
	"io"
	"strings"

	"github.com/brevdev/brev-cli/pkg/brev_errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/spf13/afero"
)

// !! need something to resolve file path of user ssh
func (f FileStore) GetSSHConfig() (string, error) {
	file, err := files.GetOrCreateSSHConfigFile(f.fs)
	if err != nil {
		return "", brev_errors.WrapAndTrace(err)
	}

	buf := new(strings.Builder)
	_, err = io.Copy(buf, file)
	if err != nil {
		return "", brev_errors.WrapAndTrace(err)
	}
	return buf.String(), nil
}

func (f FileStore) WriteSSHConfig(config string) error {
	csp, err := files.GetUserSSHConfigPath()
	if err != nil {
		return brev_errors.WrapAndTrace(err)
	}
	err = afero.WriteFile(f.fs, *csp, []byte(config), 0644)
	if err != nil {
		return brev_errors.WrapAndTrace(err)
	}
	return nil
}

func (f FileStore) CreateNewSSHConfigBackup() error {
	backupFilePath, err := files.GetNewBackupSSHConfigFilePath()
	if err != nil {
		return brev_errors.WrapAndTrace(err)
	}

	csp, err := files.GetUserSSHConfigPath()
	if err != nil {
		return brev_errors.WrapAndTrace(err)
	}
	file, err := f.fs.Open(*csp)
	if err != nil {
		return brev_errors.WrapAndTrace(err)
	}

	buf := new(strings.Builder)
	_, err = io.Copy(buf, file)
	if err != nil {
		return brev_errors.WrapAndTrace(err)
	}

	err = afero.WriteFile(f.fs, *backupFilePath, []byte(buf.String()), 0644)
	if err != nil {
		return brev_errors.WrapAndTrace(err)
	}

	fmt.Printf("Editing ssh config, backed up at path %s\n", *backupFilePath)
	return nil
}

func (f FileStore) WritePrivateKey(pem string) error {
	err := files.WriteSSHPrivateKey(f.fs, pem)
	if err != nil {
		return brev_errors.WrapAndTrace(err)
	}
	return nil
}
