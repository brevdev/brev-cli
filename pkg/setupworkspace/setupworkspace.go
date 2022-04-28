package setupworkspace

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
)

func SetupWorkspace(params *store.SetupParamsV0) error {
	user, err := GetUserFromUserStr("brev")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	wi := NewWorkspaceIniter(user, params)
	err = wi.Setup()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
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
			params.ProjectFolderName = strings.Split(params.WorkspaceProjectRepo[strings.LastIndex(params.WorkspaceProjectRepo, "/")+1:], ".")[0]
		} else {
			params.ProjectFolderName = strings.Split(params.WorkspaceHost.GetSlug(), "-")[0]
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

func (w WorkspaceIniter) CmdAsUser(cmd *exec.Cmd) error {
	err := CmdAsUser(cmd, w.User)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func SendLogToFile(cmd *exec.Cmd, filePath string) (func(), error) {
	outfile, err := os.Create(filePath) //nolint:gosec // occurs in safe area
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	stdOut := io.MultiWriter(outfile, os.Stdout)
	stdErr := io.MultiWriter(outfile, os.Stderr)
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr

	return func() {
		PrintErrFromFunc(outfile.Close)
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
	err := w.PrepareWorkspace()
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

	err = w.SetupCodeServer(w.Params.WorkspacePassword, fmt.Sprintf("127.0.0.1:%d", w.Params.WorkspacePort), string(w.Params.WorkspaceHost))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = w.SetupUserDotBrev(w.Params.WorkspaceBaseRepo)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fmt.Println("------ Setup Project Start ------")
	err = w.SetupProject(w.Params.WorkspaceProjectRepo, w.Params.WorkspaceProjectRepoBranch)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fmt.Println("------ Setup Project .brev Start ------")
	err = w.SetupProjectDotBrev(w.Params.SetupScript)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fmt.Println("------ Run User Setup ------")
	err = w.RunUserSetup()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fmt.Println("------ Run Project Setup ------")
	err = w.RunProjectSetup()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w WorkspaceIniter) PrepareWorkspace() error {
	cmd := CmdBuilder("chown", "-R", w.User.Username, w.WorkspaceDir) // TODO only do this if not done before
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = os.Remove(w.BuildWorkspacePath("lost+found"))
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
	defer PrintErrFromFunc(idRsa.Close)
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

	c := fmt.Sprintf(`su %s -c 'git config --global user.email "%s"'`, w.User.Username, email) // dont know why I have to do this
	cmd = CmdStringBuilder(c)
	// cmd = CmdBuilder("git", "config", "--global", "user.email", fmt.Sprintf(`"%s"`, email))
	// err = w.CmdAsUser(cmd)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	c = fmt.Sprintf(`su %s -c 'git config --global user.name "%s"'`, w.User.Username, username) // dont know why I have to do this
	cmd = CmdStringBuilder(c)
	// cmd = CmdBuilder("git", "config", "--global", "user.name", fmt.Sprintf(`"%s"`, username))
	// err = w.CmdAsUser(cmd)
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
	return nil
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
	cmd = CmdBuilder("echo", fmt.Sprintf(`"proxy-domain: %s"`, workspaceHost), codeServerConfig)
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	codeServerLogLevel := "trace"
	cmd = CmdBuilder("echo", fmt.Sprintf(`"log:: %s"`, codeServerLogLevel), codeServerConfig)
	err = cmd.Run()
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
	setupShPath := w.BuildUserDotBrevPath("setup.sh")
	if PathExists(setupShPath) {
		cmd := CmdBuilder("bash", "-c", setupShPath)
		cmd.Dir = w.BuildUserPath()
		err := w.CmdAsUser(cmd)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = os.MkdirAll(w.BuildUserDotBrevPath("logs"), os.ModePerm)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		done, err := SendLogToFile(cmd, w.BuildUserDotBrevPath("logs", "setup.log"))
		defer done()
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

func (w WorkspaceIniter) RunProjectSetup() error {
	setupShPath := w.BuildProjectDotBrevPath("setup.sh")
	if PathExists(setupShPath) {
		cmd := CmdBuilder("bash", "-c", setupShPath)
		cmd.Dir = w.BuildProjectPath()
		err := w.CmdAsUser(cmd)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = os.MkdirAll(w.BuildProjectDotBrevPath("logs"), os.ModePerm)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		done, err := SendLogToFile(cmd, w.BuildProjectDotBrevPath("logs", "setup.log"))
		defer done()
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
