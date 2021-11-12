package files

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/spf13/afero"
)

const (
	brevDirectory = ".brev"
	// This might be better as a context.json??
	activeOrgFile                 = "active_org.json"
	orgCacheFile                  = "org_cache.json"
	workspaceCacheFile            = "workspace_cache.json"
	kubeCertFileName              = "brev.crt"
	sshPrivateKeyFileName         = "brev.pem"
	backupSSHConfigFileNamePrefix = "config.bak"
	sshPrivateKeyFilePermissions  = 0600
	defaultFilePermission         = 0770
)

var AppFs = afero.NewOsFs()

func GetBrevDirectory() string {
	return brevDirectory
}

func GetActiveOrgFile() string {
	return activeOrgFile
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

func GetNewBackupSSHConfigFileName() string {
	return fmt.Sprintf("%s.%s", backupSSHConfigFileNamePrefix, uuid.New())
}

func makeBrevFilePath(filename string) (*string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	fpath, err := filepath.Join(home, brevDirectory, filename), nil
	if err != nil {
		return nil, err
	}
	return &fpath, nil
}

func makeBrevFilePathOrPanic(filename string) string {
	fpath, err := makeBrevFilePath(filename)
	if err != nil {
		log.Fatal(err)
	}
	return *fpath
}

func GetWorkspacesCacheFilePath() string {
	return makeBrevFilePathOrPanic(workspaceCacheFile)
}

func GetOrgCacheFilePath() string {
	return makeBrevFilePathOrPanic(orgCacheFile)
}

func GetActiveOrgsPath() string {
	return makeBrevFilePathOrPanic(activeOrgFile)
}

func GetCertFilePath() string {
	return makeBrevFilePathOrPanic(GetKubeCertFileName())
}

func GetSSHPrivateKeyFilePath() string {
	return makeBrevFilePathOrPanic(GetSSHPrivateKeyFileName())
}

func GetUserSSHConfigPath() (*string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	sshConfigPath := filepath.Join(home, ".ssh", "config")
	return &sshConfigPath, nil
}

func GetNewBackupSSHConfigFilePath() (*string, error) {
	fp, err := makeBrevFilePath(GetNewBackupSSHConfigFileName())
	if err != nil {
		return nil, err
	}
	return fp, nil
}

func GetOrCreateSSHConfigFile(fs afero.Fs) (afero.File, error) {
	sshConfigPath, err := GetUserSSHConfigPath()
	if err != nil {
		return nil, err
	}
	sshConfigExists, err := Exists(*sshConfigPath, false)
	if err != nil {
		return nil, err
	}
	var file afero.File
	if sshConfigExists {
		file, err = AppFs.OpenFile(*sshConfigPath, os.O_RDWR, 0644)
		if err != nil {
			return nil, err
		}
	} else {
		file, err = AppFs.Create(*sshConfigPath)
		if err != nil {
			return nil, err
		}
	}
	return file, nil
}

func Exists(filepath string, isDir bool) (bool, error) {
	info, err := os.Stat(filepath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if info == nil {
		return false, fmt.Errorf("Could not stat file %s", filepath)
	}
	if info.IsDir() {
		// error?
		if isDir {
			return true, nil
		}
		return false, nil
	}
	return true, nil
}

// ReadJSON reads data from a file into the given struct
//
// Usage:
//   var foo myStruct
//   files.ReadJSON("tmp/a.json", &foo)
func ReadJSON(unsafeFilePathString string, v interface{}) error {
	safeFilePath := filepath.Clean(unsafeFilePathString)
	f, err := os.Open(safeFilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	dataBytes, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	return json.Unmarshal(dataBytes, v)
}

func ReadString(unsafeFilePathString string) (string, error) {
	safeFilePath := filepath.Clean(unsafeFilePathString)
	f, err := os.Open(safeFilePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	dataBytes, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
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
		return nil
	}
	defer f.Close()

	// clear
	err = f.Truncate(0)

	// write
	dataBytes, err := json.Marshal(v)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filepath, dataBytes, os.ModePerm)

	return err
}

// OverwriteString data in the target file with data from the given string
//
// Usage
//   OverwriteString("tmp/a/b/c.txt", "hi there")
func OverwriteString(filepath string, data string) error {
	f, err := touchFile(filepath)
	if err != nil {
		return nil
	}
	defer f.Close()

	// clear
	err = f.Truncate(0)

	// write
	err = ioutil.WriteFile(filepath, []byte(data), os.ModePerm)

	return err
}

func WriteSSHPrivateKey(fs afero.Fs, data string) error {
	// write
	return afero.WriteFile(fs, GetSSHPrivateKeyFilePath(), []byte(data), sshPrivateKeyFilePermissions)
}

// Delete a single file altogether.
func DeleteFile(filepath string) error {
	error := os.Remove(filepath)
	if error != nil {
		return nil
	}
	return error
}

// Create file (and full path) if it does not already exit.
func touchFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), defaultFilePermission); err != nil {
		return nil, err
	}
	return os.Create(path)
}
