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

func (f FileStore) GetDotGitConfigFile() (string, error) {
	dotGitConfigFile := filepath.Join(".git", "config")

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
		return "", breverrors.WrapAndTrace(errors.New("couldn't verify if "+ dotGitConfigFile +" is an active git directory"))
	}

	buf := new(strings.Builder)
	_, err = io.Copy(buf, file)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return buf.String(), nil

}

type Dependencies struct {
	Rust string
	Java string
	Node string
	TS   string
	Solana   string
}

func (f FileStore) GetDependenciesForImport() (*Dependencies, error) {

	deps := &Dependencies{
		Rust: "",
		Java: "",
		Node: "",
		TS: "",
		Solana: "",
	}

	// Check Rust Version
	filePath := filepath.Join("", "Cargo.lock")
	doesCargoLockExist, err := afero.Exists(f.fs, filePath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	filePath = filepath.Join("", "Cargo.toml")
	doesCargoTomlExist, err := afero.Exists(f.fs, filePath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	if doesCargoLockExist || doesCargoTomlExist {
		// should be version number
		deps.Rust = "true"
	}

	// Check Node
	// look for package.json or package_lock.json
	filePath = filepath.Join("", "package_lock.json")
	doesPkgLockExist, err := afero.Exists(f.fs, filePath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	filePath = filepath.Join("", "package.json")
	doesPkgExist, err := afero.Exists(f.fs, filePath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	
	if doesPkgLockExist || doesPkgExist {
		// should be version number
		deps.Node = "true"
	}

	// Check Typescript
	// look for package.json or package_lock.json
	filePath = filepath.Join("", "tsconfig.json")
	doesTSConfigExist, err := afero.Exists(f.fs, filePath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	if doesTSConfigExist {
		// should be version number
		deps.TS = "true"
	}

	// Check Java Version
	// idea: look for JAVA_HOME or JRE_HOME. Right now uses Java CLI
	cmdddd := exec.Command("java", "--version") // #nosec G204
	in, err := cmdddd.Output()
	if err != nil {
		// return nil, breverrors.WrapAndTrace(err)
	} else {
		d := charmap.CodePage850.NewDecoder()
		out, err := d.Bytes(in)
		if err != nil {
			// return nil, breverrors.WrapAndTrace(err)
		} else {
			if len(string(out)) > 0 {
				// fmt.Println(string(out))
				deps.Java = "true"
			}
		}
	}

	return deps, nil
}
