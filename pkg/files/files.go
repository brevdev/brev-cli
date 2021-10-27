package files

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
)

const (
	brevDirectory = ".brev"
	// This might be better as a context.json??
	activeOrgFile         = "active_org.json"
	orgCacheFile          = "org_cache.json"
	workspaceCacheFile    = "workspace_cache.json"
	kubeCertFileName      = "brev.crt"
	sshPrivateKeyFileName = "brev.pem"
)

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

func GetHomeDir() string {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	return usr.HomeDir
}

func GetWorkspacesCacheFilePath() string {
	rootDir := GetHomeDir()

	return fmt.Sprintf("%s/%s/%s", rootDir, brevDirectory, workspaceCacheFile)
}

func GetOrgCacheFilePath() string {
	rootDir := GetHomeDir()

	return fmt.Sprintf("%s/%s/%s", rootDir, brevDirectory, orgCacheFile)
}

func GetActiveOrgsPath() string {
	rootDir := GetHomeDir()

	return fmt.Sprintf("%s/%s/%s", rootDir, brevDirectory, activeOrgFile)
}

func GetCertFilePath() string {
	home, _ := os.UserHomeDir()
	return home + "/" + GetBrevDirectory() + "/" + GetKubeCertFileName()
}

func GetSSHPrivateKeyFilePath() string {
	home, _ := os.UserHomeDir()
	return home + "/" + GetBrevDirectory() + "/" + GetSSHPrivateKeyFileName()
}

func Exists(filepath string, isDir bool) (bool, error) {
	info, err := os.Stat(filepath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if info == nil {
		return false, errors.New(fmt.Sprintf("Could not stat file %s", filepath))
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
func ReadJSON(filepath string, v interface{}) error {
	f, err := os.Open(filepath)
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

func ReadString(filepath string) (string, error) {
	f, err := os.Open(filepath)
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

// OverwriteString data in the target file with data from the given string
//
// Usage
//   OverwriteString("tmp/a/b/c.txt", "hi there")
func WriteSSHPrivateKey(data string) error {
	// write
	err := ioutil.WriteFile(GetSSHPrivateKeyFilePath(), []byte(data), 0600)
	if err := os.Chmod(GetSSHPrivateKeyFilePath(), 0600); err != nil {
		log.Fatal(err)
	}

	return err
}

// Delete a single file altogether
func DeleteFile(filepath string) error {
	error := os.Remove(filepath)
	if error != nil {
		return nil
	}
	return error
}

// Create file (and full path) if it does not already exit
func touchFile(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0770); err != nil {
		return nil, err
	}
	return os.Create(path)
}
