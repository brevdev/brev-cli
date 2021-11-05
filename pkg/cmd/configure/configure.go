// Package configure is for the configure command
//
// ssh host file format:
//
// 	Host <workspace-name>
// 		Hostname 0.0.0.0
// 		IdentityFile ~/.brev/brev.pem
//		User brev
//		Port <some-available-port>
//
// also think that file stuff should probably live in files package
package configure

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/kevinburke/ssh_config"
	"github.com/spf13/cobra"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type workspaceSSHConfig struct {
	Host         string
	Hostname     string
	User         string
	IdentityFile string
	Port         string
}

const workspaceSSHConfigTemplate = `
Host {{ .Host }}
	 Hostname {{ .Hostname }}
	 IdentityFile {{ .IdentityFile }}
	 User brev
	 Port {{ .Port }}
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
			cmdutil.CheckErr(opts.RunConfigure(cmd, args))
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

func (o *configureOptions) RunConfigure(cmd *cobra.Command, args []string) error {
	return ConfigureSSH()
}

// ConfigureSSH
// 	[ ] 0. checks to see if a user has configured their ssh private key
// 	[x] 1. gets a list of the current user's workspaces
// 	[x] 2. finds the user's ssh config file,
// 	[x] 3. looks at entries in the ssh config file and:
//         for each active workspace from brev delpoy
//            create ssh config entry if it does not exist
// 	[x] 4. After creating the ssh config entries, prune entries from workspaces
//        that exist in the ssh config but not as active workspaces.
// 	[ ] 5. Check for and remove duplicates?
func ConfigureSSH() error {
	// to get workspaces, we need to get the active org
	activeorg, err := brev_api.GetActiveOrgContext()
	if err != nil {
		return err
	}
	client, err := brev_api.NewClient()
	if err != nil {
		return err
	}
	workspaces, err := client.GetMyWorkspaces(activeorg.ID)

	var workspaceNames []string
	for _, workspace := range workspaces {
		workspaceNames = append(workspaceNames, workspace.Name)
	}
	cfg, err := getSSHConfig()
	if err != nil {
		return err
	}
	namesToCreate, existingNames := filterExistingWorkspaceNames(workspaceNames[:], *cfg)
	for _, name := range namesToCreate {
		// re get ssh config from disk at begining of loop b/c it's modified
		// at the end of the loop
		cfg, err = getSSHConfig()
		if err != nil {
			return err
		}
		// TODO getPort func?
		ports, err := getBrevPorts(*cfg, existingNames)
		if err != nil {
			return err
		}
		port := 2222
		for ports[fmt.Sprint(port)] {
			port++
		}
		err = appendBrevEntry(name, fmt.Sprint(port))
		if err != nil {
			return err
		}
	}

	// re get ssh cfg again from disk since we likely just modified it
	cfg, err = getSSHConfig()
	if err != nil {
		return err
	}

	newConfig := ""

	for _, host := range cfg.Hosts {
		// if a host is not a brev entry, it should stay in the config and there
		// is nothing for us to do to it.
		// if the host is a brev entry, make sure that it's hostname maps to an
		// active workspace, otherwise this host should be deleted.
		brevEntry := checkIfBrevHost(*host)
		if brevEntry {
			// if this host does not match a workspacename, then delete since it belongs to an inactive
			// workspace or deleted one.
			foundMatch := false
			for _, name := range workspaceNames {
				if host.Matches(name) {
					foundMatch = true
					break
				}
			}
			if foundMatch {
				newConfig = newConfig + host.String()
			}
		} else {
			newConfig = newConfig + host.String()
		}

	}
	return nil
}

func checkIfBrevHost(host ssh_config.Host) bool {
	brevEntry := false
	for _, node := range host.Nodes {
		if strings.Contains(node.String(), "~/.brev/brev.pem") {
			brevEntry = true
			break
		}
	}
	return brevEntry
}

func getSSHConfigFile() (*os.File, error) {
	sshConfigPath := filepath.Join(os.Getenv("HOME"), ".ssh", "config")
	sshConfigExists, err := files.Exists(sshConfigPath, false)
	if err != nil {
		return nil, err
	}
	var file *os.File
	if sshConfigExists {
		file, err = os.Open(sshConfigPath)
		if err != nil {
			return nil, err
		}
	} else {
		file, err = os.Create(sshConfigPath)
		if err != nil {
			return nil, err
		}
	}
	return file, nil
}

func getSSHConfig() (*ssh_config.Config, error) {
	file, err := getSSHConfigFile()
	if err != nil {
		return nil, err
	}
	cfg, err := ssh_config.Decode(file)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func getBrevPorts(cfg ssh_config.Config, hostnames []string) (map[string]bool, error) {
	portSet := make(map[string]bool)

	for _, name := range hostnames {
		port, err := cfg.Get(name, "Port")
		if err != nil {
			return nil, err
		}
		portSet[port] = true
	}
	return portSet, nil
}

func filterExistingWorkspaceNames(workspaceNames []string, cfg ssh_config.Config) ([]string, []string) {
	var existingNames []string
	for _, host := range cfg.Hosts {
		// is this host a brev entry? if not, we don't care, and on to the
		// next one
		// TODO maybe not brute force here?
		brevEntry := false
		var nameToRemove int
		for nameIndex, name := range workspaceNames {
			if host.Matches(name) {
				brevEntry = true
				nameToRemove = nameIndex
				break
			}
		}
		if brevEntry {
			existingNames = append(existingNames, workspaceNames[nameToRemove])
			unorderedRemove(workspaceNames, nameToRemove)
		}
	}
	return workspaceNames, existingNames
}

// https://stackoverflow.com/questions/37334119/how-to-delete-an-element-from-a-slice-in-golang
func unorderedRemove(s []string, i int) []string {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

// sshConfigHasValidEntry checks a user's ssh config to see if their is a valid
// entry for a workspace, which is defined as:
//
// 		Host <workspace-name>
// 	 		Hostname 0.0.0.0
// 	 		IdentityFile ~/.brev/brev.pem
// 	 		User brev
// 	 		Port <some-random-port>
//
// a workspace's config definition must use a unique port
func GetWorkspaceSSHConfig(cfg *ssh_config.Config, workspaceName string) (*workspaceSSHConfig, error) {
	Hostname, err := cfg.Get(workspaceName, "Hostname")
	if err != nil {
		return nil, err
	}
	IdentityFile, err := cfg.Get(workspaceName, "IdentityFile")
	if err != nil {
		return nil, err
	}
	User, err := cfg.Get(workspaceName, "User")
	if err != nil {
		return nil, err
	}
	Port, err := cfg.Get(workspaceName, "Port")
	if err != nil {
		return nil, err
	}
	return &workspaceSSHConfig{
		Host:         Hostname,
		IdentityFile: IdentityFile,
		User:         User,
		Port:         Port,
	}, nil
}

func appendBrevEntry(workspaceName, port string) error {
	file, err := getSSHConfigFile()
	if err != nil {
		return err
	}
	workspaceSSHConfig := workspaceSSHConfig{
		Host:         workspaceName,
		Hostname:     "0.0.0.0",
		User:         "brev",
		IdentityFile: files.GetSSHPrivateKeyFilePath(),
		Port:         port,
	}

	tmpl, err := template.New(workspaceName).Parse(workspaceSSHConfigTemplate)
	if err != nil {
		return err
	}
	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, workspaceSSHConfig)
	if err != nil {
		return err
	}
	_, err = file.Write(buf.Bytes())
	return err
}
