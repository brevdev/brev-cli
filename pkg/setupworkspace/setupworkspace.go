package setupworkspace

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strings"

	"github.com/alessio/shellescape"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
)

func ExecSetupScript(path string) error {
	//  executed as current user which is root on brev image, from current working dir which is on brev images "/home/brev/workspace/"
	cmd := exec.Command("bash", path)

	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func CreateSetupScript(params *store.SetupParamsV0) (string, error) {
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

	// TODO bad that it depends on host being special
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
        (mkdir /home/brev/workspace/$1/.brev || true)
        chown -R brev /home/brev/workspace/$1/.brev
        chmod 755 /home/brev/workspace/$1/.brev
        rm -rf /home/brev/workspace/$1/.brev/logs
        mv /home/brev/workspace/logs /home/brev/workspace/$1/.brev
        echo "" >> /home/brev/workspace/$1/.brev/logs/setup.log
        echo "##############################" >> /home/brev/workspace/$1/.brev/logs/setup.log
        echo "##### RUNNING SETUP FILE #####" >> /home/brev/workspace/$1/.brev/logs/setup.log
        echo "##############################" >> /home/brev/workspace/$1/.brev/logs/setup.log
        echo "" >> /home/brev/workspace/$1/.brev/logs/setup.log
        chmod +x  /home/brev/workspace/$1/.brev/setup.sh || true
        su brev -c /home/brev/workspace/$1/.brev/setup.sh >> /home/brev/workspace/$1/.brev/logs/setup.log 2>&1
        exitCodeSetup=$?
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
}

whoami
sudo su root
whoami
eval "$(ssh-agent -s)"
chown -R brev /home/brev/workspace
rm -rf lost+found
mkdir -p /home/brev/.ssh
echo "%s" > /home/brev/.ssh/id_rsa
chmod 400 /home/brev/.ssh/id_rsa
echo "%s" > /home/brev/.ssh/id_rsa.pub
chmod 400 /home/brev/.ssh/id_rsa.pub
ssh-add /home/brev/.ssh/id_rsa
ssh-keyscan github.com >> /home/brev/.ssh/known_hosts
ssh-keyscan gitlab.com >> /home/brev/.ssh/known_hosts
cat /home/brev/.ssh/id_rsa.pub > /home/brev/.ssh/authorized_keys
chown -R brev /home/brev/.ssh
su brev -c 'git config --global user.email "%s"'
su brev -c 'git config --global user.name "%s"'
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
		echo "Cloning git@%[1]s as %[2]s -- try = $(( $c + 1 ))  ..." >> logs/setup.log
		su brev -c 'git clone git@%[1]s %[2]s >> logs/setup.log 2>&1'
		exitCodeClone=$?
		if [ $exitCodeClone == 0 ]; then
			echo "Successfully cloned!" >> logs/setup.log
			didClone%[3]s=1
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
		fi
	done
else
	didClone%[3]s=1
fi

if [ $didClone%[3]s -eq 1 ]; then
	cd './%[2]s'
	(mkdir ./.brev || true)
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
(su brev -c 'mkdir ./%[1]s' || true)
cd './%[1]s'
(su brev -c 'git init' || true)
didCloneProject=1
cd ..
`

func makeSetupProjectDotBrevScript(projectFolderName string, setupScript *string) (string, error) {
	before := fmt.Sprintf(timestampTemplate, "START", "makeSetupProjectDotBrevScript")
	after := fmt.Sprintf(timestampTemplate, "END", "makeSetupProjectDotBrevScript")
	setupScriptString := ""
	if setupScript == nil || *setupScript == "" {
		resp, err := http.Get("https://raw.githubusercontent.com/brevdev/default-project-dotbrev/main/.brev/setup.sh")
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		setupScriptString = base64.StdEncoding.EncodeToString(body)
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
		mkdir "./%[1]s/.brev"
	fi
	if [ ! -f "./%[1]s/.brev/ports.yaml" ]; then
		echo "Creating a ports.yaml file!"
		curl "https://raw.githubusercontent.com/brevdev/default-project-dotbrev/main/.brev/ports.yaml" -o "./%[1]s/.brev/ports.yaml"
		chown -R brev "./%[1]s/.brev/ports.yaml"
	fi
	if [ ! -f "./%[1]s/.brev/setup.sh" ]; then
		echo "Creating a setup.sh file!"
		echo %[2]s| base64 --decode > ./%[1]s/.brev/setup.sh
		chown -R brev "./%[1]s/.brev/setup.sh"
		chmod +x "./%[1]s/.brev/setup.sh"
	fi
	if [ ! -f "./%[1]s/.brev/.gitignore" ]; then
		echo "Creating a .gitignore file!"
		curl "https://raw.githubusercontent.com/brevdev/default-project-dotbrev/main/.brev/.gitignore" -o "./%[1]s/.brev/.gitignore"
		chown -R brev "./%[1]s/.brev/.gitignore"
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
(su brev -c "code-server --install-extension /home/brev/.config/code-server/brev-vscode.vsix" || true)
sed -ri 's/^(\s*)(password\s*:\s*.*\s*$)/\1password: %s/' /home/brev/.config/code-server/config.yaml
sed -ri 's/^(\s*)(bind-addr\s*:\s*.*\s*$)/\bind-addr: %s/' /home/brev/.config/code-server/config.yaml
echo "proxy-domain: %s" >> /home/brev/.config/code-server/config.yaml
echo "log: trace" >> /home/brev/.config/code-server/config.yaml
systemctl daemon-reload
sudo systemctl restart code-server
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
