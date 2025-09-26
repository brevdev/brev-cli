package util

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"

	"github.com/hashicorp/go-multierror"
)

// This package should only be used as a holding pattern to be later moved into more specific packages

func MapAppend(m map[string]interface{}, n ...map[string]interface{}) map[string]interface{} {
	if m == nil { // we may get nil maps from legacy users not having user.OnboardingStatus set
		m = make(map[string]interface{})
	}
	for _, item := range n {
		for key, value := range item {
			m[key] = value
		}
	}
	return m
}

// checks if noun or pulural version of noun (checks if s at end)
func IsSingularOrPlural(check, noun string) bool {
	// TODO complex logic
	return check == noun || fmt.Sprintf("%ss", noun) == check
}

func DecodeBase64OrReturnSelf(maybeBase64 string) []byte {
	res, err := base64.StdEncoding.DecodeString(maybeBase64)
	if err != nil {
		fmt.Println("could not decode base64 assuming regular string")
		return []byte(maybeBase64)
	}
	return res
}

func RemoveFileExtenstion(path string) string {
	return strings.TrimRight(path, filepath.Ext(path))
}

type RunEResult struct {
	errChan chan error
	num     int
}

func (r RunEResult) Await() error {
	var allErr error
	for i := 0; i < r.num; i++ {
		err := <-r.errChan
		if err != nil {
			allErr = multierror.Append(err)
		}
	}
	if allErr != nil {
		return breverrors.WrapAndTrace(allErr)
	}
	return nil
}

func RunEAsync(calls ...func() error) RunEResult {
	res := RunEResult{make(chan error), len(calls)}
	for _, c := range calls {
		go func(cl func() error) {
			err := cl()
			res.errChan <- err
		}(c)
	}
	return res
}

func IsGitURL(u string) bool {
	return strings.Contains(u, "https://") || strings.Contains(u, "git@")
}

func DoesPathExist(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func InstallVscodeExtension(extensionID string) error {
	_, err := TryRunVsCodeCommand([]string{"--install-extension", extensionID, "--force"})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func IsVSCodeExtensionInstalled(extensionID string) (bool, error) {
	out, err := TryRunVsCodeCommand([]string{"--list-extensions"})
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	return strings.Contains(string(out), extensionID), nil
}

func InstallCursorExtension(extensionID string) error {
	_, err := TryRunCursorCommand([]string{"--install-extension", extensionID, "--force"})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func IsCursorExtensionInstalled(extensionID string) (bool, error) {
	out, err := TryRunCursorCommand([]string{"--list-extensions"})
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}

	// Check for the original extension ID
	if strings.Contains(string(out), extensionID) {
		return true, nil
	}

	// Check for Cursor-specific extension ID mappings
	cursorEquivalent := mapVSCodeToCursorExtension(extensionID)
	if cursorEquivalent != "" && strings.Contains(string(out), cursorEquivalent) {
		return true, nil
	}

	return false, nil
}

func InstallWindsurfExtension(extensionID string) error {
	_, err := TryRunWindsurfCommand([]string{"--install-extension", extensionID, "--force"})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func IsWindsurfExtensionInstalled(extensionID string) (bool, error) {
	out, err := TryRunWindsurfCommand([]string{"--list-extensions"})
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	return strings.Contains(string(out), extensionID), nil
}

func TryRunVsCodeCommand(args []string, extraPaths ...string) ([]byte, error) {
	extraPaths = append(commonVSCodePaths, extraPaths...)
	out, err := runManyVsCodeCommand(extraPaths, args)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return out, nil
}

func TryRunCursorCommand(args []string, extraPaths ...string) ([]byte, error) {
	extraPaths = append(commonCursorPaths, extraPaths...)
	out, err := runManyCursorCommand(extraPaths, args)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return out, nil
}

func TryRunWindsurfCommand(args []string, extraPaths ...string) ([]byte, error) {
	extraPaths = append(commonWindsurfPaths, extraPaths...)
	out, err := runManyWindsurfCommand(extraPaths, args)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return out, nil
}

func runManyVsCodeCommand(vscodepaths []string, args []string) ([]byte, error) {
	errs := multierror.Append(nil)
	for _, vscodepath := range vscodepaths {
		out, err := runVsCodeCommand(vscodepath, args)
		if err != nil {
			errs = multierror.Append(errs, err)
		} else {
			return out, nil
		}
	}
	return nil, breverrors.WrapAndTrace(errs.ErrorOrNil())
}

func runManyCursorCommand(cursorpaths []string, args []string) ([]byte, error) {
	errs := multierror.Append(nil)
	for _, cursorpath := range cursorpaths {
		out, err := runCursorCommand(cursorpath, args)
		if err != nil {
			errs = multierror.Append(errs, err)
		} else {
			return out, nil
		}
	}
	return nil, breverrors.WrapAndTrace(errs.ErrorOrNil())
}

func runVsCodeCommand(vscodepath string, args []string) ([]byte, error) {
	cmd := exec.Command(vscodepath, args...) // #nosec G204
	res, err := cmd.CombinedOutput()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return res, nil
}

func runCursorCommand(cursorpath string, args []string) ([]byte, error) {
	cmd := exec.Command(cursorpath, args...) // #nosec G204
	res, err := cmd.CombinedOutput()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return res, nil
}

func runManyWindsurfCommand(windsurfpaths []string, args []string) ([]byte, error) {
	errs := multierror.Append(nil)
	for _, windsurfpath := range windsurfpaths {
		out, err := runWindsurfCommand(windsurfpath, args)
		if err != nil {
			errs = multierror.Append(errs, err)
		} else {
			return out, nil
		}
	}
	return nil, breverrors.WrapAndTrace(errs.ErrorOrNil())
}

func runWindsurfCommand(windsurfpath string, args []string) ([]byte, error) {
	cmd := exec.Command(windsurfpath, args...) // #nosec G204
	res, err := cmd.CombinedOutput()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return res, nil
}

var commonVSCodePaths = []string{
	"code",
	"/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code",
	"/mnt/c/Program Files/Microsoft VS Code/bin/code",
	"/usr/bin/code",
	"/usr/local/bin/code",
	"/snap/bin/code",
	"/usr/local/share/code/bin/code",
	"/usr/share/code/bin/code",
	"/usr/share/code-insiders/bin/code-insiders",
	"/usr/share/code-oss/bin/code-oss",
	"/usr/share/code/bin/code",
}

var commonCursorPaths = []string{
	"cursor",
	"/Applications/Cursor.app/Contents/Resources/app/bin/cursor",
	"/mnt/c/Program Files/Cursor/bin/cursor",
	"/usr/bin/cursor",
	"/usr/local/bin/cursor",
	"/snap/bin/cursor",
	"/usr/local/share/cursor/bin/cursor",
	"/usr/share/cursor/bin/cursor",
}

var commonWindsurfPaths = []string{
	"windsurf",
	"/Applications/Windsurf.app/Contents/Resources/app/bin/windsurf",
	"/mnt/c/Program Files/Windsurf/Windsurf.exe",
	"/mnt/c/Users/*/AppData/Local/Programs/Windsurf/bin/windsurf",
	"/usr/bin/windsurf",
	"/usr/local/bin/windsurf",
	"/snap/bin/windsurf",
	"/usr/local/share/windsurf/bin/windsurf",
	"/usr/share/windsurf/bin/windsurf",
}

// mapVSCodeToCursorExtension maps VSCode extension IDs to their Cursor equivalents
func mapVSCodeToCursorExtension(vscodeExtensionID string) string {
	cursorMappings := map[string]string{
		"ms-vscode-remote.remote-ssh": "anysphere.remote-ssh",
		// Add more mappings here as we discover them
	}
	return cursorMappings[vscodeExtensionID]
}
