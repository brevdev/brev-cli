package setupworkspace

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/alessio/shellescape"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
)

func ExecSetupScript(path string) error {
	//  executed as current user which is root on brev image, from current working dir which is on brev images "/home/brev/workspace/"
	cmd := exec.Command("sh", path)
	sendToOut(cmd)

	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func sendToOut(c *exec.Cmd) {
	c.Stderr = os.Stderr
	c.Stdout = os.Stdout
}

func CreateSetupScript(params *store.SetupParamsV0) (string, error) {
	// TO DEPRECATE -- setup script should be burned with fire as soon as possible
	scriptStart := makeSetupScriptStart(
		params.WorkspaceKeyPair.PrivateKeyData,
		params.WorkspaceKeyPair.PublicKeyData,
		params.WorkspaceEmail,
		params.WorkspaceUsername,
	)

	userConfigScript := ""
	if params.WorkspaceBaseRepo != "" {
		userConfigScript = makeSetupUserConfigScript(params.WorkspaceBaseRepo)
	}

	// TODO bad that it depends on "host" being special a special string
	projectFolderName := ""
	if params.WorkspaceProjectRepo != "" {
		projectFolderName = strings.Split(params.WorkspaceProjectRepo[strings.LastIndex(params.WorkspaceProjectRepo, "/")+1:], ".")[0]
	} else {
		projectFolderName = strings.Split(params.WorkspaceHost.GetSlug(), "-")[0]
	}

	projectScript := makeSetupProjectScript(
		params.WorkspaceProjectRepo,
		projectFolderName,
	)

	defaultProjectDotBrevScript, err := makeSetupProjectDotBrevScript(
		projectFolderName,
		params.SetupScript,
	)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	codeServerScript := makeSetupCodeServerScript(
		params.WorkspacePassword,
		fmt.Sprintf("127.0.0.1:%d", params.WorkspacePort),
		string(params.WorkspaceHost),
	)

	applicationStartScript := makeApplicationStartScript(params.WorkspaceApplicationStartScripts)

	postStartScript := makePostStartScript(
		scriptStart + codeServerScript + userConfigScript + projectScript + defaultProjectDotBrevScript + applicationStartScript,
	)

	bashScript := makeBashScript("post-start", postStartScript)

	return bashScript, nil
}

func makeSetupScriptStart(
	sshPrivateKey string,
	sshPubKey string,
	email string,
	username string,
) string {
	before := fmt.Sprintf(timestampTemplate, "START", "makeSetupScriptStart")
	after := fmt.Sprintf(timestampTemplate, "END", "makeSetupScriptStart")
	return before + fmt.Sprintf(
		setupScriptStartTemplate,
		sshPrivateKey,
		sshPubKey,
		email,
		username,
	) + after
}

const timestampTemplate = `
echo %[1]s %[2]s $(eval date +"%%Y-%%m-%%d_%%H:%%M:%%S")
`

// note: these commands are executed as root, from "/home/brev/workspace/"
const setupScriptStartTemplate = `
function run_setup_script {
		echo r1
        (mkdir -p /home/brev/workspace/$1/.brev || true)
        chown -R brev /home/brev/workspace/$1/.brev
        chmod 755 /home/brev/workspace/$1/.brev
        rm -rf /home/brev/workspace/$1/.brev/logs
        mv /home/brev/workspace/logs /home/brev/workspace/$1/.brev
        mkdir -p /home/brev/workspace/$1/.brev/logs
		echo r2
        echo "" >> /home/brev/workspace/$1/.brev/logs/setup.log
        echo "##############################" >> /home/brev/workspace/$1/.brev/logs/setup.log
        echo "##### RUNNING SETUP FILE #####" >> /home/brev/workspace/$1/.brev/logs/setup.log
        echo "##############################" >> /home/brev/workspace/$1/.brev/logs/setup.log
        echo "" >> /home/brev/workspace/$1/.brev/logs/setup.log
        chmod +x  /home/brev/workspace/$1/.brev/setup.sh || true
		echo r2.5 $(eval date +"%%Y-%%m-%%d_%%H:%%M:%%S")
        su brev -c /home/brev/workspace/$1/.brev/setup.sh >> /home/brev/workspace/$1/.brev/logs/setup.log 2>&1
        exitCodeSetup=$?
		echo 3 $(eval date +"%%Y-%%m-%%d_%%H:%%M:%%S")
        if [ $exitCodeSetup == 0 ]; then
                echo "Successfully ran setup script" >> /home/brev/workspace/$1/.brev/logs/setup.log
                echo "Dont forget to commit changes to your dot brev configuration files!" >> /home/brev/workspace/$1/.brev/logs/setup.log
        else
                echo "Exit code: $exitCodeSetup" >> /home/brev/workspace/$1/.brev/logs/setup.log
                echo "" >> /home/brev/workspace/$1/.brev/logs/setup.log
                echo "######################################" >> /home/brev/workspace/$1/.brev/logs/setup.log
                echo "##### ERROR DETECTED IN DOT BREV #####" >> /home/brev/workspace/$1/.brev/logs/setup.log
                echo "######################################" >> /home/brev/workspace/$1/.brev/logs/setup.log
                echo "" >> /home/brev/workspace/$1/.brev/logs/setup.log
                echo "Could not complete running setup.sh configuration file." >> /home/brev/workspace/$1/.brev/logs/setup.log
                echo "(1) See above log and modify the setup.sh file." >> /home/brev/workspace/$1/.brev/logs/setup.log
                echo "(2) Re-run the setup.sh file. To do this, click the play button next to the filename in the Brev extension." >> /home/brev/workspace/$1/.brev/logs/setup.log
                echo "(3) Commit your changes to git. Click the go to folder button next to the appropriate dot brev directory (hover over the appropriate top level folder in the extension to see the button)." >> /home/brev/workspace/$1/.brev/logs/setup.log
        fi
		echo r4
}

echo 2 $(eval date +"%%Y-%%m-%%d_%%H:%%M:%%S")
eval "$(ssh-agent -s)"
chown -R brev /home/brev/workspace
rm -rf lost+found
mkdir -p /home/brev/.ssh
echo 3 $(eval date +"%%Y-%%m-%%d_%%H:%%M:%%S")
echo "%s" > /home/brev/.ssh/id_rsa
chmod 400 /home/brev/.ssh/id_rsa
echo "%s" > /home/brev/.ssh/id_rsa.pub
chmod 400 /home/brev/.ssh/id_rsa.pub
ssh-add /home/brev/.ssh/id_rsa
echo 4 $(eval date +"%%Y-%%m-%%d_%%H:%%M:%%S")
ssh-keyscan github.com >> /home/brev/.ssh/known_hosts
echo 4.4 $(eval date +"%%Y-%%m-%%d_%%H:%%M:%%S")
ssh-keyscan gitlab.com >> /home/brev/.ssh/known_hosts
echo 4.8 $(eval date +"%%Y-%%m-%%d_%%H:%%M:%%S")
cat /home/brev/.ssh/id_rsa.pub > /home/brev/.ssh/authorized_keys
echo 5 $(eval date +"%%Y-%%m-%%d_%%H:%%M:%%S")
chown -R brev /home/brev/.ssh
echo 5.5 $(eval date +"%%Y-%%m-%%d_%%H:%%M:%%S")
su brev -c 'git config --global user.email "%s"'
su brev -c 'git config --global user.name "%s"'
echo 6 $(eval date +"%%Y-%%m-%%d_%%H:%%M:%%S")
cat <<EOT >> /etc/ssh/sshd_config.d/brev.conf
PubkeyAuthentication yes
AuthorizedKeysFile      /home/brev/.ssh/authorized_keys
PasswordAuthentication no
EOT
`

func makeSetupUserConfigScript(baseRepoURL string) string {
	before := fmt.Sprintf(timestampTemplate, "START", "makeSetupUserConfigScript")
	after := fmt.Sprintf(timestampTemplate, "END", "makeSetupUserConfigScript")
	return before + fmt.Sprintf(
		setupScriptFromGitTemplate,
		baseRepoURL,
		"user-dotbrev",
		"UserConfig",
	) + after
}

const setupScriptFromGitTemplate = `
(mkdir logs || true)
chown -R brev logs
chmod 755 logs
(touch logs/setup.log || true)
chown -R brev logs/setup.log
chmod 644 logs/setup.log
if [ ! -d "./%[2]s" ]; then
	echo "##############################" >> logs/setup.log
	echo "##### CLONING REPOSITORY #####" >> logs/setup.log
	echo "##############################" >> logs/setup.log
	echo "" >> logs/setup.log
	didClone%[3]s=0
	c=0
	while [ $c -lt 5 ]
	do
		echo "cloning..."
		echo "Cloning git@%[1]s as %[2]s -- try = $(( $c + 1 ))  ..." >> logs/setup.log
		su brev -c 'git clone git@%[1]s %[2]s >> logs/setup.log 2>&1'
		exitCodeClone=$?
		if [ $exitCodeClone == 0 ]; then
			echo "Successfully cloned!" >> logs/setup.log
			didClone%[3]s=1
			echo "clone done successfully"
			break
		else
			echo "Exit code: $exitCodeClone" >> logs/setup.log
			if [ $c -eq 4 ]; then
				echo "Max tries reached, unable to clone repository. See last error message" >> logs/setup.log
				echo "" >> './logs/setup.log'
				echo "######################################" >> './logs/setup.log'
				echo "##### ERROR DETECTED IN DOT BREV #####" >> './logs/setup.log'
				echo "######################################" >> './logs/setup.log'
				echo "" >> './logs/setup.log'
				echo "Fatal: could not clone repository from %[1]s" >> './logs/setup.log'
				echo "Check if you have access to that repository. If you do and this issue persists, contact support." >> './logs/setup.log'
			fi
			c=$(( $c + 1 ))
			echo "clone failed"
		fi
	done
else
	didClone%[3]s=1
fi

if [ $didClone%[3]s -eq 1 ]; then
	cd './%[2]s'
	(mkdir -p ./.brev || true)
	chown -R brev ./.brev
	chmod 755 ./.brev
	rm -rf './.brev/logs'
	mv '../logs' './.brev'
	echo "" >> './.brev/logs/setup.log'
	echo "##############################" >> './.brev/logs/setup.log'
	echo "##### RUNNING SETUP FILE #####" >> './.brev/logs/setup.log'
	echo "##############################" >> './.brev/logs/setup.log'
	echo "" >> './.brev/logs/setup.log'
	chmod +x .brev/setup.sh || true
	su brev -c './.brev/setup.sh >> ./.brev/logs/setup.log 2>&1'
	exitCodeSetup=$?
	if [ $exitCodeSetup == 0 ]; then
		echo "Successfully ran setup script" >> './.brev/logs/setup.log'
		echo "Don't forget to commit changes to your dot brev configuration files!" >> './.brev/logs/setup.log'
	else
		echo "Exit code: $exitCodeSetup" >> './.brev/logs/setup.log'
		echo "" >> './.brev/logs/setup.log'
		echo "######################################" >> './.brev/logs/setup.log'
		echo "##### ERROR DETECTED IN DOT BREV #####" >> './.brev/logs/setup.log'
		echo "######################################" >> './.brev/logs/setup.log'
		echo "" >> './.brev/logs/setup.log'
		echo "Could not complete running setup.sh configuration file." >> './.brev/logs/setup.log'
		echo "(1) See above log and modify the setup.sh file." >> './.brev/logs/setup.log'
		echo "(2) Re-run the setup.sh file. To do this, click the play button next to the filename in the Brev extension." >> './.brev/logs/setup.log'
		echo "(3) Commit your changes to git. Click the go to folder button next to the appropriate dot brev directory (hover over the appropriate top level folder in the extension to see the button)." >> './.brev/logs/setup.log'
	fi
	cd ..
fi
`

func makeSetupProjectScript(
	projectRepoURL string,
	projectFolderName string,
) string {
	if projectRepoURL != "" {
		before := fmt.Sprintf(
			timestampTemplate,
			"START",
			"makeSetupProjectScript -- Git",
		)
		after := fmt.Sprintf(timestampTemplate, "END", "makeSetupProjectScript -- Git")
		return before + fmt.Sprintf(
			setupScriptFromGitTemplate,
			projectRepoURL,
			projectFolderName,
			"Project",
		) + after
	} else {
		before := fmt.Sprintf(timestampTemplate, "START", "makeSetupProjectScript -- Blank")
		after := fmt.Sprintf(timestampTemplate, "END", "makeSetupProjectScript -- Blank")
		return before + fmt.Sprintf(setupProjectScriptFromScratchTemplate, projectFolderName) + after
	}
}

const setupProjectScriptFromScratchTemplate = `
echo "setting up from scratch"
if [ ! -d "./%[1]s" ]; then
	echo making repo
	mkdir ./%[1]s
	echo chown repo
	chown brev ./%[1]s
	echo done making repo
fi
cd './%[1]s'
if [ ! -d "./.git" ]; then
	echo initing git
	git init
	echo chown git
	chown brev .git
	echo done git
fi
didCloneProject=1
cd ..
echo "done setting up from scratch"
`

func makeSetupProjectDotBrevScript(projectFolderName string, setupScript *string) (string, error) {
	before := fmt.Sprintf(timestampTemplate, "START", "makeSetupProjectDotBrevScript")
	after := fmt.Sprintf(timestampTemplate, "END", "makeSetupProjectDotBrevScript")
	setupScriptString := ""
	if setupScript == nil || *setupScript == "" {
		fmt.Println("getting default setup.sh")
		resp, err := http.Get("https://raw.githubusercontent.com/brevdev/default-project-dotbrev/main/.brev/setup.sh") //nolint:noctx // TODO refactor to store
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		err = resp.Body.Close()
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		setupScriptString = base64.StdEncoding.EncodeToString(body)
		fmt.Println("done getting default setup.sh")
	} else {
		setupScriptString = base64.StdEncoding.EncodeToString([]byte(*setupScript))
	}

	return before + fmt.Sprintf(
		setupProjectDotBrevScript,
		projectFolderName,
		setupScriptString,
	) + after, nil
}

const setupProjectDotBrevScript = `
if [ $didCloneProject -eq 1 ]; then
	if [ ! -d "./%[1]s/.brev" ]; then
		echo "There is no .brev folder! Creating folder at ./%[1]s/.brev"
		mkdir -p "./%[1]s/.brev"
	fi
	if [ ! -f "./%[1]s/.brev/ports.yaml" ]; then
		echo "Creating a ports.yaml file!"
		curl "https://raw.githubusercontent.com/brevdev/default-project-dotbrev/main/.brev/ports.yaml" -o "./%[1]s/.brev/ports.yaml"
		chown -R brev "./%[1]s/.brev/ports.yaml"
		echo "done making ports yaml"
	fi
	if [ ! -f "./%[1]s/.brev/setup.sh" ]; then
		echo "Creating a setup.sh file!"
		echo %[2]s| base64 --decode > ./%[1]s/.brev/setup.sh
		chown -R brev "./%[1]s/.brev/setup.sh"
		chmod +x "./%[1]s/.brev/setup.sh"
		echo "done making setup"
	fi
	if [ ! -f "./%[1]s/.brev/.gitignore" ]; then
		echo "Creating a .gitignore file!"
		curl "https://raw.githubusercontent.com/brevdev/default-project-dotbrev/main/.brev/.gitignore" -o "./%[1]s/.brev/.gitignore"
		chown -R brev "./%[1]s/.brev/.gitignore"
		echo "done creating .gitignore"
	fi
fi

run_setup_script %[1]s
`

func makeSetupCodeServerScript(
	password string,
	codeServerAddr string,
	host string,
) string {
	before := fmt.Sprintf(timestampTemplate, "START", "makeSetupCodeServerScript")
	after := fmt.Sprintf(timestampTemplate, "END", "makeSetupCodeServerScript")
	return before + fmt.Sprintf(
		setupCodeServerScript,
		password,
		codeServerAddr,
		host,
	) + after
}

const setupCodeServerScript = `
echo "configuring code-server"
(su brev -c "code-server --install-extension /home/brev/.config/code-server/brev-vscode.vsix" || true)
sed -ri 's/^(\s*)(password\s*:\s*.*\s*$)/\1password: %s/' /home/brev/.config/code-server/config.yaml
sed -ri 's/^(\s*)(bind-addr\s*:\s*.*\s*$)/\bind-addr: %s/' /home/brev/.config/code-server/config.yaml
echo "proxy-domain: %s" >> /home/brev/.config/code-server/config.yaml
echo "log: trace" >> /home/brev/.config/code-server/config.yaml
sudo systemctl start code-server
sudo systemctl restart code-server
echo "done configuring code-server"
`

func makeApplicationStartScript(workspaceApplicationStartScripts []string) string {
	before := fmt.Sprintf(timestampTemplate, "START", "makeApplicationStartScript")
	after := fmt.Sprintf(timestampTemplate, "END", "makeApplicationStartScript")
	allApplicationStartScripts := ""
	for _, script := range workspaceApplicationStartScripts {
		allApplicationStartScripts += script
	}
	return before + allApplicationStartScripts + after
}

func makePostStartScript(script string) string {
	before := fmt.Sprintf(timestampTemplate, "####### START", "#######")
	after := fmt.Sprintf(timestampTemplate, "####### END", "#######")
	return fmt.Sprintf("%s\n%s\n%s", before, script, after)
}

func makeBashScript(name string, script string) string {
	escapedScript := shellEscape(script)
	return fmt.Sprintf(runBashScript, escapedScript, name)
}

const runBashScript = `
echo %[1]s > ~/%[2]s.sh && bash ~/%[2]s.sh >> ~/%[2]s.log 2>&1
`

func shellEscape(str string) string {
	backslashEscape := strings.ReplaceAll(str, "\\", "\\\\")
	quoteEscapedScript := shellescape.Quote(backslashEscape)
	return quoteEscapedScript
}

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

func (w WorkspaceIniter) BuildDotBrevPath(suffix ...string) string {
	return filepath.Join(append([]string{w.BuildProjectPath(w.Params.BrevPath)}, suffix...)...)
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

	err = w.SetupProject(w.Params.WorkspaceProjectRepo, w.Params.WorkspaceProjectRepoBranch)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = w.SetupProjectDotBrev(w.Params.SetupScript)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = w.RunUserSetup()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

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
	cmd = CmdBuilder("bash", "-c", c)
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
	cmd := CmdBuilder("ssh-keyscan", "github.com", ">>", w.BuildHomePath(".ssh", "known_hosts"))
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	cmd = CmdBuilder("ssh-keyscan", "gitlab.com", ">>", w.BuildHomePath(".ssh", "known_hosts"))
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	c := fmt.Sprintf(`su brev -c 'git config --global user.email "%s"'`, email) // dont know why I have to do this
	cmd = CmdBuilder("bash", "-c", c)
	// cmd = CmdBuilder("git", "config", "--global", "user.email", fmt.Sprintf(`"%s"`, email))
	// err = w.CmdAsUser(cmd)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	c = fmt.Sprintf(`su brev -c 'git config --global user.name "%s"'`, username) // dont know why I have to do this
	cmd = CmdBuilder("bash", "-c", c)
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
	err := w.GitCloneIfDNE(source, w.BuildUserPath(""), "")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

// source is a git url
func (w WorkspaceIniter) SetupProject(source string, branch string) error {
	err := w.GitCloneIfDNE(source, w.BuildProjectPath(""), branch)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w WorkspaceIniter) SetupProjectDotBrev(defaultSetupScriptB64 *string) error { //nolint:funlen // function is scoped appropriately
	dotBrevPath := w.BuildDotBrevPath("")
	if !PathExists(dotBrevPath) {
		err := os.MkdirAll(dotBrevPath, 0o755) //nolint:gosec // occurs in safe area
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	portsYamlPath := w.BuildDotBrevPath("ports.yaml")
	if !PathExists(portsYamlPath) {
		cmd := CmdBuilder("curl", `"https://raw.githubusercontent.com/brevdev/default-project-dotbrev/main/.brev/ports.yaml"`, "-o", fmt.Sprintf(`"%s"`, portsYamlPath))
		err := w.CmdAsUser(cmd)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = cmd.Run()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	setupScriptPath := w.BuildDotBrevPath("setup.sh")
	if !PathExists(setupScriptPath) && defaultSetupScriptB64 != nil {
		file, err := os.Create(setupScriptPath) //nolint:gosec // occurs in safe area
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		defer PrintErrFromFunc(file.Close)
		err = w.ChownFileToUser(file)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = file.Chmod(0o777)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		setupSh, err := base64.StdEncoding.DecodeString(*defaultSetupScriptB64)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		_, err = file.Write(setupSh)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}
	gitIgnorePath := w.BuildDotBrevPath(".gitignore")
	if !PathExists(gitIgnorePath) {
		cmd := CmdBuilder("curl", `"https://raw.githubusercontent.com/brevdev/default-project-dotbrev/main/.brev/.gitignore"`, "-o", fmt.Sprintf(`"%s"`, gitIgnorePath)) //nolint:gosec // occurs in safe area
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
		cmd := CmdBuilder("git", "clone", url, dirPath)
		err := w.CmdAsUser(cmd)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = cmd.Run()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		if branch != "" {
			cmd = CmdBuilder("git", "-c", dirPath, "checkout", branch)
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
	setupShPath := w.BuildUserPath(".brev", "setup.sh")
	if PathExists(setupShPath) {
		cmd := CmdBuilder(setupShPath) //nolint:gosec // occurs in safe area
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

func (w WorkspaceIniter) RunProjectSetup() error {
	setupShPath := w.BuildDotBrevPath("setup.sh")
	if PathExists(setupShPath) {
		cmd := CmdBuilder(setupShPath) //nolint:gosec // occurs in safe area
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
