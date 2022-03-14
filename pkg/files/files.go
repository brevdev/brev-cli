package files

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/google/uuid"
	"github.com/spf13/afero"
)

const (
	brevDirectory = ".brev"
	// This might be better as a context.json??
	activeOrgFile      = "active_org.json"
	orgCacheFile       = "org_cache.json"
	workspaceCacheFile = "workspace_cache.json"
	// WIP: This will be used to let people "brev open" with editors other than VS Code
	personalSettingsCache         = "personal_settings.json"
	kubeCertFileName              = "brev.crt"
	sshPrivateKeyFileName         = "brev.pem"
	backupSSHConfigFileNamePrefix = "config.bak"
	tailscaleOutFileName          = "tailscale_out.log"
	sshPrivateKeyFilePermissions  = 0o600
	defaultFilePermission         = 0o770
)

var AppFs = afero.NewOsFs()

func GetBrevDirectory() string {
	return brevDirectory
}

func GetActiveOrgFile() string {
	return activeOrgFile
}

func GetPersonalSettingsCache() string {
	return personalSettingsCache
}

func GetOrgCacheFile() string {
	return orgCacheFile
}

func GetWorkspaceCacheFile() string {
	return workspaceCacheFile
}

func GetKubeCertFileName() string {
	return kubeCertFileName
}

func GetSSHPrivateKeyFileName() string {
	return sshPrivateKeyFileName
}

func GetTailScaleOutFileName() string {
	return tailscaleOutFileName
}

func GetNewBackupSSHConfigFileName() string {
	return fmt.Sprintf("%s.%s", backupSSHConfigFileNamePrefix, uuid.New())
}

func BuildBrevHome(userHome string) error {
	brevHome, err := GetBrevHome(userHome)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if err := os.MkdirAll(brevHome, defaultFilePermission); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func makeBrevFilePath(filename string, userHome string) (*string, error) {
	brevHome, err := GetBrevHome(userHome)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	fpath := filepath.Join(brevHome, filename)

	return &fpath, nil
}

func GetBrevHome(userHome string) (string, error) {
	return filepath.Join(userHome, brevDirectory), nil
}

func GetActiveOrgsPath(home string) (string, error) {
	fpath, err := makeBrevFilePath(activeOrgFile, home)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return *fpath, nil
}

func GetPersonalSettingsCachePath(home string) (string, error) {
	fpath, err := makeBrevFilePath(personalSettingsCache, home)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return *fpath, nil
}

func GetSSHPrivateKeyPath(home string) (string, error) {
	fpath, err := makeBrevFilePath(GetSSHPrivateKeyFileName(), home)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return *fpath, nil
}

func GetUserSSHConfigPath(home string) (string, error) {
	sshConfigPath := filepath.Join(home, ".ssh", "config")
	return sshConfigPath, nil
}

func GetBrevSSHConfigPath(home string) (string, error) {
	path, err := GetBrevHome(home)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	brevSSHConfigPath := filepath.Join(path, "ssh_config")
	return brevSSHConfigPath, nil
}

func GetNewBackupSSHConfigFilePath(home string) (*string, error) {
	fp, err := makeBrevFilePath(GetNewBackupSSHConfigFileName(), home)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return fp, nil
}

func GetTailScaleOutFilePath(home string) (*string, error) {
	fp, err := makeBrevFilePath(GetTailScaleOutFileName(), home)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return fp, nil
}

func GetOrCreateSSHConfigFile(fs afero.Fs, home string) (afero.File, error) {
	sshConfigPath, err := GetUserSSHConfigPath(home)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	sshConfigExists, err := afero.Exists(fs, sshConfigPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	var file afero.File
	if sshConfigExists {
		file, err = fs.OpenFile(sshConfigPath, os.O_RDWR, 0o644)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
	} else {
		file, err = fs.Create(sshConfigPath)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
	}
	return file, nil
}

// ReadJSON reads data from a file into the given struct
//
// Usage:
//   var foo myStruct
//   files.ReadJSON("tmp/a.json", &foo)
func ReadJSON(fs afero.Fs, unsafeFilePathString string, v interface{}) error {
	safeFilePath := filepath.Clean(unsafeFilePathString)
	f, err := fs.Open(safeFilePath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	dataBytes, err := afero.ReadAll(f)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = json.Unmarshal(dataBytes, v)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if err = f.Close(); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func ReadString(unsafeFilePathString string) (string, error) {
	safeFilePath := filepath.Clean(unsafeFilePathString)
	f, err := os.Open(safeFilePath)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	dataBytes, err := ioutil.ReadAll(f)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	if err = f.Close(); err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	return string(dataBytes), nil
	// fmt.Println(dataBytes)
	// fmt.Println(string(dataBytes))
}

// OverwriteJSON data in the target file with data from the given struct
//
// Usage (unstructured):
//   OverwriteJSON("tmp/a/b/c.json", map[string]string{
// 	    "hi": "there",
//   })
//
//
// Usage (struct):
//   var foo myStruct
//   OverwriteJSON("tmp/a/b/c.json", foo)
func OverwriteJSON(filepath string, v interface{}) error {
	f, err := touchFile(filepath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// clear
	err = f.Truncate(0)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// write
	dataBytes, err := json.Marshal(v)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = ioutil.WriteFile(filepath, dataBytes, os.ModePerm)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if err = f.Close(); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return breverrors.WrapAndTrace(err)
}

// OverwriteString data in the target file with data from the given string
//
// Usage
//   OverwriteString("tmp/a/b/c.txt", "hi there")
func OverwriteString(filepath string, data string) error {
	f, err := touchFile(filepath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// clear
	err = f.Truncate(0)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// write
	err = ioutil.WriteFile(filepath, []byte(data), os.ModePerm)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if err = f.Close(); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return breverrors.WrapAndTrace(err)
}

func WriteSSHPrivateKey(fs afero.Fs, data string, home string) error {
	pkPath, err := GetSSHPrivateKeyPath(home)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = afero.WriteFile(fs, pkPath, []byte(data), sshPrivateKeyFilePermissions)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

// Delete a single file altogether.
func DeleteFile(filepath string) error {
	err := os.Remove(filepath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

// Create file (and full path) if it does not already exit.
func touchFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), defaultFilePermission); err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return f, nil
}
