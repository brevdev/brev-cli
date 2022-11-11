package store

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/spf13/afero"
)

// !! need something to resolve file path of user ssh
func (f FileStore) GetUserSSHConfig() (string, error) {
	home, err := f.UserHomeDir()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	path, err := files.GetUserSSHConfigPath(home)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	if path == "" {
		return "", errors.New("nil path when getting ssh config")
	}
	file, err := f.GetOrCreateFile(path)
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

func (f FileStore) GetWSLUserSSHConfig() (string, error) {
	home, err := f.b.GetWSLHostHomeDir()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	path, err := files.GetUserSSHConfigPath(home)
	if path == "" {
		return "", errors.New("nil path when getting ssh config")
	}
	file, err := f.GetOrCreateFile(path)
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

func (f FileStore) GetUserSSHConfigPath() (string, error) {
	home, err := f.UserHomeDir()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	path, err := files.GetUserSSHConfigPath(home)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return path, nil
}

func (f FileStore) GetWSLHostUserSSHConfigPath() (string, error) {
	home, err := f.b.GetWSLHostHomeDir()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	path, err := files.GetUserSSHConfigPath(home)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return path, nil
}

func (f FileStore) GetBrevSSHConfigPath() (string, error) {
	home, err := f.UserHomeDir()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	path := files.GetBrevSSHConfigPath(home)
	return path, nil
}

func (f FileStore) GetWSLHostBrevSSHConfigPath() (string, error) {
	home, err := f.b.GetWSLHostHomeDir()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	path := files.GetBrevSSHConfigPath(home)
	return path, nil
}

func (f FileStore) WriteUserSSHConfig(config string) error {
	home, err := f.UserHomeDir()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	csp, err := files.GetUserSSHConfigPath(home)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = afero.WriteFile(f.fs, csp, []byte(config), 0o644)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (f FileStore) WriteWSLUserSSHConfig(config string) error {
	home, err := f.b.GetWSLHostHomeDir()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	csp, err := files.GetUserSSHConfigPath(home)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = afero.WriteFile(f.fs, csp, []byte(config), 0o644)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (f FileStore) WriteBrevSSHConfig(config string) error {
	home, err := f.UserHomeDir()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	bsp := files.GetBrevSSHConfigPath(home)

	err = f.fs.MkdirAll(filepath.Dir(bsp), 0o755)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = afero.WriteFile(f.fs, bsp, []byte(config), 0o644)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (f FileStore) WriteBrevSSHConfigWSL(config string) error {
	home, err := f.b.GetWSLHostHomeDir()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	path := files.GetBrevSSHConfigPath(home)
	err = f.fs.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = afero.WriteFile(f.fs, path, []byte(config), 0o644)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (f FileStore) CreateNewSSHConfigBackup() error {
	home, err := f.UserHomeDir()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	backupFilePath := files.GetNewBackupSSHConfigFilePath(home)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	csp, err := files.GetUserSSHConfigPath(home)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	file, err := f.fs.Open(csp)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	buf := new(strings.Builder)
	_, err = io.Copy(buf, file)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = afero.WriteFile(f.fs, backupFilePath, []byte(buf.String()), 0o644)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fmt.Printf("Editing ssh config, backed up at path %s\n", backupFilePath)
	return nil
}

func (f FileStore) WritePrivateKey(pem string) error {
	home, err2 := f.UserHomeDir()
	if err2 != nil {
		return breverrors.WrapAndTrace(err2)
	}
	err2 = files.WriteSSHPrivateKey(f.fs, pem, home)
	if err2 != nil {
		return breverrors.WrapAndTrace(err2)
	}
	// write ssh key to windows if possible
	windowsHome, err := f.b.GetWSLHostHomeDir()
	if err != nil {
		return nil
	}
	err = files.WriteSSHPrivateKey(f.fs, pem, windowsHome)
	if err != nil {
		return nil
	}
	return nil
}

func (f FileStore) GetPrivateKeyPath() (string, error) {
	home, err := f.UserHomeDir()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	path := files.GetSSHPrivateKeyPath(home)

	return path, nil
}

func VerifyPrivateKey(key []byte) error {
	block, rest := pem.Decode(key)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return errors.New("failed to decode PEM block")
	}
	if len(rest) > 0 {
		return errors.New("extra data in key")
	}
	_, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
