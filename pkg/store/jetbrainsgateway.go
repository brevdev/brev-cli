package store

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/spf13/afero"
)

const (
	JetbrainsGatewayConfigFileName = "sshConfigs.xml"
)

func getJebrainsConfigDir(home string) (string, error) {
	var infix string
	infixSuffix := "Jetbrains"
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

	path := filepath.Join(home, infix)
	filePaths, err := getDirsWithPrefix(path, "JetBrainsGateway")
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	if len(filePaths) == 0 {
		return "", errors.New("could not find jetbrains gateway path")
	}
	jbgwPath := filePaths[len(filePaths)-1]

	return filepath.Join(jbgwPath, "options"), nil
}

func getDirsWithPrefix(parentPath string, prefix string) ([]string, error) {
	var files []string

	err := filepath.Walk(parentPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		if info.IsDir() && strings.HasPrefix(info.Name(), prefix) {
			files = append(files, path)
		}

		return nil
	})
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	sort.Strings(files)

	return files, nil
}

func (f FileStore) GetJetBrainsConfigPath() (string, error) {
	home, err := f.UserHomeDir()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	configDir, err := getJebrainsConfigDir(home)
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
	home, err := f.UserHomeDir()
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	_, err = getJebrainsConfigDir(home)
	if err != nil && strings.Contains(err.Error(), "could not find jetbrains gateway path") {
		return false, nil
	}
	return true, nil
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
