package store

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/spf13/afero"
)

const (
	JetbrainsGatewayConfigFileName = "sshConfigs.xml"
)

func getJebrainsConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	var infix string
	infixSuffix := filepath.Join("JetBrains", "JetBrainsGateway2021.3", "options")
	switch runtime.GOOS {
	case "windows":
		return "", errors.New("windows not supported at this time")
	case "linux":
		infix = filepath.Join(".config", infixSuffix)
	case "darwin":
		infix = filepath.Join("Library", "Application Support", infixSuffix)
	default:
		return "", fmt.Errorf("invalid goos")
	}
	return filepath.Join(home, infix), nil
}

func (f FileStore) GetJetBrainsConfigPath() (string, error) {
	configDir, err := getJebrainsConfigDir()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	sshConfigPath := filepath.Join(configDir, JetbrainsGatewayConfigFileName)
	return sshConfigPath, nil
}

func (f FileStore) GetJetBrainsConfig() (string, error) {
	path, err := f.GetJetBrainsConfigPath()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
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

func (f FileStore) DoesJetbrainsFilePathExist() (bool, error) {
	path, err := getJebrainsConfigDir()
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	_, err = os.Stat(path)
	return !os.IsNotExist(err), nil
}

func (f FileStore) WriteJetBrainsConfig(config string) error {
	path, err := f.GetJetBrainsConfigPath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = afero.WriteFile(f.fs, path, []byte(config), 0o644)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
