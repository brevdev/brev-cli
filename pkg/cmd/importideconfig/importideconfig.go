package importideconfig

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/hashicorp/go-multierror"
	"github.com/tidwall/gjson"
	"golang.org/x/text/encoding/charmap"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"

	"github.com/spf13/cobra"
)

// startLong    = "[internal] test"
var startExample = "[internal] test"

type ImportIDEConfigStore interface {
	GetCurrentUser() (*entity.User, error)
	UpdateUser(userID string, updatedUser *entity.UpdateUser) (*entity.User, error)
}

func NewCmdImportIDEConfig(t *terminal.Terminal, s ImportIDEConfigStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "import-ide-config",
		DisableFlagsInUseLine: true,
		Short:                 "Import your IDE config",
		Long:                  "Import your IDE config like vscode extensions",
		Example:               startExample,
		// Args:                 cmderrors.TransformToValidationError(cobra.MinimumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			return breverrors.WrapAndTrace(RunImportIDEConfig(t, s))
		},
	}

	return cmd
}

func RunImportIDEConfig(_ *terminal.Terminal, store ImportIDEConfigStore) error {
	fmt.Println("updating vscode extensions...")
	homedir, err := os.UserHomeDir()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	var extensions []entity.VscodeExtensionMetadata
	// intentionally reading from .vscode and not .vscode_extensions because if they want the extension, it should be installed locally
	paths, err := recursivelyFindFile([]string{"package.json"}, homedir+"/.vscode/extensions")
	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			var err1 error
			paths, err1 = recursivelyFindFile([]string{"package.json"}, homedir+"/.vscode-server/extensions")
			if err1 != nil {
				return multierror.Append(err, err1)
			}
		} else {
			return breverrors.WrapAndTrace(err)
		}
	}
	for _, v := range paths {
		pathWithoutHome := strings.Split(v, homedir+"/")[1]

		// of the format
		//       .vscode / extensions / extension_name / package.json
		//          1          2              3               4s
		if len(strings.Split(pathWithoutHome, "/")) == 4 {
			obj, err0 := createVSCodeMetadataObject(homedir, pathWithoutHome)
			if err0 != nil {
				return err0
			}
			extensions = append(extensions, *obj)
		}
	}

	user, err := store.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	_, err = store.UpdateUser(user.ID, &entity.UpdateUser{
		IdeConfig: entity.IDEConfig{
			VSCode: entity.VSCodeConfig{
				Extensions: extensions,
			},
		},
	})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

// Create a VSCodeMetadataObject from package.json file
func createVSCodeMetadataObject(homedir string, path string) (*entity.VscodeExtensionMetadata, error) {
	segments := strings.Split(path, "/")
	if !strings.Contains(segments[0], ".vscode") &&
		segments[1] != "extensions" && segments[3] != "package.json" {
		return nil, errors.New("extension could not be imported") // TODO: return this as a metric!!!
	}
	contents, err := catFile(homedir + "/" + path)
	if err != nil {
		return nil, err
	} else {
		repoBlock := gjson.Get(contents, "repository").String()
		repoURL := gjson.Get(repoBlock, "url").String()
		return &entity.VscodeExtensionMetadata{
			Name:        gjson.Get(contents, "name").String(),
			DisplayName: gjson.Get(contents, "displayName").String(),
			Version:     gjson.Get(contents, "version").String(),
			Publisher:   gjson.Get(contents, "publisher").String(),
			Description: gjson.Get(contents, "description").String(),
			Repository:  repoURL,
		}, nil
	}
}

func catFile(filePath string) (string, error) {
	gocmd := exec.Command("cat", filePath) // #nosec G204
	in, err := gocmd.Output()
	if err != nil {
		return "", breverrors.WrapAndTrace(err, "error reading file "+filePath)
	} else {
		d := charmap.CodePage850.NewDecoder()
		out, err := d.Bytes(in)
		if err != nil {
			return "", breverrors.WrapAndTrace(err, "error reading file "+filePath)
		}
		return string(out), nil
	}
}

func appendPath(a string, b string) string {
	if a == "." {
		return b
	}
	return a + "/" + b
}

// Returns list of paths to file
func recursivelyFindFile(filenames []string, path string) ([]string, error) {
	var paths []string

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	for _, f := range files {
		dir, err := os.Stat(appendPath(path, f.Name()))
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		} else {
			for _, filename := range filenames {
				if filename == f.Name() {
					paths = append(paths, appendPath(path, f.Name()))
				}
			}

			if dir.IsDir() {
				res, err := recursivelyFindFile(filenames, appendPath(path, f.Name()))
				if err != nil {
					return nil, breverrors.WrapAndTrace(err)
				}
				paths = append(paths, res...)
			}
		}
	}

	return paths, nil
}
