package setupworkspace

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/uri"
	"github.com/hashicorp/go-multierror"
)

func SetupWorkspace(params *store.SetupParamsV0) error {
	user, err := GetUserFromUserStr("brev")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	wi := NewWorkspaceIniter(user, params)
	done, err := mirrorPipesToFile("/var/log/brev-workspace.log")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	defer done()
	err = wi.Setup()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func mirrorPipesToFile(logFile string) (func(), error) {
	// https://gist.github.com/jerblack/4b98ba48ed3fb1d9f7544d2b1a1be287
	// open file read/write | create if not exist | clear file at open if exists
	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o666) //nolint:gosec // occurs in safe area
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	// save existing stdout | MultiWriter writes to saved stdout and file
	stdOut := os.Stdout
	stdErr := os.Stderr
	mw := io.MultiWriter(stdOut, f)

	// get pipe reader and writer | writes to pipe writer come out pipe reader
	r, w, err := os.Pipe()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	// replace stdout,stderr with pipe writer | all writes to stdout, stderr will go through pipe instead (fmt.print, log)
	os.Stdout = w
	os.Stderr = w

	// writes with log.Print should also write to mw
	log.SetOutput(mw)

	// create channel to control exit | will block until all copies are finished
	exit := make(chan bool)

	go func() {
		// copy all reads from pipe to multiwriter, which writes to stdout and file
		_, _ = io.Copy(mw, r)
		// when r or w is closed copy will finish and true will be sent to channel
		exit <- true
	}()

	// function to be deferred in main until program exits
	return func() {
		// close writer then block on exit channel | this will let mw finish writing before the program exits
		_ = w.Close()
		<-exit
		// close file after all writes have finished
		_ = f.Close()
		os.Stdout = stdOut
		os.Stderr = stdErr
	}, nil
}

func GetUserFromUserStr(userStr string) (*user.User, error) {
	var osUser *user.User
	var err error
	osUser, err = user.Lookup(userStr)
	if err != nil {
		_, ok := err.(*user.UnknownUserError)
		if !ok {
			osUser, err = user.LookupId(userStr)
			if err != nil {
				return nil, breverrors.WrapAndTrace(err)
			}
		} else {
			return nil, breverrors.WrapAndTrace(err)
		}
	}
	return osUser, nil
}

type WorkspaceIniter struct {
	WorkspaceDir string
	UserRepoName string
	User         *user.User
	Params       *store.SetupParamsV0
}

func NewWorkspaceIniter(user *user.User, params *store.SetupParamsV0) *WorkspaceIniter {
	if params.BrevPath == "" {
		params.BrevPath = ".brev"
	}

	if params.ProjectFolderName == "" {
		if params.WorkspaceProjectRepo != "" {
			params.ProjectFolderName = getDefaultProjectFolderNameFromRepo(params.WorkspaceProjectRepo)
		} else {
			params.ProjectFolderName = getDefaultProjectFolderNameFromHost(params.WorkspaceHost)
		}
	}
	if params.SetupScript == nil || *params.SetupScript == "" {
		defaultScript := "#!/bin/bash\n"
		b64DefaultScript := base64.StdEncoding.EncodeToString([]byte(defaultScript))
		params.SetupScript = &b64DefaultScript
	}
	return &WorkspaceIniter{
		WorkspaceDir: "/home/brev/workspace",
		UserRepoName: "user-dotbrev",
		User:         user,
		Params:       params,
	}
}

func getDefaultProjectFolderNameFromRepo(repo string) string {
	return strings.Split(repo[strings.LastIndex(repo, "/")+1:], ".")[0]
}

func getDefaultProjectFolderNameFromHost(host uri.Host) string {
	slugSplitOnDash := strings.Split(host.GetSlug(), "-")
	nameSplitOnDash := slugSplitOnDash[:len(slugSplitOnDash)-2]
	return strings.Join(nameSplitOnDash, "-")
}

func (w WorkspaceIniter) CmdAsUser(cmd *exec.Cmd) error {
	err := CmdAsUser(cmd, w.User)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func SendLogToFiles(cmd *exec.Cmd, filePaths ...string) (func(), error) {
	outfiles := []io.Writer{}
	for _, f := range filePaths {
		outfile, err := os.Create(f) //nolint:gosec // occurs in safe area
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		outfiles = append(outfiles, outfile)
	}
	allStdout := append([]io.Writer{os.Stdout}, outfiles...)
	stdOut := io.MultiWriter(allStdout...)
	allStderr := append([]io.Writer{os.Stderr}, outfiles...)
	stdErr := io.MultiWriter(allStderr...)
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr

	return func() {
		for _, f := range outfiles {
			ff, ok := f.(*os.File)
			if !ok {
				panic("could not cast object to file")
			}
			PrintErrFromFunc(ff.Close)
		}
	}, nil
}

func (w WorkspaceIniter) ChownFileToUser(file *os.File) error {
	err := ChownFileToUser(file, w.User)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w WorkspaceIniter) BuildHomePath(suffix ...string) string {
	return filepath.Join(append([]string{w.User.HomeDir}, suffix...)...)
}

func (w WorkspaceIniter) BuildWorkspacePath(suffix ...string) string {
	return filepath.Join(append([]string{w.WorkspaceDir}, suffix...)...)
}

func (w WorkspaceIniter) BuildProjectPath(suffix ...string) string {
	return filepath.Join(append([]string{w.BuildWorkspacePath(w.Params.ProjectFolderName)}, suffix...)...)
}

func (w WorkspaceIniter) BuildUserPath(suffix ...string) string {
	return filepath.Join(append([]string{w.BuildWorkspacePath(w.UserRepoName)}, suffix...)...)
}

func (w WorkspaceIniter) BuildProjectDotBrevPath(suffix ...string) string {
	return filepath.Join(append([]string{w.BuildProjectPath(w.Params.BrevPath)}, suffix...)...)
}

func (w WorkspaceIniter) BuildUserDotBrevPath(suffix ...string) string {
	return filepath.Join(append([]string{w.BuildUserPath(".brev")}, suffix...)...)
}

func (w WorkspaceIniter) Setup() error {
	fmt.Println("------ Preparing the workspace ------")
	err := w.PrepareWorkspace()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = w.SetupCodeServer(w.Params.WorkspacePassword, fmt.Sprintf("127.0.0.1:%d", w.Params.WorkspacePort), string(w.Params.WorkspaceHost))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = w.SetupSSH(w.Params.WorkspaceKeyPair)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = w.SetupGit(w.Params.WorkspaceUsername, w.Params.WorkspaceEmail)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = w.RunApplicationScripts(w.Params.WorkspaceApplicationStartScripts)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fmt.Println("------ Setup User Config ------")
	err = w.SetupUserDotBrev(w.Params.WorkspaceBaseRepo)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fmt.Println("------ Setup Project Config ------")
	err = w.SetupProject(w.Params.WorkspaceProjectRepo, w.Params.WorkspaceProjectRepoBranch)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fmt.Println("------ Setup Project .brev ------")
	err = w.SetupProjectDotBrev(w.Params.SetupScript)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fmt.Println("------ Run User Setup ------")
	var setupErr error
	err = w.RunUserSetup()
	if err != nil {
		setupErr = multierror.Append(breverrors.WrapAndTrace(err, "user setup failed"))
	}

	fmt.Println("------ Run Project Setup ------")
	err = w.RunProjectSetup()
	if err != nil {
		setupErr = multierror.Append(breverrors.WrapAndTrace(err, "project setup failed"))
	}

	if setupErr != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w WorkspaceIniter) PrepareWorkspace() error {
	cmd := CmdBuilder("chown", "-R", w.User.Username, w.BuildHomePath()) // TODO only do this if not done before
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = os.Remove(w.BuildWorkspacePath("lost+found"))
	if err != nil {
		fmt.Printf("did not remove lost+found: %v\n", err)
	}
	err = os.Remove(w.BuildHomePath("lost+found"))
	if err != nil {
		fmt.Printf("did not remove lost+found: %v\n", err)
	}
	return nil
}

func PrintErrFromFunc(fn func() error) {
	err := fn()
	if err != nil {
		fmt.Println(err)
	}
}

func CmdBuilder(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	return cmd
}

func CmdStringBuilder(c string) *exec.Cmd {
	cmd := CmdBuilder("bash", "-c", c)
	return cmd
}

func (w WorkspaceIniter) SetupSSH(keys *store.KeyPair) error {
	cmd := CmdBuilder("mkdir", "-p", w.BuildHomePath(".ssh"))
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	idRsa, err := os.Create(w.BuildHomePath(".ssh", "id_rsa"))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	defer PrintErrFromFunc(idRsa.Close)
	_, err = idRsa.Write([]byte(keys.PrivateKeyData))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = idRsa.Chmod(0o400)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	idRsaPub, err := os.Create(w.BuildHomePath(".ssh", "id_rsa.pub"))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	defer PrintErrFromFunc(idRsaPub.Close)
	_, err = idRsaPub.Write([]byte(keys.PublicKeyData))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = idRsaPub.Chmod(0o400)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	c := fmt.Sprintf(`eval "$(ssh-agent -s)" && ssh-add %s`, w.BuildHomePath(".ssh", "id_rsa"))
	cmd = CmdStringBuilder(c)
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = os.WriteFile(w.BuildHomePath(".ssh", "authorized_keys"), []byte(keys.PublicKeyData), 0o644) //nolint:gosec // verified based on curr env
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	sshConfMod := fmt.Sprintf(`PubkeyAuthentication yes
AuthorizedKeysFile      %s
PasswordAuthentication no`, w.BuildHomePath(".ssh", "authorized_keys"))
	err = os.WriteFile(filepath.Join("/etc", "ssh", "sshd_config.d", fmt.Sprintf("%s.conf", w.User.Username)), []byte(sshConfMod), 0o644) //nolint:gosec // verified based on curr env
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func (w WorkspaceIniter) SetupGit(username string, email string) error {
	cmd := CmdStringBuilder(fmt.Sprintf("ssh-keyscan github.com >> %s", w.BuildHomePath(".ssh", "known_hosts")))
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	cmd = CmdStringBuilder(fmt.Sprintf("ssh-keyscan gitlab.com >> %s", w.BuildHomePath(".ssh", "known_hosts")))
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	cmd = CmdBuilder("git", "config", "--global", "user.email", fmt.Sprintf(`"%s"`, email))
	err = w.CmdAsUser(cmd)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	cmd = CmdBuilder("git", "config", "--global", "user.name", fmt.Sprintf(`"%s"`, username))
	err = w.CmdAsUser(cmd)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	cmd = CmdBuilder("chown", "-R", w.User.Username, w.BuildHomePath(".ssh"))
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = w.EnsureGitAuthOrError()
	if err != nil {
		fmt.Println("WARNING: ssh keys not added to git provider")
		fmt.Println(err)
	}

	return nil
}

func (w WorkspaceIniter) EnsureGitAuthOrError() error {
	if w.ShouldCheckGithubAuth() {
		fmt.Println("checking github auth")
		cmd := CmdBuilder("ssh", "-T", "git@github.com")
		cmd.Stderr = nil
		cmd.Stdout = nil
		err := w.CmdAsUser(cmd)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			if strings.Contains(string(out), "successfully authenticated") {
				return nil
			}
			fmt.Print(string(out))
			return fmt.Errorf("failed to authenticate to github (ensure your ssh keys are setup correctly)")
		}
	}
	if w.ShouldCheckGitlabAuth() {
		fmt.Println("checking gitlab auth")
		cmd := CmdBuilder("ssh", "-T", "git@gitlab.com")
		err := w.CmdAsUser(cmd)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			if strings.Contains(string(out), "successfully authenticated") {
				return nil
			}
			fmt.Print(string(out))
			return fmt.Errorf("failed to authenticate to gitlab (ensure your ssh keys are setup correctly)")
		}
	}
	return nil
}

func (w WorkspaceIniter) ShouldCheckGithubAuth() bool {
	return strings.Contains(w.Params.WorkspaceProjectRepo+w.Params.WorkspaceBaseRepo, "github")
}

func (w WorkspaceIniter) ShouldCheckGitlabAuth() bool {
	return strings.Contains(w.Params.WorkspaceProjectRepo+w.Params.WorkspaceBaseRepo, "gitlab")
}

func (w WorkspaceIniter) SetupCodeServer(password string, bindAddr string, workspaceHost string) error {
	cmd := CmdBuilder("code-server", "--install-extension", w.BuildHomePath(".config", "code-server", "brev-vscode.vsix"))
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	codeServerConfig := w.BuildHomePath(".config", "code-server", "config.yaml")
	cmd = CmdBuilder("sed", "-ri", fmt.Sprintf(`s/^(\s*)(password\s*:\s*.*\s*$)/\1password: %s/`, password), codeServerConfig)
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	cmd = CmdBuilder("sed", "-ri", fmt.Sprintf(`s/^(\s*)(bind-addr\s*:\s*.*\s*$)/\bind-addr: %s/`, bindAddr), codeServerConfig)
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = AppendToOrCreateFile(codeServerConfig, fmt.Sprintf("proxy-domain: %s\n", workspaceHost))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	codeServerLogLevel := "trace"
	err = AppendToOrCreateFile(codeServerConfig, fmt.Sprintf("log: %s\n", codeServerLogLevel))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	cmd = CmdBuilder("systemctl", "daemon-reload")
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	cmd = CmdBuilder("systemctl", "restart", "code-server")
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func (w WorkspaceIniter) RunApplicationScripts(scripts []string) error {
	for _, s := range scripts {
		cmd := CmdStringBuilder(s)
		_ = cmd.Run()
	}
	return nil
}

// source is a git url
func (w WorkspaceIniter) SetupUserDotBrev(source string) error {
	if source == "" {
		fmt.Println("user .brev not provided skipping")
		return nil
	}
	err := w.GitCloneIfDNE(source, w.BuildUserPath(), "")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = os.Chmod(w.BuildUserDotBrevPath("setup.sh"), 0o700) //nolint:gosec // occurs in safe area
	if err != nil {
		// if fails no need to crash
		fmt.Println(err)
	}
	return nil
}

// source is a git url
func (w WorkspaceIniter) SetupProject(source string, branch string) error {
	if source != "" {
		err := w.GitCloneIfDNE(source, w.BuildProjectPath(), branch)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = os.Chmod(w.BuildProjectDotBrevPath("setup.sh"), 0o700) //nolint:gosec // occurs in safe area
		if err != nil {
			// if fails no need to crash
			fmt.Println(err)
		}
	} else {
		fmt.Println("no project source -- creating default")
		projectPath := w.BuildProjectPath()
		if !PathExists(projectPath) {
			err := os.MkdirAll(projectPath, 0o775) //nolint:gosec // occurs in safe area
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			err = ChownFilePathToUser(projectPath, w.User)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			cmd := CmdBuilder("git", "init")
			cmd.Dir = projectPath
			err = w.CmdAsUser(cmd)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			err = cmd.Run()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
		}
	}
	return nil
}

func (w WorkspaceIniter) SetupProjectDotBrev(defaultSetupScriptMaybeB64 *string) error { //nolint:funlen // function is scoped appropriately
	dotBrevPath := w.BuildProjectDotBrevPath()
	if !PathExists(dotBrevPath) {
		err := os.MkdirAll(dotBrevPath, 0o775) //nolint:gosec // occurs in safe area
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		cmd := CmdBuilder("chown", "-R", w.User.Username, dotBrevPath)
		err = cmd.Run()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	portsYamlPath := w.BuildProjectDotBrevPath("ports.yaml")
	if !PathExists(portsYamlPath) {
		cmd := CmdBuilder("curl", `https://raw.githubusercontent.com/brevdev/default-project-dotbrev/main/.brev/ports.yaml`, "-o", portsYamlPath)
		err := w.CmdAsUser(cmd)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = cmd.Run()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	setupScriptPath := w.BuildProjectDotBrevPath("setup.sh")
	if !PathExists(setupScriptPath) && defaultSetupScriptMaybeB64 != nil {
		file, err := os.Create(setupScriptPath) //nolint:gosec // occurs in safe area
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		defer PrintErrFromFunc(file.Close)
		err = w.ChownFileToUser(file)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		setupSh := decodeBase64OrReturnSelf(*defaultSetupScriptMaybeB64)
		_, err = file.Write(setupSh)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = file.Chmod(0o700)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	gitIgnorePath := w.BuildProjectDotBrevPath(".gitignore")
	if !PathExists(gitIgnorePath) {
		cmd := CmdBuilder("curl", `https://raw.githubusercontent.com/brevdev/default-project-dotbrev/main/.brev/.gitignore`, "-o", gitIgnorePath)
		err := w.CmdAsUser(cmd)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = cmd.Run()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

func decodeBase64OrReturnSelf(maybeBase64 string) []byte {
	res, err := base64.StdEncoding.DecodeString(maybeBase64)
	if err != nil {
		fmt.Println("could not decode base64 assuming regular string")
		return []byte(maybeBase64)
	}
	return res
}

func (w WorkspaceIniter) GitCloneIfDNE(url string, dirPath string, branch string) error {
	if !PathExists(dirPath) {
		// TODO implement multiple retry
		if !strings.HasPrefix(url, "git@") {
			url = "git@" + url
		}
		cmd := CmdBuilder("git", "clone", "--recursive", url, dirPath)
		err := w.CmdAsUser(cmd)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = cmd.Run()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if branch != "" {
			cmd = CmdBuilder("git", "checkout", branch)
			cmd.Dir = dirPath
			err = w.CmdAsUser(cmd)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			err = cmd.Run()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
		}
	} else {
		fmt.Printf("did not clone %s to %s\n", url, dirPath)
	}
	return nil
}

func (w WorkspaceIniter) RunUserSetup() error {
	err := RunSetupScript(w.BuildUserDotBrevPath(), w.BuildUserPath(), w.User)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w WorkspaceIniter) RunProjectSetup() error {
	err := RunSetupScript(w.BuildProjectDotBrevPath(), w.BuildProjectPath(), w.User)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func RunSetupScript(dotBrevPath string, workingDir string, user *user.User) error {
	setupShPath := filepath.Join(dotBrevPath, "setup.sh")
	logsPath := filepath.Join(dotBrevPath, "logs")
	setupLogPath := filepath.Join(logsPath, "setup.log")
	archivePath := filepath.Join(logsPath, "archive")
	archiveLogFile := filepath.Join(archivePath, fmt.Sprintf("setup-%s.log", time.Now().UTC().Format(time.RFC3339)))
	if workingDir == "" {
		workingDir = filepath.Dir(setupShPath)
	}
	if PathExists(setupShPath) {
		cmd := CmdStringBuilder(fmt.Sprintf("echo user: $(whoami) && echo pwd: $(pwd) && %s", setupShPath))
		cmd.Dir = workingDir
		err := CmdAsUser(cmd, user)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = os.MkdirAll(logsPath, os.ModePerm)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = os.MkdirAll(archivePath, os.ModePerm)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		done, err := SendLogToFiles(cmd, setupLogPath, archiveLogFile)
		defer done()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = cmd.Run()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else {
		fmt.Printf("no setup script found at %s\n", setupShPath)
	}
	return nil
}

func PathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

type CommandGroup struct {
	Cmds []*exec.Cmd
	User *user.User
}

func NewCommandGroup() *CommandGroup {
	return &CommandGroup{}
}

func (c *CommandGroup) WithUser(user *user.User) *CommandGroup {
	c.User = user
	return c
}

func (c *CommandGroup) AddCmd(cmd *exec.Cmd) {
	c.Cmds = append(c.Cmds, cmd)
}

func (c *CommandGroup) Run() error {
	// TODO batch
	for _, cmd := range c.Cmds {
		if c.User != nil && (cmd.SysProcAttr == nil || cmd.SysProcAttr.Credential == nil) {
			err := CmdAsUser(cmd, c.User)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
		}
		err := cmd.Run()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	return nil
}

func CmdAsUser(cmd *exec.Cmd, user *user.User) error {
	uid, err := strconv.ParseInt(user.Uid, 10, 32)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	gid, err := strconv.ParseInt(user.Gid, 10, 32)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	cmd.Env = append(os.Environ(), "USER="+user.Username, "HOME="+user.HomeDir, "SHELL=/bin/bash") // TODO get shell from user
	return nil
}

func ChownFileToUser(file *os.File, user *user.User) error {
	uid, err := strconv.ParseInt(user.Uid, 10, 32)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	gid, err := strconv.ParseInt(user.Gid, 10, 32)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = file.Chown(int(uid), int(gid))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func ChownFilePathToUser(filePath string, user *user.User) error {
	uid, err := strconv.ParseInt(user.Uid, 10, 32)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	gid, err := strconv.ParseInt(user.Gid, 10, 32)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = os.Chown(filePath, int(uid), int(gid))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

// similar to redirect operator '>'
func AppendToOrCreateFile(filePath string, toAppend string) error {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644) //nolint:gosec // occurs in safe area
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	defer PrintErrFromFunc(f.Close)
	if _, err := f.WriteString(toAppend); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
