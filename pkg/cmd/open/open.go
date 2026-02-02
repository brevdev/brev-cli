package open

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/alessio/shellescape"
	"github.com/brevdev/brev-cli/pkg/analytics"
	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/hello"
	"github.com/brevdev/brev-cli/pkg/cmd/refresh"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	uutil "github.com/brevdev/brev-cli/pkg/util"
	"github.com/brevdev/brev-cli/pkg/writeconnectionevent"
	"github.com/briandowns/spinner"
	"github.com/hashicorp/go-multierror"
	"github.com/samber/mo"

	"github.com/spf13/cobra"
)

const (
	EditorVSCode   = "code"
	EditorCursor   = "cursor"
	EditorWindsurf = "windsurf"
	EditorTerminal = "terminal"
	EditorTmux     = "tmux"
)

var (
	openLong = `[command in beta] This will open an editor SSH-ed in to your instance.

Supported editors:
  code      - VS Code
  cursor    - Cursor
  windsurf  - Windsurf
  terminal  - Opens a new terminal window with SSH
  tmux      - Opens a new terminal window with SSH + tmux session

Terminal support by platform:
  macOS:   Terminal.app
  Linux:   gnome-terminal, konsole, or xterm
  WSL:     Windows Terminal (wt.exe)
  Windows: Windows Terminal or cmd

You must have the editor installed in your path.`
	openExample = `  # Open an instance by name or ID
  brev open instance_id_or_name
  brev open my-instance

  # Open multiple instances (each in separate editor window)
  brev open instance1 instance2 instance3

  # Open with a specific editor
  brev open my-instance code
  brev open my-instance cursor
  brev open my-instance windsurf
  brev open my-instance terminal
  brev open my-instance tmux

  # Open multiple instances with specific editor (flag is explicit)
  brev open instance1 instance2 --editor cursor
  brev open instance1 instance2 -e cursor

  # Or use positional arg (last arg is editor if it matches code/cursor/windsurf/tmux)
  brev open instance1 instance2 cursor

  # Set a default editor
  brev open --set-default cursor
  brev open --set-default windsurf

  # Create a GPU instance and open it immediately (reads instance name from stdin)
  brev create my-instance | brev open

  # Open a cluster (multiple instances from stdin)
  brev create my-cluster --count 3 | brev open

  # Create with specific GPU and open in Cursor
  brev search --gpu-name A100 | brev create ml-box | brev open cursor

  # Open in a new terminal window with SSH
  brev create my-instance | brev open terminal

  # Open in a new terminal window with tmux (supports multiple instances)
  brev create my-cluster --count 3 | brev open tmux`
)

type OpenStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	refresh.RefreshStore
	UpdateUser(string, *entity.UpdateUser) (*entity.User, error)
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	StartWorkspace(workspaceID string) (*entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetWindowsDir() (string, error)
	IsWorkspace() (bool, error)
}

func NewCmdOpen(t *terminal.Terminal, store OpenStore, noLoginStartStore OpenStore) *cobra.Command {
	var waitForSetupToFinish bool
	var directory string
	var host bool
	var setDefault string
	var editor string

	cmd := &cobra.Command{
		Annotations:           map[string]string{"access": ""},
		Use:                   "open [instance...] [editor]",
		DisableFlagsInUseLine: true,
		Short:                 "[beta] Open VSCode, Cursor, Windsurf, or tmux to your instance",
		Long:                  openLong,
		Example:               openExample,
		Args: cmderrors.TransformToValidationError(func(cmd *cobra.Command, args []string) error {
			setDefaultFlag, _ := cmd.Flags().GetString("set-default")
			if setDefaultFlag != "" {
				return cobra.NoArgs(cmd, args)
			}
			// Allow arbitrary args: instance names can come from stdin, last arg might be editor
			return nil
		}),
		ValidArgsFunction: completions.GetAllWorkspaceNameCompletionHandler(noLoginStartStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			if setDefault != "" {
				return handleSetDefault(t, setDefault)
			}

			// Validate editor flag if provided
			if editor != "" && !isEditorType(editor) {
				return breverrors.NewValidationError(fmt.Sprintf("invalid editor: %s. Must be 'code', 'cursor', 'windsurf', or 'tmux'", editor))
			}

			// Get instance names and editor type from args or stdin
			instanceNames, editorType, err := getInstanceNamesAndEditor(args, editor)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			setupDoneString := "------ Git repo cloned ------"
			if waitForSetupToFinish {
				setupDoneString = "------ Done running execs ------"
			}

			// Open each instance
			var lastErr error
			for _, instanceName := range instanceNames {
				if len(instanceNames) > 1 {
					fmt.Fprintf(os.Stderr, "Opening %s...\n", instanceName)
				}
				err = runOpenCommand(t, store, instanceName, setupDoneString, directory, host, editorType)
				if err != nil {
					if len(instanceNames) > 1 {
						fmt.Fprintf(os.Stderr, "Error opening %s: %v\n", instanceName, err)
						lastErr = err
						continue
					}
					return breverrors.WrapAndTrace(err)
				}
			}
			if lastErr != nil {
				return breverrors.NewValidationError("one or more instances failed to open")
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&host, "host", "", false, "ssh into the host machine instead of the container")
	cmd.Flags().BoolVarP(&waitForSetupToFinish, "wait", "w", false, "wait for setup to finish")
	cmd.Flags().StringVarP(&directory, "dir", "d", "", "directory to open")
	cmd.Flags().StringVar(&setDefault, "set-default", "", "set default editor (code, cursor, windsurf, terminal, or tmux)")
	cmd.Flags().StringVarP(&editor, "editor", "e", "", "editor to use (code, cursor, windsurf, terminal, or tmux)")

	return cmd
}

// isEditorType checks if a string is a valid editor type
func isEditorType(s string) bool {
	return s == EditorVSCode || s == EditorCursor || s == EditorWindsurf || s == EditorTerminal || s == EditorTmux
}

// getInstanceNamesAndEditor gets instance names from args/stdin and determines editor type
// editorFlag takes precedence, otherwise last arg may be an editor type (code, cursor, windsurf, tmux)
func getInstanceNamesAndEditor(args []string, editorFlag string) ([]string, string, error) {
	var names []string
	editorType := editorFlag

	// If no editor flag, check if last arg is an editor type
	if editorType == "" && len(args) > 0 && isEditorType(args[len(args)-1]) {
		editorType = args[len(args)-1]
		args = args[:len(args)-1]
	}

	// Add names from remaining args
	names = append(names, args...)

	// Check if stdin is piped
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		// Stdin is piped, read instance names (one per line)
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			name := strings.TrimSpace(scanner.Text())
			if name != "" {
				names = append(names, name)
			}
		}
	}

	if len(names) == 0 {
		return nil, "", breverrors.NewValidationError("instance name required: provide as argument or pipe from another command")
	}

	// If no editor specified, get default
	if editorType == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			editorType = EditorVSCode
		} else {
			settings, err := files.ReadPersonalSettings(files.AppFs, homeDir)
			if err != nil {
				editorType = EditorVSCode
			} else {
				editorType = settings.DefaultEditor
			}
		}
	}

	return names, editorType, nil
}

func handleSetDefault(t *terminal.Terminal, editorType string) error {
	if !isEditorType(editorType) {
		return fmt.Errorf("invalid editor type: %s. Must be 'code', 'cursor', 'windsurf', 'terminal', or 'tmux'", editorType)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	settings := &files.PersonalSettings{
		DefaultEditor: editorType,
	}

	err = files.WritePersonalSettings(files.AppFs, homeDir, settings)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	t.Vprint(t.Green("Default editor set to " + editorType + "\n"))
	return nil
}

// Fetch workspace info, then open code editor
func runOpenCommand(t *terminal.Terminal, tstore OpenStore, wsIDOrName string, setupDoneString string, directory string, host bool, editorType string) error { //nolint:funlen,gocyclo // define brev command
	// todo check if workspace is stopped and start if it if it is stopped
	fmt.Println("finding your instance...")
	res := refresh.RunRefreshAsync(tstore)
	workspace, err := util.GetUserWorkspaceByNameOrIDErr(tstore, wsIDOrName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if workspace.Status == "STOPPED" { // we start the env for the user
		err = startWorkspaceIfStopped(t, tstore, wsIDOrName, workspace)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	err = pollUntil(t, workspace.ID, "RUNNING", tstore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	workspace, err = util.GetUserWorkspaceByNameOrIDErr(tstore, wsIDOrName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if workspace.Status != "RUNNING" {
		return breverrors.WorkspaceNotRunning{Status: workspace.Status}
	}

	projPath, err := workspace.GetProjectFolderPath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if directory != "" {
		projPath = directory
	}

	localIdentifier := workspace.GetLocalIdentifier()
	if host {
		localIdentifier = workspace.GetHostIdentifier()
	}

	err = res.Await()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = hello.SetHasRunOpen(true)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	// we don't care about the error here but should log with sentry
	// legacy environments wont support this and cause errrors,
	// but we don't want to block the user from using vscode
	_ = writeconnectionevent.WriteWCEOnEnv(tstore, string(localIdentifier))
	err = openEditorWithSSH(t, string(localIdentifier), projPath, tstore, setupDoneString, editorType)
	if err != nil {
		if strings.Contains(err.Error(), `"code": executable file not found in $PATH`) {
			errMsg := "code\": executable file not found in $PATH\n\nadd 'code' to your $PATH to open VS Code from the terminal\n\texport PATH=\"/Applications/Visual Studio Code.app/Contents/Resources/app/bin:$PATH\""
			return handlePathError(tstore, workspace, errMsg)
		}
		if strings.Contains(err.Error(), `"cursor": executable file not found in $PATH`) {
			errMsg := "cursor\": executable file not found in $PATH\n\nadd 'cursor' to your $PATH to open Cursor from the terminal"
			return handlePathError(tstore, workspace, errMsg)
		}
		if strings.Contains(err.Error(), `"windsurf": executable file not found in $PATH`) {
			errMsg := "windsurf\": executable file not found in $PATH\n\nadd 'windsurf' to your $PATH to open Windsurf from the terminal"
			return handlePathError(tstore, workspace, errMsg)
		}
		if strings.Contains(err.Error(), `tmux: command not found`) {
			errMsg := "tmux not found on remote instance. This will be installed automatically."
			return handlePathError(tstore, workspace, errMsg)
		}
		return breverrors.WrapAndTrace(err)
	}
	// Call analytics for open
	_ = pushOpenAnalytics(tstore, workspace)
	return nil
}

func pushOpenAnalytics(tstore OpenStore, workspace *entity.Workspace) error {
	userID := ""
	user, err := tstore.GetCurrentUser()
	if err != nil {
		userID = workspace.CreatedByUserID
	} else {
		userID = user.ID
	}
	data := analytics.EventData{
		EventName: "Brev Open",
		UserID:    userID,
		Properties: map[string]string{
			"instanceId": workspace.ID,
		},
	}
	err = analytics.TrackEvent(data)
	return breverrors.WrapAndTrace(err)
}

func pollUntil(t *terminal.Terminal, wsid string, state string, openStore OpenStore) error {
	s := t.NewSpinner()
	isReady := false
	s.Suffix = " hang tight 🤙"
	s.Start()
	for !isReady {
		time.Sleep(5 * time.Second)
		ws, err := openStore.GetWorkspace(wsid)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		s.Suffix = "  instance is currently " + strings.ToLower(ws.Status)
		if ws.Status == state {
			s.Suffix = "Instance is ready!"
			s.Stop()
			isReady = true
		}
	}
	return nil
}

func startWorkspaceIfStopped(t *terminal.Terminal, tstore OpenStore, wsIDOrName string, workspace *entity.Workspace) error {
	startedWorkspace, err := tstore.StartWorkspace(workspace.ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	t.Vprintf("%s", t.Yellow("Instance %s is starting. \n\n", startedWorkspace.Name))
	workspace, err = util.GetUserWorkspaceByNameOrIDErr(tstore, wsIDOrName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func tryToInstallExtensions(
	t *terminal.Terminal,
	extIDs []string,
) {
	for _, extID := range extIDs {
		extInstalled, err0 := uutil.IsVSCodeExtensionInstalled(extID)
		if !extInstalled {
			err1 := uutil.InstallVscodeExtension(extID)
			isRemoteInstalled, err2 := uutil.IsVSCodeExtensionInstalled(extID)
			if !isRemoteInstalled {
				err := multierror.Append(err0, err1, err2)
				t.Print(t.Red("Couldn't install the necessary VSCode extension automatically.\nError: " + err.Error()))
				t.Print("\tPlease install VSCode and the following VSCode extension: " + t.Yellow(extID) + ".\n")
				_ = terminal.PromptGetInput(terminal.PromptContent{
					Label:      "Hit enter when finished:",
					ErrorMsg:   "error",
					AllowEmpty: true,
				})
			}
		}
	}
}

func tryToInstallCursorExtensions(
	t *terminal.Terminal,
	extIDs []string,
) {
	for _, extID := range extIDs {
		extInstalled, err0 := uutil.IsCursorExtensionInstalled(extID)
		if !extInstalled {
			err1 := uutil.InstallCursorExtension(extID)
			isRemoteInstalled, err2 := uutil.IsCursorExtensionInstalled(extID)
			if !isRemoteInstalled {
				err := multierror.Append(err0, err1, err2)
				t.Print(t.Red("Couldn't install the necessary Cursor extension automatically.\nError: " + err.Error()))
				t.Print("\tPlease install Cursor and the following Cursor extension: " + t.Yellow(extID) + ".\n")
				_ = terminal.PromptGetInput(terminal.PromptContent{
					Label:      "Hit enter when finished:",
					ErrorMsg:   "error",
					AllowEmpty: true,
				})
			}
		}
	}
}

func tryToInstallWindsurfExtensions(
	t *terminal.Terminal,
	extIDs []string,
) {
	for _, extID := range extIDs {
		extInstalled, err0 := uutil.IsWindsurfExtensionInstalled(extID)
		if !extInstalled {
			err1 := uutil.InstallWindsurfExtension(extID)
			isRemoteInstalled, err2 := uutil.IsWindsurfExtensionInstalled(extID)
			if !isRemoteInstalled {
				err := multierror.Append(err0, err1, err2)
				t.Print(t.Red("Couldn't install the necessary Windsurf extension automatically.\nError: " + err.Error()))
				t.Print("\tPlease install Windsurf and the following Windsurf extension: " + t.Yellow(extID) + ".\n")
				_ = terminal.PromptGetInput(terminal.PromptContent{
					Label:      "Hit enter when finished:",
					ErrorMsg:   "error",
					AllowEmpty: true,
				})
			}
		}
	}
}

// Opens code editor. Attempts to install code in path if not installed already
func getEditorName(editorType string) string {
	switch editorType {
	case EditorCursor:
		return "Cursor"
	case EditorWindsurf:
		return "Windsurf"
	case EditorTerminal:
		return "Terminal"
	case EditorTmux:
		return "tmux"
	default:
		return "VSCode"
	}
}

func handlePathError(tstore OpenStore, workspace *entity.Workspace, errMsg string) error {
	_, errStore := tstore.UpdateUser(
		workspace.CreatedByUserID,
		&entity.UpdateUser{
			OnboardingData: map[string]interface{}{
				"pathErrorTS": time.Now().UTC().Unix(),
			},
		})
	if errStore != nil {
		return errors.New(errMsg + "\n" + errStore.Error())
	}
	return errors.New(errMsg)
}

func openEditorByType(t *terminal.Terminal, editorType string, sshAlias string, path string, tstore OpenStore) error {
	extensions := []string{"ms-vscode-remote.remote-ssh", "ms-toolsai.jupyter-keymap", "ms-python.python"}
	switch editorType {
	case EditorCursor:
		tryToInstallCursorExtensions(t, extensions)
		return openCursor(sshAlias, path, tstore)
	case EditorWindsurf:
		tryToInstallWindsurfExtensions(t, extensions)
		return openWindsurf(sshAlias, path, tstore)
	case EditorTerminal:
		return openTerminal(sshAlias, path, tstore)
	case EditorTmux:
		return openTerminalWithTmux(sshAlias, path, tstore)
	default:
		tryToInstallExtensions(t, extensions)
		return openVsCode(sshAlias, path, tstore)
	}
}

func validateRemoteWorkspace(t *terminal.Terminal, tstore OpenStore, editorType string, originalErr error) error {
	err := mo.TupleToResult(tstore.IsWorkspace()).Match(
		func(value bool) (bool, error) {
			if value {
				return true, errors.New("you are in a remote brev instance; brev open is not supported. Please run brev open locally instead")
			}
			return false, breverrors.WrapAndTrace(originalErr)
		},
		func(err2 error) (bool, error) {
			return false, multierror.Append(originalErr, err2)
		},
	).Error()
	if err != nil {
		if strings.Contains(err.Error(), "you are in a remote brev instance;") {
			return breverrors.WrapAndTrace(err)
		}
		editorName := getEditorName(editorType)
		return breverrors.WrapAndTrace(fmt.Errorf(t.Red("couldn't open %s, try adding it to PATH\n"), editorName))
	}
	return nil
}

func openEditorWithSSH(
	t *terminal.Terminal,
	sshAlias string,
	path string,
	tstore OpenStore,
	_ string,
	editorType string,
) error {
	res := refresh.RunRefreshAsync(tstore)
	err := res.Await()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	s := t.NewSpinner()
	s.Start()
	s.Suffix = "  checking if your instance is ready..."
	err = waitForSSHToBeAvailable(t, s, sshAlias)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	editorName := getEditorName(editorType)
	s.Suffix = fmt.Sprintf(" Instance is ready. Opening %s 🤙", editorName)
	time.Sleep(250 * time.Millisecond)
	s.Stop()
	t.Vprintf("\n")

	err = openEditorByType(t, editorType, sshAlias, path, tstore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return validateRemoteWorkspace(t, tstore, editorType, err)
}

func waitForSSHToBeAvailable(t *terminal.Terminal, s *spinner.Spinner, sshAlias string) error {
	counter := 0
	for {
		cmd := exec.Command("ssh", "-o", "ConnectTimeout=3", sshAlias, "echo", " ")
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}

		outputStr := string(out)
		stdErr := strings.Split(outputStr, "\n")[1]

		if counter == 160 || !store.SatisfactorySSHErrMessage(stdErr) {
			return breverrors.WrapAndTrace(errors.New("\n" + stdErr))
		}

		if counter == 10 {
			s.Suffix = t.Green(" waiting for SSH connection to be available 🤙")
		}

		counter++
		time.Sleep(1 * time.Second)
	}
}

type vscodePathStore interface {
	GetWindowsDir() (string, error)
}

func getWindowsVsCodePaths(store vscodePathStore) []string {
	wd, _ := store.GetWindowsDir()
	paths := append([]string{}, fmt.Sprintf("%s/AppData/Local/Programs/Microsoft VS Code/Code.exe", wd), fmt.Sprintf("%s/AppData/Local/Programs/Microsoft VS Code/bin/code", wd))
	return paths
}

func openVsCode(sshAlias string, path string, store OpenStore) error {
	vscodeString := fmt.Sprintf("vscode-remote://ssh-remote+%s%s", sshAlias, path)
	vscodeString = shellescape.QuoteCommand([]string{vscodeString})

	windowsPaths := getWindowsVsCodePaths(store)
	_, err := uutil.TryRunVsCodeCommand([]string{"--folder-uri", vscodeString}, windowsPaths...)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func openCursor(sshAlias string, path string, store OpenStore) error {
	cursorString := fmt.Sprintf("vscode-remote://ssh-remote+%s%s", sshAlias, path)
	cursorString = shellescape.QuoteCommand([]string{cursorString})

	windowsPaths := getWindowsCursorPaths(store)
	_, err := uutil.TryRunCursorCommand([]string{"--folder-uri", cursorString}, windowsPaths...)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func getWindowsCursorPaths(store vscodePathStore) []string {
	wd, _ := store.GetWindowsDir()
	paths := append([]string{}, fmt.Sprintf("%s/AppData/Local/Programs/Cursor/Cursor.exe", wd), fmt.Sprintf("%s/AppData/Local/Programs/Cursor/bin/cursor", wd))
	return paths
}

func openWindsurf(sshAlias string, path string, store OpenStore) error {
	windsurfString := fmt.Sprintf("vscode-remote://ssh-remote+%s%s", sshAlias, path)
	windsurfString = shellescape.QuoteCommand([]string{windsurfString})

	windowsPaths := getWindowsWindsurfPaths(store)
	_, err := uutil.TryRunWindsurfCommand([]string{"--folder-uri", windsurfString}, windowsPaths...)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func getWindowsWindsurfPaths(store vscodePathStore) []string {
	wd, _ := store.GetWindowsDir()
	paths := append([]string{}, fmt.Sprintf("%s/AppData/Local/Programs/Windsurf/Windsurf.exe", wd), fmt.Sprintf("%s/AppData/Local/Programs/Windsurf/bin/windsurf", wd))
	return paths
}

// openInNewTerminalWindow opens a command in a new terminal window based on the platform
// macOS: Terminal.app via osascript
// Linux: gnome-terminal, konsole, or xterm (tries in order)
// Windows/WSL: Windows Terminal (wt.exe)
func openInNewTerminalWindow(command string) error {
	switch runtime.GOOS {
	case "darwin":
		// macOS: use osascript to open Terminal.app
		script := fmt.Sprintf(`tell application "Terminal"
	activate
	do script "%s"
end tell`, command)
		cmd := exec.Command("osascript", "-e", script) // #nosec G204
		return breverrors.WrapAndTrace(cmd.Run())

	case "linux":
		// Check if we're in WSL by looking for wt.exe
		if _, err := exec.LookPath("wt.exe"); err == nil {
			// WSL: use Windows Terminal
			cmd := exec.Command("wt.exe", "new-tab", "bash", "-c", command) // #nosec G204
			return breverrors.WrapAndTrace(cmd.Run())
		}
		// Try gnome-terminal first (Ubuntu/GNOME)
		if _, err := exec.LookPath("gnome-terminal"); err == nil {
			cmd := exec.Command("gnome-terminal", "--", "bash", "-c", command+"; exec bash") // #nosec G204
			return breverrors.WrapAndTrace(cmd.Run())
		}
		// Try konsole (KDE)
		if _, err := exec.LookPath("konsole"); err == nil {
			cmd := exec.Command("konsole", "-e", "bash", "-c", command+"; exec bash") // #nosec G204
			return breverrors.WrapAndTrace(cmd.Run())
		}
		// Try xterm as fallback
		if _, err := exec.LookPath("xterm"); err == nil {
			cmd := exec.Command("xterm", "-e", "bash", "-c", command+"; exec bash") // #nosec G204
			return breverrors.WrapAndTrace(cmd.Run())
		}
		return breverrors.NewValidationError("no supported terminal emulator found. Install gnome-terminal, konsole, or xterm")

	case "windows":
		// Windows: use Windows Terminal
		if _, err := exec.LookPath("wt.exe"); err == nil {
			cmd := exec.Command("wt.exe", "new-tab", "cmd", "/c", command) // #nosec G204
			return breverrors.WrapAndTrace(cmd.Run())
		}
		// Fallback to start cmd
		cmd := exec.Command("cmd", "/c", "start", "cmd", "/k", command) // #nosec G204
		return breverrors.WrapAndTrace(cmd.Run())

	default:
		return breverrors.NewValidationError(fmt.Sprintf("'terminal' editor is not supported on %s", runtime.GOOS))
	}
}

func openTerminal(sshAlias string, path string, store OpenStore) error {
	_ = store // unused parameter required by interface
	_ = path  // unused, just opens SSH

	sshCmd := fmt.Sprintf("ssh %s", sshAlias)
	err := openInNewTerminalWindow(sshCmd)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func openTerminalWithTmux(sshAlias string, path string, store OpenStore) error {
	_ = store // unused parameter required by interface

	err := ensureTmuxInstalled(sshAlias)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	sessionName := "brev"

	// Check if tmux session exists
	checkCmd := fmt.Sprintf("ssh %s 'tmux has-session -t %s 2>/dev/null'", sshAlias, sessionName)
	checkExec := exec.Command("bash", "-c", checkCmd) // #nosec G204
	checkErr := checkExec.Run()

	var tmuxCmd string
	if checkErr == nil {
		// Session exists, attach to it
		tmuxCmd = fmt.Sprintf("ssh -t %s 'tmux attach-session -t %s'", sshAlias, sessionName)
	} else {
		// Create new session
		tmuxCmd = fmt.Sprintf("ssh -t %s 'cd %s && tmux new-session -s %s'", sshAlias, path, sessionName)
	}

	err = openInNewTerminalWindow(tmuxCmd)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func ensureTmuxInstalled(sshAlias string) error {
	checkCmd := fmt.Sprintf("ssh %s 'which tmux >/dev/null 2>&1'", sshAlias)
	checkExec := exec.Command("bash", "-c", checkCmd) // #nosec G204
	err := checkExec.Run()
	if err == nil {
		return nil
	}

	installCmd := fmt.Sprintf("ssh %s 'sudo apt-get update && sudo apt-get install -y tmux'", sshAlias)
	installExec := exec.Command("bash", "-c", installCmd) // #nosec G204
	installExec.Stderr = os.Stderr
	installExec.Stdout = os.Stdout

	err = installExec.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
