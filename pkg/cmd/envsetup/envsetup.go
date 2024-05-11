package envsetup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	_ "embed"

	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/cmd/updatemodel"
	"github.com/brevdev/brev-cli/pkg/cmd/version"
	"github.com/brevdev/brev-cli/pkg/collections"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/setupworkspace"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/util"
)

type envsetupStore interface {
	GetEnvSetupParams(wsid string) (*store.SetupParamsV0, error)
	WriteSetupScript(script string) error
	GetSetupScriptPath() string
	GetCurrentUser() (*entity.User, error)
	GetCurrentWorkspaceID() (string, error)
	GetOSUser() string
	GetOrCreateSetupLogFile(path string) (afero.File, error)
	GetBrevHomePath() (string, error)
	BuildBrevHome() error
	CopyBin(targetBin string) error
	WriteString(path, data string) error
	UserHomeDir() (string, error)
	Remove(target string) error
	FileExists(target string) (bool, error)
	DownloadBinary(url string, target string) error
	AppendString(path string, content string) error
	Chmod(path string, mode os.FileMode) error
	ChownFilePathToUser(path string) error
	OverWriteString(path string, content string) error
	GetFileAsString(path string) (string, error)
}

type nologinEnvStore interface {
	LoginWithToken(token string) error
}

const name = "envsetup"

func NewCmdEnvSetup(store envsetupStore, noLoginStore nologinEnvStore) *cobra.Command {
	var forceEnableSetup bool
	// add debugger flag to toggle features when running command through a debugger
	// this is useful for debugging setup scripts
	debugger := false
	configureSystemSSHConfig := true

	// if a token flag is supplied, log in with it
	var token string

	var datadogAPIKey string

	var disableAutostop bool

	var reportInterval string

	var autostopPort string
	cmd := &cobra.Command{
		Use:                   name,
		DisableFlagsInUseLine: true,
		Short:                 "TODO",
		Long:                  "TODO",
		Example:               "TODO",
		RunE: func(cmd *cobra.Command, args []string) error {
			var errors error
			for _, arg := range args {
				err := RunEnvSetup(
					store,
					name,
					forceEnableSetup,
					debugger,
					configureSystemSSHConfig,
					arg,
					token,
					noLoginStore,
					datadogAPIKey,
					disableAutostop,
					reportInterval,
					autostopPort,
				)
				if err != nil {
					errors = multierror.Append(err)
				}
			}
			if errors != nil {
				return breverrors.WrapAndTrace(errors)
			}
			return nil
		},
	}
	cmd.PersistentFlags().BoolVar(&forceEnableSetup, "force-enable", false, "force the setup script to run despite params")
	cmd.PersistentFlags().BoolVar(&debugger, "debugger", debugger, "toggle features that don't play well with debuggers")
	cmd.PersistentFlags().BoolVar(&configureSystemSSHConfig, "configure-system-ssh-config", configureSystemSSHConfig, "configure system ssh config")
	cmd.PersistentFlags().StringVar(&token, "token", "", "token to use for login")
	cmd.PersistentFlags().StringVar(&datadogAPIKey, "datadog-api-key", "", "datadog API key to use for logging")
	cmd.PersistentFlags().BoolVar(&disableAutostop, "disable-autostop", false, "disable autostop")
	cmd.PersistentFlags().StringVar(&reportInterval, "report-interval", "10m", "report interval")
	cmd.PersistentFlags().StringVar(&autostopPort, "autostop-port", "22", "autostop port")

	return cmd
}

func RunEnvSetup(
	store envsetupStore,
	name string,
	forceEnableSetup, debugger, configureSystemSSHConfig bool,
	workspaceid, token string,
	noLoginStore nologinEnvStore,
	datadogAPIKey string,
	disableAutostop bool,
	reportInterval string,
	portToCheckAutostopTrafficOn string,
) error {
	if token != "" {
		err := noLoginStore.LoginWithToken(token)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	breverrors.GetDefaultErrorReporter().AddTag("command", name)
	_, err := store.GetCurrentWorkspaceID() // do this to error reporting
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	fmt.Println("setting up instance")

	params, err := store.GetEnvSetupParams(workspaceid)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	res, err := json.MarshalIndent(params, "", "")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	fmt.Println(string(res))

	if !featureflag.IsDev() && !debugger {
		_, err = store.GetCurrentUser() // do this to set error user reporting
		if err != nil {
			fmt.Println(err)
			if !params.DisableSetup {
				breverrors.GetDefaultErrorReporter().ReportError(breverrors.WrapAndTrace(err, "setup continued"))
			}
		}
	}

	if !forceEnableSetup && params.DisableSetup {
		fmt.Printf("WARNING: setup script not running [params.DisableSetup=%v, forceEnableSetup=%v]", params.DisableSetup, forceEnableSetup)
		return nil
	}

	err = setupEnv(
		store,
		params,
		configureSystemSSHConfig,
		datadogAPIKey,
		disableAutostop,
		reportInterval,
		portToCheckAutostopTrafficOn,
	)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fmt.Println("done setting up instance")
	return nil
}

type envInitier struct {
	setupworkspace.WorkspaceIniter
	ConfigureSystemSSHConfig bool
	brevMonConfigurer        autostartconf.DaemonConfigurer
	datadogAPIKey            string
	store                    envsetupStore
}

func appendLogToFile(content string, file string) error {
	cmd := setupworkspace.CmdStringBuilder(fmt.Sprintf(`echo "%s" >> %s`, content, file))
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

//go:embed motd.sh
var motd string

func (e *envInitier) SetupMOTD() error {
	err := e.store.OverWriteString("/etc/ssh/my_banner", motd)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fstring, err := e.store.GetFileAsString("/etc/ssh/sshd_config")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if !strings.Contains(fstring, "Banner /etc/ssh/my_banner") {
		err = e.store.AppendString("/etc/ssh/sshd_config", "Banner /etc/ssh/my_banner")
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	err = setupworkspace.BuildAndRunCmd("systemctl", "reload", "ssh.service")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

//go:embed speedtest.py
var speedtest string

func (e *envInitier) SetupSpeedTest() error {
	err := e.store.WriteString("/usr/local/bin/speedtest", speedtest)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = e.store.Chmod("/usr/local/bin/speedtest", 0o755)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (e *envInitier) SetupUpdateModel() error {
	dc := updatemodel.DaemonConfigurer{
		Store: e.store,
	}
	err := dc.Configure()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (e envInitier) Setup() error { //nolint:funlen,gocyclo // TODO
	var setupErr error

	err := appendLogToFile("setup started", "/var/log/brev-setup-steps.log")
	if err != nil {
		setupErr = multierror.Append(setupErr, breverrors.WrapAndTrace(err))
	}

	err = setupworkspace.BuildAndRunCmd("systemctl", "stop", "unattended-upgrades")
	if err != nil {
		setupErr = multierror.Append(setupErr, breverrors.WrapAndTrace(err))
	}
	out, err := setupworkspace.RunCMDWithOutput("apt-get", "-y", "remove", "unattended-upgrades")
	if err != nil {
		setupErr = multierror.Append(setupErr,
			breverrors.WrapAndTrace(err, "apt-get -y remove unattended-upgrades", out))
	}

	cmd := setupworkspace.CmdStringBuilder("echo user: $(whoami) && echo pwd: $(pwd)")
	err = cmd.Run()
	if err != nil {
		setupErr = multierror.Append(setupErr, breverrors.WrapAndTrace(err))
	}

	postPrepare := util.RunEAsync(
		e.SetupVsCodeExtensions,
		e.SetupSpeedTest,
		e.SetupMOTD,
		e.SetupUpdateModel,
	)

	err = util.RunEAsync(
		e.SetupSSH,
		e.SetupGit,
	).Await()
	if err != nil {
		setupErr = multierror.Append(setupErr, breverrors.WrapAndTrace(err))
	}

	err = appendLogToFile("starting repo setup", "/var/log/brev-steps.log")
	if err != nil {
		setupErr = multierror.Append(setupErr, breverrors.WrapAndTrace(err))
	}
	err = e.SetupRepos()
	if err != nil {
		setupErr = multierror.Append(breverrors.WrapAndTrace(err))
	}
	fmt.Println("------ Git repo cloned ------")
	err = appendLogToFile("repo setup done", "/var/log/brev-steps.log")
	if err != nil {
		setupErr = multierror.Append(setupErr, breverrors.WrapAndTrace(err))
	}

	err = e.SetupEnvVars() // here
	if err != nil {
		setupErr = multierror.Append(breverrors.WrapAndTrace(err))
	}
	err = appendLogToFile("starting to run execs", "/var/log/brev-steps.log")
	if err != nil {
		setupErr = multierror.Append(setupErr, breverrors.WrapAndTrace(err))
	}
	err = e.RunExecs() // here
	if err != nil {
		setupErr = multierror.Append(breverrors.WrapAndTrace(err))
	}

	err = e.CreateVerbYamlFile() // create
	if err != nil {
		setupErr = multierror.Append(breverrors.WrapAndTrace(err))
	}

	fmt.Println("------ Done running execs ------")
	err = appendLogToFile("done running execs", "/var/log/brev-steps.log")
	if err != nil {
		setupErr = multierror.Append(breverrors.WrapAndTrace(err))
	}

	err = e.brevMonConfigurer.Install()
	if err != nil {
		setupErr = multierror.Append(breverrors.WrapAndTrace(err))
	}

	if e.datadogAPIKey != "" {
		err = e.SetupDatadog()
		if err != nil {
			setupErr = multierror.Append(breverrors.WrapAndTrace(err))
		}
	}

	err = postPrepare.Await()
	if err != nil {
		setupErr = multierror.Append(breverrors.WrapAndTrace(err))
	}

	err = appendLogToFile("setup done", "/var/log/brev-steps.log")
	if err != nil {
		setupErr = multierror.Append(breverrors.WrapAndTrace(err))
	}

	if setupErr != nil {
		return breverrors.WrapAndTrace(setupErr)
	}

	return nil
}

func (e envInitier) SetupDatadog() error {
	installScriptURL := "https://s3.amazonaws.com/dd-agent/scripts/install_script.sh"
	var installScript string

	resp, err := collections.GetRequestWithContext(context.TODO(), installScriptURL)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	defer resp.Body.Close() //nolint: errcheck // we don't care about the error here b/c defer

	if resp.StatusCode != 200 {
		return breverrors.WrapAndTrace(fmt.Errorf("failed to download datadog install script"))
	}

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	installScript = string(bodyBytes)

	cmd := setupworkspace.CmdStringBuilder(installScript)

	cmd.Env = append(cmd.Env,
		append(
			os.Environ(),
			[]string{
				"DD_API_KEY=" + e.datadogAPIKey,
				"DD_AGENT_MAJOR_VERSION=7",
				"DD_SITE=\"datadoghq.com\"",
			}...,
		)...)

	err = cmd.Run()
	if err != nil {
		out, err0 := cmd.CombinedOutput()
		if err0 != nil {
			return breverrors.WrapAndTrace(err0)
		}
		return breverrors.WrapAndTrace(fmt.Errorf("failed to install datadog agent: %s", string(out)))
	}

	err = e.store.WriteString("/etc/datadog-agent/conf.d/systemd.d/conf.yaml", `
init_config:
instances:
    ## @param unit_names - list of strings - required
    ## List of systemd units to monitor.
    ## Full names must be used. Examples: ssh.service, docker.socket
    #
  - unit_names:
      - ssh.service
      - brevmon.service
`)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = e.store.WriteString("/etc/datadog-agent/conf.d/journald.d/conf.yaml", `
logs:
    - type: journald
      path: /var/log/journal/
      include_units:
          - brevmon.service
          - sshd.service
`)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = setupworkspace.BuildAndRunCmd(
		"/usr/sbin/usermod",
		"-a",
		"-G",
		"systemd-journal",
		"dd-agent",
	)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	hostname, _ := os.Hostname()
	stringToAppend := fmt.Sprintf("\nlogs_enabled: true\nhostname: %s\n", hostname)
	// add logs_enabled: true to /etc/datadog-agent/datadog.yaml
	err = e.store.AppendString("/etc/datadog-agent/datadog.yaml", stringToAppend)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = setupworkspace.BuildAndRunCmd(
		"systemctl",
		"restart",
		"datadog-agent",
	)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

type setupKeyI interface {
	WriteString(path, content string) error
	Chmod(path string, mode os.FileMode) error
	ChownFilePathToUser(path string) error
}

func setupKey(path, content string, perm os.FileMode, store setupKeyI) error {
	err := store.WriteString(path, content)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = store.Chmod(path, perm)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = store.ChownFilePathToUser(path)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (e envInitier) setupPrivateKey(content string) error {
	pkpath := e.BuildHomePath(".ssh", "id_rsa")
	err := setupKey(pkpath, content, 0o600, e.store)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (e envInitier) setupPublicKey(content string) error {
	pubkeypath := e.BuildHomePath(".ssh", "id_rsa.pub")
	err := setupKey(pubkeypath, content, 0o644, e.store)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (e envInitier) SetupSSHKeys(keys *store.KeyPair) error {
	err := e.setupPrivateKey(keys.PrivateKeyData)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = e.setupPublicKey(keys.PublicKeyData)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (e envInitier) SetupSSH() error {
	keys := e.Params.WorkspaceKeyPair
	err := e.SetupSSHKeys(keys)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	c := fmt.Sprintf(`eval "$(ssh-agent -s)" && ssh-add %s`, e.BuildHomePath(".ssh", "id_rsa"))
	cmd := setupworkspace.CmdStringBuilder(c)
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	authorizedKeyPath := e.BuildHomePath(".ssh", "authorized_keys")

	err = e.store.AppendString(authorizedKeyPath, "\n"+keys.PublicKeyData)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = e.store.ChownFilePathToUser(authorizedKeyPath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if e.ConfigureSystemSSHConfig {
		err = e.store.WriteString(
			filepath.Join(
				"/etc",
				"ssh",
				"sshd_config.d",
				fmt.Sprintf("%s.conf", e.User.Username),
			),
			fmt.Sprintf(
				`PubkeyAuthentication yes
AuthorizedKeysFile      %s
PasswordAuthentication no`, authorizedKeyPath),
		)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	return nil
}

func (e envInitier) SetupEnvVars() error {
	// set env vars
	err := e.store.AppendString("/etc/bash.bashrc", `
_brev_hook() {
  local previous_exit_status=$?;
  trap -- '' SIGINT;
  eval "$(/usr/local/bin/brev configure-env-vars bash)";
  trap - SIGINT;
  return $previous_exit_status;
};
if ! [[ "${PROMPT_COMMAND:-}" =~ _brev_hook ]]; then
  PROMPT_COMMAND="_brev_hook${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
fi
`)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fileExists, err := e.store.FileExists("/etc/zsh/zshrc")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !fileExists {
		err = e.store.WriteString("/etc/zsh/zshrc", "")
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	err = e.store.AppendString("/etc/zsh/zshrc", `
_brev_hook() {
  trap -- '' SIGINT;
  eval "$(/usr/local/bin/brev configure-env-vars zsh)";
  trap - SIGINT;
}
typeset -ag precmd_functions;
if [[ -z "${precmd_functions[(r)_brev_hook]+1}" ]]; then
  precmd_functions=( _brev_hook ${precmd_functions[@]} )
fi
typeset -ag chpwd_functions;
if [[ -z "${chpwd_functions[(r)_brev_hook]+1}" ]]; then
  chpwd_functions=( _brev_hook ${chpwd_functions[@]} )
fi
export PATH="/opt/conda/bin:$PATH"
`)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func newEnvIniter(
	user *user.User,
	params *store.SetupParamsV0,
	configureSystemSSHConfig bool,
	store envsetupStore,
	datadogAPIKey string,
	disableAbleAUtosop bool,
	reportInterval string,
	portToCheckTrafficOn string,
) *envInitier {
	workspaceIniter := setupworkspace.NewWorkspaceIniter(user.HomeDir, user, params)
	return &envInitier{
		*workspaceIniter,
		configureSystemSSHConfig,
		autostartconf.NewBrevMonConfigure(
			store,
			disableAbleAUtosop,
			reportInterval,
			portToCheckTrafficOn,
		),
		datadogAPIKey,
		store,
	}
}

func setupEnv(
	store envsetupStore,
	params *store.SetupParamsV0,
	configureSystemSSHConfig bool,
	datadogAPIKey string,
	disableAutoStop bool,
	reportInterval string,
	portToCheckAutostopTrafficOn string,
) error {
	err := store.BuildBrevHome()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	user, err := setupworkspace.GetUserFromUserStr(store.GetOSUser())
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	wi := newEnvIniter(
		user,
		params,
		configureSystemSSHConfig,
		store,
		datadogAPIKey,
		disableAutoStop,
		reportInterval,
		portToCheckAutostopTrafficOn,
	)
	// set logfile path to ~/.brev/envsetup.log
	logFilePath := filepath.Join(user.HomeDir, ".brev", "envsetup.log")
	done, err := mirrorPipesToFile(store, logFilePath)
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
		//nolint:gosec // constant
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

func mirrorPipesToFile(store envsetupStore, logFile string) (func(), error) {
	f, err := store.GetOrCreateSetupLogFile(logFile)
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
