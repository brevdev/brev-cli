package store

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/spf13/afero"
	user "github.com/tweekmonster/luser"
	"golang.org/x/text/encoding/charmap"
)

type FileStore struct {
	b                 BasicStore
	fs                afero.Fs
	User              *user.User
	userHomeDirGetter func() (string, error)
}

func (f *FileStore) GetWindowsDir() (string, error) {
	return f.b.GetWSLHostHomeDir()
}

func (f *FileStore) WithUserHomeDirGetter(userHomeDirGetter func() (string, error)) *FileStore {
	f.userHomeDirGetter = userHomeDirGetter
	return f
}

func (b *BasicStore) WithFileSystem(fs afero.Fs) *FileStore {
	f := &FileStore{
		b:    *b,
		fs:   fs,
		User: nil,
	}
	return f.WithUserHomeDirGetter(getUserHomeDir(f))
}

func (f *FileStore) WithUserID(userID string) (*FileStore, error) {
	var userToConfigure *user.User
	var err error
	userToConfigure, err = user.Lookup(userID)
	if err != nil {
		_, ok := err.(*user.UnknownUserError)
		if !ok {
			userToConfigure, err = user.LookupId(userID)
			if err != nil {
				return nil, breverrors.WrapAndTrace(err)
			}
		} else {
			return nil, breverrors.WrapAndTrace(err)
		}
	}
	f.User = userToConfigure
	return f, nil
}

func (f FileStore) GetOrCreateFile(path string) (afero.File, error) {
	fileExists, err := afero.Exists(f.fs, path)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	var file afero.File
	if fileExists {
		file, err = f.fs.OpenFile(path, os.O_RDWR, 0o644)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
	} else {
		if err = f.fs.MkdirAll(filepath.Dir(path), 0o775); err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		file, err = f.fs.Create(path)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
	}
	return file, nil
}

func (f FileStore) FileExists(filepath string) (bool, error) {
	fileExists, err := afero.Exists(f.fs, filepath)
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	return fileExists, nil
}

func (f FileStore) GetDotGitConfigFile(path string) (string, error) {
	dotGitConfigFile := filepath.Join(path, ".git", "config")

	dotGitConfigExists, err := afero.Exists(f.fs, dotGitConfigFile)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	var file afero.File
	if dotGitConfigExists {
		file, err = f.fs.OpenFile(dotGitConfigFile, os.O_RDWR, 0o644)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
	} else {
		return "", breverrors.WrapAndTrace(errors.New("couldn't verify if " + dotGitConfigFile + " is an active git directory"))
	}

	buf := new(strings.Builder)
	_, err = io.Copy(buf, file)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return buf.String(), nil
}

type Dependencies struct {
	Rust   string
	Java   string
	Node   string
	TS     string
	Go     string
	Solana string
}

func (f FileStore) GetDependenciesForImport(path string) (*Dependencies, error) {
	deps := &Dependencies{
		Rust:   "",
		Java:   "",
		Node:   "",
		TS:     "",
		Go:     "",
		Solana: "",
	}

	// Check Golang
	filePath := filepath.Join(path, "go.mod")
	gocmd := exec.Command("cat", filePath) // #nosec G204
	in, err := gocmd.Output()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err, "error reading go.mod")
	} else {
		d := charmap.CodePage850.NewDecoder()
		out, err := d.Bytes(in)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err, "error reading go.mod")
		} else if len(string(out)) > 0 {
			for _, v := range strings.Split(string(out), "\n") {
				if strings.HasPrefix(v, "go ") {
					versionNum := strings.Split(v, "go ")
					deps.Go = versionNum[1]
				}
			}
		}
	}

	return deps, nil
}

type WorkspaceMeta struct {
	WorkspaceID      string `json:"workspaceId"`
	WorkspaceGroupID string `json:"workspaceGroupId"`
	UserID           string `json:"userId"`
	OrganizationID   string `json:"organizationId"`
}

// GetCurrentWorkspaceID will return an empty string when
// not in a brev workspace, and otherwise return its id
func (f FileStore) GetCurrentWorkspaceID() (string, error) {
	meta, err := f.GetCurrentWorkspaceMeta()
	if err != nil {
		return "", nil
	}
	return meta.WorkspaceID, nil
}

func (f FileStore) IsWorkspace() (bool, error) {
	id, err := f.GetCurrentWorkspaceID()
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	return id != "", nil
}

func (f FileStore) GetCurrentWorkspaceGroupID() (string, error) {
	meta, err := f.GetCurrentWorkspaceMeta()
	if err != nil {
		return "", nil
	}
	return meta.WorkspaceGroupID, nil
}

func (f FileStore) GetCurrentWorkspaceMeta() (*WorkspaceMeta, error) {
	file, err := f.fs.Open("/etc/meta/workspace.json")
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	fileB, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	var wm WorkspaceMeta
	err = json.Unmarshal(fileB, &wm)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	err = file.Close()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if wm.WorkspaceID != "" {
		breverrors.GetDefaultErrorReporter().AddTag("workspaceId", wm.WorkspaceID)
		breverrors.GetDefaultErrorReporter().AddTag("organizationId", wm.OrganizationID)
		breverrors.GetDefaultErrorReporter().AddTag("workspaceGroupId", wm.WorkspaceGroupID)
	}

	return &wm, nil
}

// // Check Rust Version
// filePath := filepath.Join(path, "Cargo.lock")
// doesCargoLockExist, err := afero.Exists(f.fs, filePath)
// if err != nil {
// 	return nil, breverrors.WrapAndTrace(err)
// }

// filePath = filepath.Join(path, "Cargo.toml")
// doesCargoTomlExist, err := afero.Exists(f.fs, filePath)
// if err != nil {
// 	return nil, breverrors.WrapAndTrace(err)
// }

// if doesCargoLockExist || doesCargoTomlExist {
// 	// should be version number
// 	deps.Rust = "true"
// }

// // Check Node
// // look for package.json or package_lock.json
// filePath = filepath.Join(path, "package_lock.json")
// doesPkgLockExist, err := afero.Exists(f.fs, filePath)
// if err != nil {
// 	return nil, breverrors.WrapAndTrace(err)
// }

// filePath = filepath.Join(path, "package.json")
// doesPkgExist, err := afero.Exists(f.fs, filePath)
// if err != nil {
// 	return nil, breverrors.WrapAndTrace(err)
// }

// if doesPkgLockExist || doesPkgExist {
// 	// should be version number
// 	deps.Node = "true"
// }

// // Check Typescript
// // look for package.json or package_lock.json
// filePath = filepath.Join(path, "tsconfig.json")
// doesTSConfigExist, err := afero.Exists(f.fs, filePath)
// if err != nil {
// 	return nil, breverrors.WrapAndTrace(err)
// }

// if doesTSConfigExist {
// 	// should be version number
// 	deps.TS = "true"
// }

// // Check Java Version
// // idea: look for JAVA_HOME or JRE_HOME. Right now uses Java CLI
// cmdddd := exec.Command("java", "--version") // #nosec G204
// in, err = cmdddd.Output()
// if err != nil {
// 	// return nil, breverrors.WrapAndTrace(err)
// } else {
// 	d := charmap.CodePage850.NewDecoder()
// 	out, err := d.Bytes(in)
// 	if err != nil {
// 		// return nil, breverrors.WrapAndTrace(err)
// 	} else {
// 		if len(string(out)) > 0 {
// 			// fmt.Println(string(out))
// 			deps.Java = "true"
// 		}
// 	}
// }

func (f FileStore) WriteString(path, data string) error {
	file, err := f.GetOrCreateFile(path)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	_, err = file.WriteString(data)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

// CopyBin copies the running executeable to a target, creating directories as needed
func (f FileStore) CopyBin(targetBin string) error {
	err := f.fs.MkdirAll(filepath.Dir(targetBin), 0o755)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	exe, err := os.Executable()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	pathTmpBin := targetBin + ".tmp"
	fileTmpBin, err := f.fs.Create(pathTmpBin)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	self, err := f.fs.Open(exe)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	_, err = io.Copy(fileTmpBin, self)
	_ = self.Close()
	if err != nil {
		_ = fileTmpBin.Close()
		return breverrors.WrapAndTrace(err)
	}
	err = fileTmpBin.Close()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = f.fs.Chmod(pathTmpBin, 0o755)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = f.fs.Rename(pathTmpBin, targetBin)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (f FileStore) Chmod(path string, mode os.FileMode) error {
	err := f.fs.Chmod(path, mode)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func getUserHomeDir(f *FileStore) func() (string, error) {
	return func() (string, error) {
		if f.User != nil {
			return f.User.HomeDir, nil
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", breverrors.WrapAndTrace(err)
			}
			return home, nil
		}
	}
}

func (f FileStore) UserHomeDir() (string, error) {
	return f.userHomeDirGetter()
}

func (f FileStore) GetBrevHomePath() (string, error) {
	home, err := f.UserHomeDir()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	brevHome := files.GetBrevHome(home)

	return brevHome, nil
}

func (f FileStore) BuildBrevHome() error {
	home, err := f.UserHomeDir()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = files.BuildBrevHome(f.fs, home)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// chown to store user if set
	path, err := f.GetBrevHomePath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = f.ChownFilePathToUser(path)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (f FileStore) GetOSUser() string {
	if f.User != nil {
		return f.User.Uid
	} else {
		return fmt.Sprint(os.Getuid())
	}
}

func (f FileStore) GetServerSockFile() string {
	return "/var/run/brev.sock"
}

func (f FileStore) Remove(target string) error {
	err := f.fs.Remove(target)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (f FileStore) GetFileAsString(path string) (string, error) {
	res, err := files.ReadString(f.fs, path)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return res, nil
}

func (f FileStore) ChownFilePathToUser(filePath string) error {
	uid, err := strconv.ParseInt(f.User.Uid, 10, 32)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	gid, err := strconv.ParseInt(f.User.Gid, 10, 32)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = f.fs.Chown(filePath, int(uid), int(gid))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

// needs a functions that will:
// given a path:
// will create the path and parent dirs if not exists as store user
// will create file if not exists as store user, as well open it as read write for appending
// then return the file so it can be used as an io multiwriter with stdout
// call it GetOrCreateSetupLogFile(path string) (io.ReaderWriter error)
func (f FileStore) GetOrCreateSetupLogFile(path string) (afero.File, error) {
	err := f.fs.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	file, err := f.fs.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	err = f.ChownFilePathToUser(path)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return file, nil
}

// download a file from a url to a target path
func (f FileStore) DownloadBinary(url string, target string) error {
	// Get the data
	resp, err := http.Get(url) //nolint:gosec // the urls are hardcoded and trusted
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	defer resp.Body.Close() //nolint:errcheck // defer

	// check response code
	if resp.StatusCode >= 400 {
		return breverrors.WrapAndTrace(fmt.Errorf("bad status code: %d", resp.StatusCode))
	}

	// create target path if not exists
	err = f.fs.MkdirAll(filepath.Dir(target), 0o755)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Create the file
	out, err := f.fs.Create(target)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	defer out.Close() //nolint:errcheck // defer

	reader := trytoUnTarGZ(resp.Body)
	// overwrite the file with the downloaded data
	_, err = io.Copy(out, reader)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// make file executable
	err = f.fs.Chmod(target, 0o755)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

// get reader from tar.gz file
func trytoUnTarGZ(in io.Reader) io.Reader {
	gzReader, err := gzip.NewReader(in)
	if err != nil {
		return in
	}
	tarReader := tar.NewReader(gzReader)
	_, err = tarReader.Next()
	if err != nil {
		return gzReader
	}
	return tarReader
}

const mailFilePath = "/etc/brev/email"

func (f FileStore) WriteEmail(email string) error {
	exists, err := f.FileExists(mailFilePath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if exists {
		err = f.Remove(mailFilePath)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	err = f.WriteString(mailFilePath, email)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (f FileStore) ReadEmail() (string, error) {
	filestring, err := f.GetFileAsString(mailFilePath)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return filestring, nil
}

// append a string to a file
func (f FileStore) AppendString(path string, content string) error {
	file, err := f.fs.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	defer file.Close() //nolint:errcheck // defer

	_, err = file.WriteString(content)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
