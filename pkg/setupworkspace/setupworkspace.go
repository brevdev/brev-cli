package setupworkspace

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/version"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/parse/pkg/parse"

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

	workspaceDir := "/home/brev/workspace"
	wi := NewWorkspaceIniter(workspaceDir, user, params)
	logFilePath := "/var/log/brev-workspace.log"
	done, err := mirrorPipesToFile(logFilePath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	defer done()
	fmt.Printf("brev %s\n", version.Version)

	fmt.Println("------ Setup Begin ------")
	err = wi.Setup()
	fmt.Println("------ Setup End ------")
	if err != nil {
		fmt.Println("------ Failure ------")
		time.Sleep(time.Millisecond * 100) // wait for buffer to be written
		logFile, errF := ioutil.ReadFile(logFilePath)
		if errF != nil {
			return multierror.Append(err, errF)
		}
		breverrors.GetDefaultErrorReporter().AddBreadCrumb(breverrors.ErrReportBreadCrumb{Type: "log-file", Message: string(logFile)})
		return breverrors.WrapAndTrace(err)
	} else {
		fmt.Println("------ Success ------")
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
	WorkspaceDir       string
	User               *user.User
	Params             *store.SetupParamsV0
	ReposV0            entity.ReposV0
	ExecsV0            entity.ExecsV0
	ReposV1            entity.ReposV1
	ExecsV1            entity.ExecsV1
	VscodeExtensionIDs []string
}

func NewWorkspaceIniter(workspaceDir string, user *user.User, params *store.SetupParamsV0) *WorkspaceIniter {
	userRepo := MakeUserRepo(*params)
	projectReop := MakeProjectRepo(*params)

	params.ReposV0 = MergeRepos(userRepo, projectReop, params.ReposV0)

	params.ReposV0 = InitRepos(params.ReposV0)

	if (params.ExecsV0 == nil || len(params.ExecsV0) == 0) && (params.ProjectSetupScript == nil || *params.ProjectSetupScript == "") {
		defaultScript := "#!/bin/bash\n"
		b64DefaultScript := base64.StdEncoding.EncodeToString([]byte(defaultScript))
		params.ProjectSetupScript = &b64DefaultScript
	}

	standardSetup := MakeExecFromSetupParams(*params)

	params.ExecsV0 = MergeExecs(standardSetup, params.ExecsV0)

	vscodeExtensionIDs := []string{}
	ideConfig, ok := params.IDEConfigs["vscode"]
	if ok {
		vscodeExtensionIDs = ideConfig.ExtensionIDs
	}

	return &WorkspaceIniter{
		WorkspaceDir:       workspaceDir,
		User:               user,
		Params:             params,
		ReposV0:            params.ReposV0,
		ExecsV0:            params.ExecsV0,
		ReposV1:            params.ReposV1,
		ExecsV1:            params.ExecsV1,
		VscodeExtensionIDs: vscodeExtensionIDs,
	}
}

func MakeUserRepo(params store.SetupParamsV0) entity.ReposV0 {
	if params.WorkspaceBaseRepo != "" {
		return entity.ReposV0{
			"user-config": {
				Repository:    params.WorkspaceBaseRepo,
				Directory:     "user-dotbrev",
				Branch:        "",
				BrevPath:      params.UserBrevPath,
				SetupExecPath: params.UserSetupExecPath,
				ExecWorkDir:   "",
			},
		}
	} else {
		return entity.ReposV0{}
	}
}

func MakeProjectRepo(params store.SetupParamsV0) entity.ReposV0 {
	if params.WorkspaceProjectRepo == "" && len(params.ReposV0) > 0 {
		return entity.ReposV0{}
	}
	if params.ProjectFolderName == "" {
		if params.WorkspaceProjectRepo != "" {
			params.ProjectFolderName = entity.GetDefaultProjectFolderNameFromRepo(params.WorkspaceProjectRepo)
		} else {
			params.ProjectFolderName = getDefaultProjectFolderNameFromHost(params.WorkspaceHost)
		}
	}
	return entity.ReposV0{
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

func MergeRepos(repos ...entity.ReposV0) entity.ReposV0 {
	newRepos := entity.ReposV0{}
	for _, rs := range repos {
		for n, r := range rs {
			newRepos[n] = r
		}
	}
	return newRepos
}

func MergeExecs(repos ...entity.ExecsV0) entity.ExecsV0 {
	newRepos := entity.ExecsV0{}
	for _, rs := range repos {
		for n, r := range rs {
			newRepos[n] = r
		}
	}
	return newRepos
}

func MakeExecFromSetupParams(params store.SetupParamsV0) entity.ExecsV0 {
	if params.ProjectSetupScript != nil {
		return entity.ExecsV0{
			"setup.sh": {
				Exec:        *params.ProjectSetupScript,
				ExecWorkDir: "",
				DependsOn:   []string{},
			},
		}
	}
	return entity.ExecsV0{}
}

func initRepo(repo entity.RepoV0) entity.RepoV0 {
	if repo.BrevPath == "" {
		repo.BrevPath = ".brev"
	}

	if repo.SetupExecPath == "" {
		repo.SetupExecPath = filepath.Join(repo.BrevPath, "setup.sh")
	}
	if repo.Directory == "" {
		repo.Directory = entity.GetDefaultProjectFolderNameFromRepo(repo.Repository)
	}
	return repo
}

func InitRepos(repos entity.ReposV0) entity.ReposV0 {
	newRepos := entity.ReposV0{}
	for n, r := range repos {
		newRepos[n] = initRepo(r)
	}
	return newRepos
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

func (w WorkspaceIniter) BuildHomePath(suffix ...string) string {
	return filepath.Join(append([]string{w.User.HomeDir}, suffix...)...)
}

func (w WorkspaceIniter) BuildWorkspacePath(suffix ...string) string {
	return filepath.Join(append([]string{w.WorkspaceDir}, suffix...)...)
}

func (w WorkspaceIniter) Setup() error {
	cmd := CmdStringBuilder("echo user: $(whoami) && echo pwd: $(pwd)")
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = w.PrepareWorkspace()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	postPrepare := util.RunEAsync(
		func() error {
			err0 := w.SetupVsCodeExtensions()
			if err0 != nil {
				fmt.Println(err0)
			}
			return nil
		},
		func() error {
			err = w.SetupCodeServer(w.Params.WorkspacePassword, fmt.Sprintf("127.0.0.1:%d", w.Params.WorkspacePort), string(w.Params.WorkspaceHost))
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	)

	err = w.SetupSSH(w.Params.WorkspaceKeyPair)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = w.SetupGit()
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

	err = w.CreateVerbYamlFile()
	if err != nil {
		setupErr = multierror.Append(breverrors.WrapAndTrace(err))
	}

	if setupErr != nil {
		return breverrors.WrapAndTrace(setupErr)
	}

	err = postPrepare.Await()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func (w WorkspaceIniter) SetupRepos() error {
	var setupErr error
	for n, r := range w.ReposV1 {
		fmt.Printf("setting up repo v1 %s\n", n)
		err := w.setupRepoV1(r)
		if err != nil {
			fmt.Printf("setup failed %s\n", n)
			setupErr = multierror.Append(breverrors.Wrap(err, fmt.Sprintf("setup failed %s", n)))
		} else {
			fmt.Printf("setup success %s\n", n)
		}
	}
	for n, r := range w.ReposV0 {
		fmt.Printf("setting up repo v0 %s\n", n)
		err := w.setupRepoV0(r)
		if err != nil {
			fmt.Printf("setup failed %s\n", n)
			setupErr = multierror.Append(breverrors.Wrap(err, fmt.Sprintf("setup failed %s", n)))
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
	for n, e := range w.ExecsV0 {
		err := w.runExecV0(n, e)
		if err != nil {
			fmt.Printf("exec failed %s\n", n)
			execErr = multierror.Append(breverrors.Wrap(err, fmt.Sprintf("exec failed %s", n)))
		} else {
			fmt.Printf("exec success %s\n", n)
		}
	}
	for n, e := range w.ExecsV1 {
		_ = e
		err := w.runExecV1(n, e)
		if err != nil {
			fmt.Printf("exec failed %s\n", n)
			execErr = multierror.Append(breverrors.Wrap(err, fmt.Sprintf("exec failed %s", n)))
		} else {
			fmt.Printf("exec success %s\n", n)
		}
	}
	if execErr != nil {
		return breverrors.WrapAndTrace(execErr)
	}
	return nil
}

func (w WorkspaceIniter) runExecV1(name entity.ExecName, exec entity.ExecV1) error {
	if exec.IsDisabled {
		fmt.Printf("exec %s disabled, not running", name)
		return nil
	}
	execWorkDir := ""
	if exec.ExecWorkDir != nil {
		execWorkDir = *exec.ExecWorkDir
	}
	workDirPath := filepath.Join(w.BuildWorkspacePath(), execWorkDir)
	if path.IsAbs(execWorkDir) {
		workDirPath = execWorkDir
	}

	execPath, err := w.GetExecPath(name, exec)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	logPath, err := w.GetLogPath(name, exec)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	logArchPath, err := w.GetLogArchivePath(name, exec)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if exec.Type == entity.StringExecType {
		err = w.CreateTempStrExecFile(execPath, exec.ExecStr)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	err = RunSetupScript(logPath, workDirPath, execPath, w.User, logArchPath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w WorkspaceIniter) CreateVerbYamlFile() error {
	fmt.Printf("Starting to add Verb Yaml: %s\n", w.Params.VerbYaml)
	dotBrev := filepath.Join(w.BuildWorkspacePath(), ".brev")
	err := w.setupDotBrev(dotBrev)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	setupExecPath := filepath.Join(dotBrev, string("verb.yaml"))
	fmt.Printf("verb yaml exec path: %s\n", setupExecPath)

	f, err := os.OpenFile(setupExecPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o700) //nolint:gosec // overwrite
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = ChownFileToUser(f, w.User)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	fmt.Printf("Adding Verb Yaml: %s\n", w.Params.VerbYaml)
	out := util.DecodeBase64OrReturnSelf(w.Params.VerbYaml)
	_, err = f.Write(out)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = f.Close()

	if err != nil {
		fmt.Printf("Adding Verb Yaml failed: %s\n", err)
		return breverrors.WrapAndTrace(err)
	} else {
		fmt.Printf("Adding Verb Yaml succeeded!\n")
		return nil
	}
}

func (w WorkspaceIniter) CreateTempStrExecFile(execPath string, execStr string) error {
	f, err := os.OpenFile(execPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o700) //nolint:gosec // overwrite
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = ChownFileToUser(f, w.User)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	out := util.DecodeBase64OrReturnSelf(execStr)
	_, err = f.Write(out)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = f.Close() // must close before run setup script
	if err != nil {
		fmt.Println(err)
	}
	return nil
}

func (w WorkspaceIniter) GetRepoPath(repo entity.RepoV1) (string, error) {
	dir, err := repo.GetDir()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	repoPath := filepath.Join(w.BuildWorkspacePath(), dir)
	if path.IsAbs(dir) {
		repoPath = dir
	}
	return repoPath, nil
}

func (w WorkspaceIniter) GetExecPath(name entity.ExecName, exec entity.ExecV1) (string, error) {
	execPath := ""
	if exec.Type == entity.PathExecType { //nolint:gocritic // i like if
		execPath = filepath.Join(w.BuildWorkspacePath(), exec.ExecPath)
		if path.IsAbs(exec.ExecPath) {
			execPath = exec.ExecPath
		}
	} else if exec.Type == entity.StringExecType {
		dotBrev := filepath.Join(w.BuildWorkspacePath(), ".brev")
		execPath = filepath.Join(dotBrev, string(name))
	} else {
		return "", fmt.Errorf("warning: exec type not supported: '%s'", exec.Type)
	}
	return execPath, nil
}

func (w WorkspaceIniter) GetLogPath(name entity.ExecName, exec entity.ExecV1) (string, error) {
	logPath := ""
	if exec.LogPath == nil || *exec.LogPath == "" {
		execPath, err := w.GetExecPath(name, exec)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		dir, _ := path.Split(execPath)
		logPath = path.Join(dir, "logs")
	} else {
		logPath = filepath.Join(w.BuildWorkspacePath(), *exec.LogPath)
		if path.IsAbs(*exec.LogPath) {
			logPath = *exec.LogPath
		}
	}
	return logPath, nil
}

func (w WorkspaceIniter) GetLogArchivePath(name entity.ExecName, exec entity.ExecV1) (string, error) {
	logArchPath := ""
	if exec.LogArchivePath == nil || *exec.LogArchivePath == "" {
		logPath, err := w.GetLogPath(name, exec)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		logArchPath = path.Join(logPath, "archive")
	} else {
		logArchPath = filepath.Join(w.BuildWorkspacePath(), *exec.LogArchivePath)
		if path.IsAbs(*exec.LogPath) {
			logArchPath = *exec.LogPath
		}
	}
	return logArchPath, nil
}

func (w WorkspaceIniter) runExecV0(name entity.ExecName, exec entity.ExecV0) error {
	workDir := filepath.Join(w.BuildWorkspacePath(), exec.ExecWorkDir)
	dotBrev := filepath.Join(w.BuildWorkspacePath(), ".brev")
	logPath := filepath.Join(dotBrev, "logs")
	setupExecPath := filepath.Join(dotBrev, string(name))

	f, err := os.OpenFile(setupExecPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o700) //nolint:gosec // overwrite
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = ChownFileToUser(f, w.User)
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

	err = RunSetupScript(logPath, workDir, setupExecPath, w.User, "")
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

func BuildAndRunCmd(name string, args ...string) error {
	cmd := CmdBuilder(name, args...)
	cmd.Env = append(cmd.Env, os.Environ()...)
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func RunCMDWithOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(cmd.Env, os.Environ()...)
	out, err := cmd.Output()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return string(out), nil
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

func (w WorkspaceIniter) SetupGit() error {
	username := w.Params.WorkspaceUsername
	email := w.Params.WorkspaceEmail
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
	cmd = CmdStringBuilder(fmt.Sprintf("ssh-keyscan bitbucket.org >> %s", w.BuildHomePath(".ssh", "known_hosts")))
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
	for _, r := range w.ReposV0 {
		res := strings.Contains(r.Repository, "github")
		if res {
			return true
		}
	}
	return false
}

func (w WorkspaceIniter) ShouldCheckGitlabAuth() bool {
	for _, r := range w.ReposV0 {
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

	fmt.Println("starting code server")
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
	fmt.Println("code server started")

	return nil
}

func (w WorkspaceIniter) SetupVsCodeExtensions() error {
	extensionIDs := w.VscodeExtensionIDs
	fmt.Println("installing vscode extensions...")
	codePathGlob := filepath.Join(w.BuildHomePath(), ".vscode-server/bin/*/bin/code-server")
	matches, err := filepath.Glob(codePathGlob)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if len(matches) == 0 {
		// TODO remove vscode after
		err := w.InstallVscode()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		codePathGlob := filepath.Join(w.BuildHomePath(), ".vscode-server/bin/*/bin/code-server")
		matches, err = filepath.Glob(codePathGlob)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if len(matches) == 0 {
			return fmt.Errorf("no vscode server found")
		}
	}
	// Glob = "~/.vscode-server/bin/c3511e6c69bb39013c4a4b7b9566ec1ca73fc4d5/bin/code-server"
	codePath := matches[0]

	fmt.Printf("exec code %s\n", codePath)

	var extErr error
	for _, e := range extensionIDs {
		fmt.Println(e)
		cmd := CmdBuilder(codePath, "--install-extension", e)
		err := w.CmdAsUser(cmd)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = cmd.Run()
		if err != nil {
			extErr = multierror.Append(extErr, err)
		}
	}
	if extErr != nil {
		return breverrors.WrapAndTrace(extErr)
	}
	fmt.Println("done vscode extensions")
	return nil
}

func (w WorkspaceIniter) InstallVscode() error {
	// TODO refactor to go code
	script := `#!/bin/bash
set -e

# You can get the latest commit SHA by looking at the latest tagged commit here: https://github.com/microsoft/vscode/releases
commit_sha="c3511e6c69bb39013c4a4b7b9566ec1ca73fc4d5"
archive="vscode-server-linux-x64.tar.gz"
owner='microsoft'
repo='vscode'

# Auto-Get the latest commit sha via command line.
get_latest_release() {
    tag=$(curl --silent "https://api.github.com/repos/${1}/releases/latest" | # Get latest release from GitHub API
          grep '"tag_name":'                                              | # Get tag line
          sed -E 's/.*"([^"]+)".*/\1/'                                    ) # Pluck JSON value

    tag_data=$(curl --silent "https://api.github.com/repos/${1}/git/ref/tags/${tag}")

    sha=$(echo "${tag_data}"           | # Get latest release from GitHub API
          grep '"sha":'                | # Get tag line
          sed -E 's/.*"([^"]+)".*/\1/' ) # Pluck JSON value

    sha_type=$(echo "${tag_data}"           | # Get latest release from GitHub API
          grep '"type":'                    | # Get tag line
          sed -E 's/.*"([^"]+)".*/\1/'      ) # Pluck JSON value

    if [[ "${sha_type}" != "commit" ]]; then
        combo_sha=$(curl -s "https://api.github.com/repos/${1}/git/tags/${sha}" | # Get latest release from GitHub API
              grep '"sha":'                                                     | # Get tag line
              sed -E 's/.*"([^"]+)".*/\1/'                                      ) # Pluck JSON value

        # Remove the tag sha, leaving only the commit sha;
        # this won't work if there are ever more than 2 sha,
        # and use xargs to remove whitespace/newline.
        sha=$(echo "${combo_sha}" | sed -E "s/${sha}//" | xargs)
    fi

    printf "${sha}"
}

commit_sha=$(get_latest_release "${owner}/${repo}")

echo "will attempt to download VS Code Server version = '${commit_sha}'"

# Download VS Code Server tarball to tmp directory.
curl -L "https://update.code.visualstudio.com/commit:${commit_sha}/server-linux-x64/stable" -o "/tmp/${archive}"

# Make the parent directory where the server should live.
# NOTE: Ensure VS Code will have read/write access; namely the user running VScode or container user.
mkdir -vp ~/.vscode-server/bin/"${commit_sha}"

# Extract the tarball to the right location.
tar --no-same-owner -xzv --strip-components=1 -C ~/.vscode-server/bin/"${commit_sha}" -f "/tmp/${archive}"
`
	err := ioutil.WriteFile("/tmp/vscode-install.sh", []byte(script), fs.ModePerm) //nolint:gosec // safe env
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	cmd := CmdBuilder("/tmp/vscode-install.sh")
	err = w.CmdAsUser(cmd)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func allRepoFormats(repo string) []string {
	repos := []string{
		repo,
		parse.GetSSHURLFromOrigin(repo),
		parse.GetHTTPSURLFromOrigin(repo),
		parse.GetHTTPURLFromOrigin(repo),
	}
	// some of these may be empty strings
	nonEmptyRepos := []string{}
	for _, r := range repos {
		if r != "" {
			nonEmptyRepos = append(nonEmptyRepos, r)
		}
	}
	return nonEmptyRepos
}

func (w WorkspaceIniter) setupRepoV1(repo entity.RepoV1) error {
	repoPath, err := w.GetRepoPath(repo)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if repo.Type == entity.GitRepoType { //nolint:gocritic // i like if
		branch := ""
		if repo.GitRepo.Branch != nil {
			branch = *repo.GitRepo.Branch
		}
		fmt.Println("setuprepov1: ", repoPath, branch)
		repository := repo.GitRepo.Repository
		repos := allRepoFormats(repository)
		for _, repoURL := range repos {
			err = w.GitCloneIfDNE(repoURL, repoPath, branch)
		}
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

	} else if repo.Type == entity.EmptyRepoType {
		fmt.Println("empty repo")
		if !PathExists(repoPath) {
			fmt.Println("setting up empty repo")
			err = os.MkdirAll(repoPath, 0o775) //nolint:gosec // occurs in safe area
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
		return fmt.Errorf("repo type not supported %s", repo.Type)
	}

	brevPath := filepath.Join(repoPath, ".brev")
	err = w.setupDotBrev(brevPath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w WorkspaceIniter) setupRepoV0(repo entity.RepoV0) error {
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
		repos := allRepoFormats(repo.Repository)
		var err error
		for _, repoURL := range repos {
			err = w.GitCloneIfDNE(repoURL, repoPath, repo.Branch)
		}
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

	err = RunSetupScript(logsPath, workDirPath, setupExecPath, w.User, "")
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
		fmt.Println("pre", url)
		if !strings.HasPrefix(url, "git@") && !strings.HasPrefix(url, "http") {
			url = "git@" + url
		}
		fmt.Printf("cloning %s to dir '%s'\n", url, dirPath)
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

func RunSetupScript(logsPath string, workingDir string, setupExecPath string, user *user.User, archivePath string) error {
	namePrefix := util.RemoveFileExtenstion(filepath.Base(setupExecPath))
	setupLogPath := filepath.Join(logsPath, fmt.Sprintf("%s.log", namePrefix))
	if archivePath == "" {
		archivePath = filepath.Join(logsPath, "archive")
	}
	archiveLogFile := filepath.Join(archivePath, fmt.Sprintf("%s-%s.log", namePrefix, time.Now().UTC().Format(time.RFC3339)))
	if workingDir == "" {
		workingDir = filepath.Dir(setupExecPath)
	}
	if PathExists(setupExecPath) {
		err := os.Chmod(setupExecPath, 0o700) //nolint:gosec // occurs in safe area
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		cmd := CmdStringBuilder(fmt.Sprintf("echo user: $(whoami) && echo pwd: $(pwd) && export PATH=\"/opt/conda/bin:$PATH\" && %s", setupExecPath))
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

// TODO batch

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
