package envsetup

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/user"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/cmd/version"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/setupworkspace"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/util"
)

type envsetupStore interface {
	GetSetupParams() (*store.SetupParamsV0, error)
	WriteSetupScript(script string) error
	GetSetupScriptPath() string
	GetCurrentUser() (*entity.User, error)
	GetCurrentWorkspaceID() (string, error)
}

const name = "envsetup"

func NewCmdEnvSetup(store envsetupStore) *cobra.Command {
	var forceEnableSetup bool

	cmd := &cobra.Command{
		Use:                   name,
		DisableFlagsInUseLine: true,
		Short:                 "TODO",
		Long:                  "TODO",
		Example:               "TODO",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunEnvSetup(store, name, forceEnableSetup)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.PersistentFlags().BoolVar(&forceEnableSetup, "force-enable", false, "force the setup script to run despite params")

	return cmd
}

func RunEnvSetup(store envsetupStore, name string, forceEnableSetup bool) error {
	breverrors.GetDefaultErrorReporter().AddTag("command", name)
	_, err := store.GetCurrentWorkspaceID() // do this to error reporting
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	fmt.Println("setting up dev environment")

	params, err := store.GetSetupParams()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if !featureflag.IsDev() {
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

	err = setupEnv(params)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	fmt.Println("done setting up dev environment")
	return nil
}

type envInitier struct {
	WorkspaceDir       string
	User               *user.User
	Params             *store.SetupParamsV0
	ReposV0            entity.ReposV0
	ExecsV0            entity.ExecsV0
	ReposV1            entity.ReposV1
	ExecsV1            entity.ExecsV1
	VscodeExtensionIDs []string
	setupworkspace.WorkspaceIniter
}

func (e envInitier) Setup() error {
	cmd := setupworkspace.CmdStringBuilder("echo user: $(whoami) && echo pwd: $(pwd)")
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = e.PrepareWorkspace()
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

func (e envInitier) PrepareWorkspace() error {
	return nil
}

func newEnvIniter(user *user.User, params *store.SetupParamsV0) *envInitier {
	userRepo := setupworkspace.MakeUserRepo(*params)
	projectReop := setupworkspace.MakeProjectRepo(*params)

	params.ReposV0 = setupworkspace.MergeRepos(userRepo, projectReop, params.ReposV0)

	workspaceDir := "/home/brev/workspace"

	params.ReposV0 = setupworkspace.InitRepos(params.ReposV0)

	if (params.ExecsV0 == nil || len(params.ExecsV0) == 0) && (params.ProjectSetupScript == nil || *params.ProjectSetupScript == "") {
		defaultScript := "#!/bin/bash\n"
		b64DefaultScript := base64.StdEncoding.EncodeToString([]byte(defaultScript))
		params.ProjectSetupScript = &b64DefaultScript
	}

	standardSetup := setupworkspace.MakeExecFromSetupParams(*params)

	params.ExecsV0 = setupworkspace.MergeExecs(standardSetup, params.ExecsV0)

	vscodeExtensionIDs := []string{}
	ideConfig, ok := params.IDEConfigs["vscode"]
	if ok {
		vscodeExtensionIDs = ideConfig.ExtensionIDs
	}

	return &envInitier{
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

func setupEnv(params *store.SetupParamsV0) error {
	user, err := setupworkspace.GetUserFromUserStr("1000")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	wi := newEnvIniter(user, params)
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
