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
}

type uFunc func(t *terminal.Terminal, args []string, store upgradeStore) error

func NewCmdUpgrade(t *terminal.Terminal, store upgradeStore) *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "upgrade",
		DisableFlagsInUseLine: true,
		Short:                 short,
		Long:                  long,
		Example:               example,
		RunE: func(cmd *cobra.Command, args []string) error {
			err := RunUpgrade(t, args, store)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			t.Green("Upgrade complete!")
			return nil
		},
	}
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

var upgradeFuncs = map[string]func(t *terminal.Terminal, args []string, store upgradeStore) error{
	"darwin": func(_ *terminal.Terminal, _ []string, _ upgradeStore) error {
		err := runcmd("brew", "upgrade", "brevdev/homebrew-brev/brev")
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		return nil
	},
	"linux": func(_ *terminal.Terminal, _ []string, store upgradeStore) error {
		uid := store.GetOSUser()
		if uid != "0" { // root is uid 0 almost always
			return breverrors.New("You must be root to upgrade, re run with: sudo brev upgrade")
		}

		err := runcmd(
			"bash",
			"-c",
			"\"$(curl -fsSL https://raw.githubusercontent.com/brevdev/brev-cli/main/bin/install-latest.sh)\"",
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

func RunUpgrade(t *terminal.Terminal, args []string, store upgradeStore) error {
	upgradeFunc := getUpgradeFunc()
	if upgradeFunc == nil {
		return breverrors.New("unsupported OS")
	}

	err := upgradeFunc(t, args, store)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}
