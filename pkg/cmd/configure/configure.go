// Package configure is for the configure command
package configure

import (
	"os"
	"path/filepath"

	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/kevinburke/ssh_config"
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

const DefaultSSHConfigEntry = `
Host brev
	 Hostname 0.0.0.0
	 IdentityFile ~/.brev/brev.pem
	 User brev
	 Port 2222
`

type configureOptions struct{}

func NewCmdConfigure() *cobra.Command {
	opts := configureOptions{}

	cmd := &cobra.Command{
		Annotations:           map[string]string{"housekeeping": ""},
		Use:                   "configure",
		DisableFlagsInUseLine: true,
		Short:                 "configure ssh for brev",
		Long:                  "configure ssh for brev",
		Example:               "brev configure",
		Args:                  cobra.NoArgs,
		// ValidArgsFunction: util.ResourceNameCompletionFunc(f, "pod"),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(opts.Complete(cmd, args))
			cmdutil.CheckErr(opts.Validate(cmd, args))
			cmdutil.CheckErr(opts.Runconfigure(cmd, args))
		},
	}
	return cmd
}

func (o *configureOptions) Complete(cmd *cobra.Command, args []string) error {
	// return fmt.Errorf("not implemented")
	return nil
}

func (o *configureOptions) Validate(cmd *cobra.Command, args []string) error {
	// return fmt.Errorf("not implemented")
	return nil
}

func (o *configureOptions) Runconfigure(cmd *cobra.Command, args []string) error {
	return configureSSH()
}

// configureSSH finds the user's ssh config file, checks to see if
// it has been configured for brev, and then sets it to the
// DefaultSSHConfigEntry if it has not been set.
func configureSSH() error {
	sshConfigPath := filepath.Join(os.Getenv("HOME"), ".ssh", "config")
	sshConfigExists, err := files.Exists(sshConfigPath, false)
	if err != nil {
		return err
	}
	var file *os.File
	if sshConfigExists {
		file, err = os.Open(sshConfigPath)
		if err != nil {
			return err
		}

	} else {
		file, err = os.Create(sshConfigPath)
		if err != nil {
			return err
		}
	}
	cfg, err := ssh_config.Decode(file)
	if err != nil {
		return err
	}
	hasValidBrevEntry, err := configHasValidBrevEntry(cfg)
	if err != nil {
		return err
	}

	if !hasValidBrevEntry {
		return createBrevEntry(file)
	}
	return nil
}

func configHasValidBrevEntry(cfg *ssh_config.Config) (bool, error) {
	brevHost := "brev"
	brevHostname, err := cfg.Get(brevHost, "Hostname")
	if err != nil {
		return false, err
	}
	brevIdentityFile, err := cfg.Get(brevHost, "IdentityFile")
	if err != nil {
		return false, err
	}
	brevUser, err := cfg.Get(brevHost, "User")
	if err != nil {
		return false, err
	}
	brevPort, err := cfg.Get(brevHost, "Port")
	if err != nil {
		return false, err
	}
	return brevHostname == "0.0.0.0" && brevIdentityFile == "~/.brev/brev.pem" && brevUser == "brev" && brevPort == "2222", nil
}

func createBrevEntry(file *os.File) error {
	_, err := file.WriteString(DefaultSSHConfigEntry)
	return err
}
