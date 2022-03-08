package store

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/spf13/afero"
	"golang.org/x/text/encoding/charmap"
)

type FileStore struct {
	BasicStore
	fs afero.Fs
}

func (b *BasicStore) WithFileSystem(fs afero.Fs) *FileStore {
	return &FileStore{*b, fs}
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

func (f FileStore) GetCurrentWorkspaceID() (string, error) {
	return os.Getenv("BREV_WORKSPACE_ID"), nil
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
