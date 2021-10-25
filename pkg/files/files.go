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
	brevDirectory      = ".brev"
	activeProjectsFile = "active_projects.json"
	projectsFile       = "projects.json"
	endpointsFile      = "endpoints.json"
	// This might be better as a context.json??
	activeOrgFile         = "active_org.json"
	kubeCertFileName      = "brev.crt"
	sshPrivateKeyFileName = "brev.pem"
)

func GetBrevDirectory() string {
	return brevDirectory
}

func GetActiveProjectFile() string {
	return activeProjectsFile
}
func GetProjectsFile() string {
	return projectsFile
}
func GetEndpointsFile() string {
	return endpointsFile
}
func GetActiveOrgFile() string {
	return activeOrgFile
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

func GetActiveProjectsPath() string {
	rootDir := GetHomeDir()

	return fmt.Sprintf("%s/%s/%s", rootDir, brevDirectory, activeProjectsFile)
}

func GetLocalBrevDir() string {
	cwd, _ := os.Getwd()
	return fmt.Sprintf("%s/%s", cwd, brevDirectory)
}

func GetActiveOrgsPath() string {
	rootDir := GetHomeDir()

	return fmt.Sprintf("%s/%s/%s", rootDir, brevDirectory, activeOrgFile)
}
func GetEndpointsPath() string {
	cwd, _ := os.Getwd()
	return fmt.Sprintf("%s/%s/%s", cwd, brevDirectory, endpointsFile)
}
func GetProjectsPath() string {
	cwd, _ := os.Getwd()
	return fmt.Sprintf("%s/%s/%s", cwd, brevDirectory, projectsFile)
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
