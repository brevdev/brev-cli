package envsetup

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/cmd/version"
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
	return cmd
}

func RunEnvSetup(
	store envsetupStore,
	name string,
	forceEnableSetup, debugger, configureSystemSSHConfig bool,
	workspaceid, token string,
	noLoginStore nologinEnvStore,
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
	fmt.Println("setting up dev environment")

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

	err = setupEnv(store, params, configureSystemSSHConfig)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fmt.Println("done setting up dev environment")
	return nil
}

type envInitier struct {
	setupworkspace.WorkspaceIniter
	ConfigureSystemSSHConfig bool
	brevMonConfigurer        autostartconf.DaemonConfigurer
}

func (e envInitier) Setup() error {
	cmd := setupworkspace.CmdStringBuilder("echo user: $(whoami) && echo pwd: $(pwd)")
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	postPrepare := util.RunEAsync(
		func() error {
			err0 := e.SetupVsCodeExtensions(e.VscodeExtensionIDs)
			if err0 != nil {
				fmt.Println(err0)
			}
			return nil
		},
	)

	err = e.brevMonConfigurer.Install()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = e.SetupSSH(e.Params.WorkspaceKeyPair)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = e.SetupGit(e.Params.WorkspaceUsername, e.Params.WorkspaceEmail)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = e.RunApplicationScripts(e.Params.WorkspaceApplicationStartScripts)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	var setupErr error

	err = e.SetupRepos()
	if err != nil {
		setupErr = multierror.Append(breverrors.WrapAndTrace(err))
	}
	fmt.Println("------ Git repo cloned ------")

	err = e.RunExecs()
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

func (e envInitier) SetupSSH(keys *store.KeyPair) error {
	cmd := setupworkspace.CmdBuilder("mkdir", "-p", e.BuildHomePath(".ssh"))
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = createPrivateKey(e, keys)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = createPublicKey(e, keys)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	c := fmt.Sprintf(`eval "$(ssh-agent -s)" && ssh-add %s`, e.BuildHomePath(".ssh", "id_rsa"))
	cmd = setupworkspace.CmdStringBuilder(c)
	err = cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	authorizedKeyPath := e.BuildHomePath(".ssh", "authorized_keys")

	err = appendToAuthorizedKeys(e, keys, authorizedKeyPath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if e.ConfigureSystemSSHConfig {
		err = configureSystemSSHConfig(e, authorizedKeyPath)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	return nil
}

// extract creating private key to function
func createPrivateKey(e envInitier, keys *store.KeyPair) error {
	idRsa, err := os.Create(e.BuildHomePath(".ssh", "id_rsa"))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	defer setupworkspace.PrintErrFromFunc(idRsa.Close)
	_, err = idRsa.Write([]byte(keys.PrivateKeyData))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = idRsa.Chmod(0o400)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = e.ChownFileToUser(idRsa)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func createPublicKey(e envInitier, keys *store.KeyPair) error {
	idRsaPub, err := os.Create(e.BuildHomePath(".ssh", "id_rsa.pub"))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	defer setupworkspace.PrintErrFromFunc(idRsaPub.Close)
	_, err = idRsaPub.Write([]byte(keys.PublicKeyData))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = idRsaPub.Chmod(0o400)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = e.ChownFileToUser(idRsaPub)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func appendToAuthorizedKeys(e envInitier, keys *store.KeyPair, authorizedKeyPath string) error {
	//nolint:gosec //todo is this a prob?
	authorizedKeyFile, err := os.OpenFile(authorizedKeyPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	defer setupworkspace.PrintErrFromFunc(authorizedKeyFile.Close)
	_, err = authorizedKeyFile.Write([]byte(keys.PublicKeyData))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	// chown to user
	err = e.ChownFileToUser(authorizedKeyFile)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func configureSystemSSHConfig(e envInitier, authorizedKeyPath string) error {
	sshConfigPath := filepath.Join("/etc", "ssh", "sshd_config.d", fmt.Sprintf("%s.conf", e.User.Username))
	sshConfMod := fmt.Sprintf(`PubkeyAuthentication yes
AuthorizedKeysFile      %s
PasswordAuthentication no`, authorizedKeyPath)
	err := os.WriteFile(sshConfigPath, []byte(sshConfMod), 0o644) //nolint:gosec // verified based on curr env
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func newEnvIniter(user *user.User, params *store.SetupParamsV0, configureSystemSSHConfig bool, store autostartconf.AutoStartStore) *envInitier {
	workspaceIniter := setupworkspace.NewWorkspaceIniter(user.HomeDir, user, params)

	return &envInitier{
		*workspaceIniter,
		configureSystemSSHConfig,
		autostartconf.NewBrevMonConfigure(store),
	}
}

func setupEnv(store envsetupStore, params *store.SetupParamsV0, configureSystemSSHConfig bool) error {
	err := store.BuildBrevHome()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	user, err := setupworkspace.GetUserFromUserStr(store.GetOSUser())
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	wi := newEnvIniter(user, params, configureSystemSSHConfig, store)
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
