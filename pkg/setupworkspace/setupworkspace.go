package setupworkspace

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
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
	"github.com/brevdev/brev-cli/pkg/util"
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
	Repos        store.Repos
	Execs        store.Execs
}

func NewWorkspaceIniter(user *user.User, params *store.SetupParamsV0) *WorkspaceIniter {
	userRepo := makeUserRepo(*params)
	projectReop := makeProjectRepo(*params)

	params.Repos = mergeRepos(userRepo, projectReop, params.Repos)

	workspaceDir := "/home/brev/workspace"

	params.Repos = initRepos(params.Repos)

	if params.ProjectSetupScript == nil || *params.ProjectSetupScript == "" {
		defaultScript := "#!/bin/bash\n"
		b64DefaultScript := base64.StdEncoding.EncodeToString([]byte(defaultScript))
		params.ProjectSetupScript = &b64DefaultScript
	}

	standardSetup := makeExecFromSetupParams(*params)

	params.Execs = mergeExecs(standardSetup, params.Execs)

	return &WorkspaceIniter{
		WorkspaceDir: workspaceDir,
		UserRepoName: "user-dotbrev",
		User:         user,
		Params:       params,
		Repos:        params.Repos,
		Execs:        params.Execs,
	}
}

func makeUserRepo(params store.SetupParamsV0) store.Repos {
	return store.Repos{
		"user-config": {
			Repository:    params.WorkspaceBaseRepo,
			Directory:     "user-dotbrev",
			Branch:        "",
			BrevPath:      params.UserBrevPath,
			SetupExecPath: params.UserSetupExecPath,
			ExecWorkDir:   "",
		},
	}
}

func makeProjectRepo(params store.SetupParamsV0) store.Repos {
	if params.ProjectFolderName == "" {
		if params.WorkspaceProjectRepo != "" {
			params.ProjectFolderName = getDefaultProjectFolderNameFromRepo(params.WorkspaceProjectRepo)
		} else {
			params.ProjectFolderName = getDefaultProjectFolderNameFromHost(params.WorkspaceHost)
		}
	}
	return store.Repos{
		"project": {
			Repository:    params.WorkspaceProjectRepo,
			Directory:     params.ProjectFolderName,
			Branch:        params.WorkspaceProjectRepoBranch,
			BrevPath:      params.ProjectBrevPath,
			SetupExecPath: params.ProjectSetupExecPath,
			ExecWorkDir:   "",
		},
	}
}

func mergeRepos(repos ...store.Repos) store.Repos {
	newRepos := store.Repos{}
	for _, rs := range repos {
		for n, r := range rs {
			newRepos[n] = r
		}
	}
	return newRepos
}

func mergeExecs(repos ...store.Execs) store.Execs {
	newRepos := store.Execs{}
	for _, rs := range repos {
		for n, r := range rs {
			newRepos[n] = r
		}
	}
	return newRepos
}

func makeExecFromSetupParams(params store.SetupParamsV0) store.Execs {
	if params.ProjectSetupScript != nil {
		return store.Execs{
			"setup.sh": {
				Exec:        *params.ProjectSetupScript,
				ExecWorkDir: "",
				DependsOn:   []string{},
			},
		}
	}
	return store.Execs{}
}

func initRepo(repo store.RepoV0) store.RepoV0 {
	if repo.BrevPath == "" {
		repo.BrevPath = ".brev"
	}

	if repo.SetupExecPath == "" {
		repo.SetupExecPath = filepath.Join(repo.BrevPath, "setup.sh")
	}
	if repo.Directory == "" {
		repo.Directory = getDefaultProjectFolderNameFromRepo(repo.Repository)
	}
	return repo
}

func initRepos(repos store.Repos) store.Repos {
	newRepos := store.Repos{}
	for n, r := range repos {
		newRepos[n] = initRepo(r)
	}
	return newRepos
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
	return filepath.Join(append([]string{w.BuildProjectPath(w.Params.ProjectBrevPath)}, suffix...)...)
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

	var setupErr error

	err = w.SetupRepos()
	if err != nil {
		setupErr = multierror.Append(breverrors.WrapAndTrace(err))
	}

	err = w.RunExecs()
	if err != nil {
		setupErr = multierror.Append(breverrors.WrapAndTrace(err))
	}

	if setupErr != nil {
		return breverrors.WrapAndTrace(setupErr)
	}

	return nil
}

func (w WorkspaceIniter) SetupRepos() error {
	var setupErr error
	for n, r := range w.Repos {
		fmt.Printf("setting up %s\n", n)
		err := w.setupRepo(r)
		if err != nil {
			fmt.Printf("setup failed %s\n", n)
			setupErr = multierror.Append(breverrors.WrapAndTrace(err, fmt.Sprintf("setup failed %s", n)))
		} else {
			fmt.Printf("setup success %s\n", n)
		}
	}
	if setupErr != nil {
		return breverrors.WrapAndTrace(setupErr)
	}
	return nil
}

func (w WorkspaceIniter) RunExecs() error {
	dotBrev := filepath.Join(w.BuildWorkspacePath(), ".brev")
	err := w.setupDotBrev(dotBrev)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	var execErr error
	for n, e := range w.Execs {
		err := w.runExec(n, e)
		if err != nil {
			fmt.Printf("exec failed %s\n", n)
			execErr = multierror.Append(breverrors.WrapAndTrace(err, fmt.Sprintf("exec failed %s", n)))
		} else {
			fmt.Printf("exec success %s\n", n)
		}
	}
	if execErr != nil {
		return breverrors.WrapAndTrace(execErr)
	}
	return nil
}

func (w WorkspaceIniter) runExec(name store.ExecName, exec store.ExecV0) error {
	workDir := filepath.Join(w.BuildWorkspacePath(), exec.ExecWorkDir)
	dotBrev := filepath.Join(w.BuildWorkspacePath(), ".brev")
	logPath := filepath.Join(dotBrev, "logs")
	setupExecPath := filepath.Join(dotBrev, string(name))

	f, err := os.OpenFile(setupExecPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o700) //nolint:gosec // overwrite
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	out := util.DecodeBase64OrReturnSelf(exec.Exec)
	_, err = f.Write(out)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = f.Close() // must close before run setup script
	if err != nil {
		fmt.Println(err)
	}

	err = RunSetupScript(logPath, workDir, setupExecPath, w.User)
	if err != nil {
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
	for _, r := range w.Repos {
		res := strings.Contains(r.Repository, "github")
		if res {
			return true
		}
	}
	return false
}

func (w WorkspaceIniter) ShouldCheckGitlabAuth() bool {
	for _, r := range w.Repos {
		res := strings.Contains(r.Repository, "gitlab")
		if res {
			return true
		}
	}
	return false
}

func (w WorkspaceIniter) SetupCodeServer(password string, bindAddr string, workspaceHost string) error {
	cmd := CmdBuilder("code-server", "--install-extension", w.BuildHomePath(".config", "code-server", "brev-vscode.vsix"))
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	codeServerConfigPath := w.BuildHomePath(".config", "code-server", "config.yaml")
	cmd = CmdBuilder("sed", "-ri", fmt.Sprintf(`s/^(\s*)(password\s*:\s*.*\s*$)/\1password: %s/`, password), codeServerConfigPath)
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	cmd = CmdBuilder("sed", "-ri", fmt.Sprintf(`s/^(\s*)(bind-addr\s*:\s*.*\s*$)/\bind-addr: %s/`, bindAddr), codeServerConfigPath)
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	configFile, err := ioutil.ReadFile(codeServerConfigPath) //nolint:gosec // secure sandbox
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	proxyStr := fmt.Sprintf("proxy-domain: %s\n", workspaceHost)
	if !strings.Contains(string(configFile), proxyStr) {
		err = AppendToOrCreateFile(codeServerConfigPath, proxyStr)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	codeServerLogLevel := "trace"
	logLevel := fmt.Sprintf("log: %s\n", codeServerLogLevel)
	if !strings.Contains(string(configFile), logLevel) {
		err = AppendToOrCreateFile(codeServerConfigPath, logLevel)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
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

func (w WorkspaceIniter) setupRepo(repo store.RepoV0) error {
	repoPath := filepath.Join(w.BuildWorkspacePath(), repo.Directory)
	workDirPath := filepath.Join(repoPath, repo.ExecWorkDir)
	if repo.Repository == "" {
		fmt.Println("no repo")
		if !PathExists(repoPath) {
			fmt.Println("setting up default repo")
			err := os.MkdirAll(repoPath, 0o775) //nolint:gosec // occurs in safe area
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			err = ChownFilePathToUser(repoPath, w.User)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			cmd := CmdBuilder("git", "init")
			cmd.Dir = repoPath
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
		err := w.GitCloneIfDNE(repo.Repository, repo.Directory, repo.Branch)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	brevPath := filepath.Join(repoPath, repo.BrevPath)
	setupExecPath := filepath.Join(repoPath, repo.SetupExecPath)
	logsPath := filepath.Join(brevPath, "logs")

	err := w.setupDotBrev(brevPath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = RunSetupScript(logsPath, workDirPath, setupExecPath, w.User)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w WorkspaceIniter) setupDotBrev(dotBrevPath string) error {
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

	portsYamlPath := filepath.Join(dotBrevPath, "ports.yaml")
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

	setupScriptPath := filepath.Join(dotBrevPath, "setup.sh")
	if !PathExists(setupScriptPath) {
		cmd := CmdBuilder("curl", `https://raw.githubusercontent.com/brevdev/default-project-dotbrev/main/.brev/setup.sh`, "-o", setupScriptPath)
		err := w.CmdAsUser(cmd)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = cmd.Run()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = os.Chmod(setupScriptPath, 0o700) //nolint:gosec // occurs in safe area
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	gitIgnorePath := filepath.Join(dotBrevPath, ".gitignore")
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

func (w WorkspaceIniter) GitCloneIfDNE(url string, dirPath string, branch string) error {
	if !PathExists(dirPath) {
		// TODO implement multiple retry
		if !strings.HasPrefix(url, "git@") && !strings.HasPrefix(url, "http") {
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
		fmt.Printf("path already exists, did not clone %s to %s\n", url, dirPath)
	}
	return nil
}

func RunSetupScript(logsPath string, workingDir string, setupExecPath string, user *user.User) error {
	namePrefix := util.RemoveFileExtenstion(filepath.Base(setupExecPath))
	setupLogPath := filepath.Join(logsPath, fmt.Sprintf("%s.log", namePrefix))
	archivePath := filepath.Join(logsPath, "archive")
	archiveLogFile := filepath.Join(archivePath, fmt.Sprintf("%s-%s.log", namePrefix, time.Now().UTC().Format(time.RFC3339)))
	if workingDir == "" {
		workingDir = filepath.Dir(setupExecPath)
	}
	if PathExists(setupExecPath) {
		err := os.Chmod(setupExecPath, 0o700) //nolint:gosec // occurs in safe area
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		cmd := CmdStringBuilder(fmt.Sprintf("echo user: $(whoami) && echo pwd: $(pwd) && %s", setupExecPath))
		cmd.Dir = workingDir
		err = CmdAsUser(cmd, user)
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
		fmt.Printf("no setup script found at %s\n", setupExecPath)
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
