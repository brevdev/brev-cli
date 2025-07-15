package files

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"golang.org/x/text/encoding/charmap"

	"github.com/google/uuid"
	"github.com/spf13/afero"
)

type PersonalSettings struct {
	DefaultEditor string `json:"default_editor"`
}

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

func GetSSHPrivateKeyFileName() string {
	return sshPrivateKeyFileName
}

func GetNewBackupSSHConfigFileName() string {
	return fmt.Sprintf("%s.%s", backupSSHConfigFileNamePrefix, uuid.New())
}

func BuildBrevHome(fs afero.Fs, userHome string) error {
	brevHome := GetBrevHome(userHome)
	if err := fs.MkdirAll(brevHome, defaultFilePermission); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func makeBrevFilePath(filename string, userHome string) string {
	brevHome := GetBrevHome(userHome)
	fpath := filepath.Join(brevHome, filename)
	return fpath
}

func GetBrevHome(userHome string) string {
	return filepath.Join(userHome, brevDirectory)
}

func GetActiveOrgsPath(home string) string {
	fpath := makeBrevFilePath(activeOrgFile, home)
	return fpath
}

func GetSSHPrivateKeyPath(home string) string {
	fpath := makeBrevFilePath(GetSSHPrivateKeyFileName(), home)
	return fpath
}

func GetUserSSHConfigPath(home string) (string, error) {
	sshConfigPath := filepath.Join(home, ".ssh", "config")
	return sshConfigPath, nil
}

func GetBrevSSHConfigPath(home string) string {
	path := GetBrevHome(home)
	brevSSHConfigPath := filepath.Join(path, "ssh_config")
	return brevSSHConfigPath
}

func GetBrevCloudflaredBinaryPath(home string) string {
	path := GetBrevHome(home)
	brevCloudflaredBinaryPath := filepath.Join(path, "cloudflared")
	return brevCloudflaredBinaryPath
}

func GetOnboardingStepPath(home string) string {
	path := GetBrevHome(home)
	brevOnboardingFilePath := filepath.Join(path, "onboarding_step.json")
	return brevOnboardingFilePath
}

func GetPersonalSettingsPath(home string) string {
	fpath := makeBrevFilePath(personalSettingsCache, home)
	return fpath
}

func GetNewBackupSSHConfigFilePath(home string) string {
	fp := makeBrevFilePath(GetNewBackupSSHConfigFileName(), home)

	return fp
}

// ReadJSON reads data from a file into the given struct
//
// Usage:
//
//	var foo myStruct
//	files.ReadJSON("tmp/a.json", &foo)
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

func ReadString(fs afero.Fs, unsafeFilePathString string) (string, error) {
	safeFilePath := filepath.Clean(unsafeFilePathString)
	f, err := fs.Open(safeFilePath)
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
//
//	  OverwriteJSON("tmp/a/b/c.json", map[string]string{
//		    "hi": "there",
//	  })
//
// Usage (struct):
//
//	var foo myStruct
//	OverwriteJSON("tmp/a/b/c.json", foo)
func OverwriteJSON(fs afero.Fs, filepath string, v interface{}) error {
	f, err := touchFile(fs, filepath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// clear
	err = f.Truncate(0)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// write
	dataBytes, err := json.MarshalIndent(v, "", " ")
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
//
//	OverwriteString("tmp/a/b/c.txt", "hi there")

// clear

// write

func WriteSSHPrivateKey(fs afero.Fs, data string, home string) error {
	pkPath := GetSSHPrivateKeyPath(home)
	err := fs.MkdirAll(filepath.Dir(pkPath), defaultFilePermission)
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
func DeleteFile(fs afero.Fs, filepath string) error {
	err := fs.Remove(filepath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

// Create file (and full path) if it does not already exit.
func touchFile(fs afero.Fs, path string) (afero.File, error) {
	if err := fs.MkdirAll(filepath.Dir(path), defaultFilePermission); err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	f, err := fs.Create(path)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return f, nil
}

func CatFile(filePath string) (string, error) {
	gocmd := exec.Command("cat", filePath) // #nosec G204
	in, err := gocmd.Output()
	if err != nil {
		return "", breverrors.Wrap(err, "error reading file "+filePath)
	} else {
		d := charmap.CodePage850.NewDecoder()
		out, err := d.Bytes(in)
		if err != nil {
			return "", breverrors.Wrap(err, "error reading file "+filePath)
		}
		return string(out), nil
	}
}

func ReadPersonalSettings(fs afero.Fs, home string) (*PersonalSettings, error) {
	settingsPath := GetPersonalSettingsPath(home)
	var settings PersonalSettings
	err := ReadJSON(fs, settingsPath, &settings)
	if err != nil {
		return &PersonalSettings{DefaultEditor: "code"}, nil
	}
	if settings.DefaultEditor == "" {
		settings.DefaultEditor = "code"
	}
	return &settings, nil
}

func WritePersonalSettings(fs afero.Fs, home string, settings *PersonalSettings) error {
	settingsPath := GetPersonalSettingsPath(home)
	return OverwriteJSON(fs, settingsPath, settings)
}

// if this doesn't work, just exit

// if this doesn't work, just exit
