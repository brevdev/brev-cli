package upgrade

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/samber/mo"
	"github.com/spf13/cobra"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

var (
	short   = "TODO"
	long    = "TODO"
	example = "TODO"
)

type upgradeStore interface {
	GetOSUser() string
	DownloadURL() (string, error)
	DownloadBrevBinary(url, path string) error
}

type uFunc func(ucmd upgradeCMD) error

func NewCmdUpgrade(t *terminal.Terminal, store upgradeStore) *cobra.Command {
	var debugger bool
	cmd := &cobra.Command{
		Use:                   "upgrade",
		DisableFlagsInUseLine: true,
		Short:                 short,
		Long:                  long,
		Example:               example,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunUpgrade(upgradeCMD{
				t:        t,
				args:     args,
				store:    store,
				debugger: debugger,
			})
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			t.Green("Upgrade complete!")
			return nil
		},
	}
	cmd.Flags().BoolVarP(&debugger, "debugger", "d", false, "indicates command is being run in debugger") // todo remove -d
	return cmd
}

type saveOutput struct {
	savedOutput []byte
}

func (so *saveOutput) Write(p []byte) (n int, err error) {
	so.savedOutput = append(so.savedOutput, p...)
	n, err = os.Stdout.Write(p)
	if err != nil {
		return n, breverrors.WrapAndTrace(err)
	}
	return n, nil
}

func runcmd(c string, args ...string) error {
	var so saveOutput
	cmd := exec.Command(c, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = &so
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err, string(so.savedOutput))
	}

	return nil
}

var upgradeFuncs = map[string]uFunc{
	"darwin": func(ucmd upgradeCMD) error {
		err := runcmd("brew", "upgrade", "brevdev/homebrew-brev/brev")
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		return nil
	},
	"linux": func(ucmd upgradeCMD) error {
		uid := ucmd.store.GetOSUser()
		if uid != "0" && !ucmd.debugger { // root is uid 0 almost always
			return breverrors.New("You must be root to upgrade, re run with: sudo brev upgrade")
		}
		// get cli download url
		url, err := ucmd.store.DownloadURL()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		// download binary
		err = ucmd.store.DownloadBrevBinary(
			url,
			"/usr/local/bin/brev",
		)

		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		return nil
	},
}

func getUpgradeFunc() uFunc {
	upgradeFunc, ok := upgradeFuncs[runtime.GOOS]
	return mo.TupleToOption(upgradeFunc, ok).OrEmpty()
}

type upgradeCMD struct {
	t        *terminal.Terminal
	args     []string
	store    upgradeStore
	debugger bool
}

func RunUpgrade(ucmd upgradeCMD) error {
	upgradeFunc := getUpgradeFunc()
	if upgradeFunc == nil {
		return breverrors.New("unsupported OS")
	}

	err := upgradeFunc(ucmd)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}
