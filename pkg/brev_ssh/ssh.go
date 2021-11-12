// Package brev_ssh exists to provide an api to configure and read from
// an ssh file
//
// brev ssh host file entry format:
//
// 	Host <workspace-dns-name
// 		Hostname 0.0.0.0
// 		IdentityFile /home//.brev/brev.pem
//		User brev
//		Port <some-available-port>
//
// also think that file stuff should probably live in files package
// TODO migrate to using dns name for hostname
package brev_ssh

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/kevinburke/ssh_config"
	"github.com/spf13/afero"
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
// 	[1/2] 6. truncate old config and write new config back to disk (making backup of original copy first)
// TODO: backup config before running these steps
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

	var activeWorkspacesNames []string
	for _, workspace := range workspaces {
		activeWorkspacesNames = append(activeWorkspacesNames, workspace.Name)
	}
	cfg, err := GetSSHConfig(files.AppFs)
	if err != nil {
		return err
	}

	err = CreateBrevSSHConfigEntries(files.AppFs, *cfg, activeWorkspacesNames)
	if err != nil {
		return err
	}
	// re get ssh cfg again from disk since we likely just modified it
	cfg, err = GetSSHConfig(files.AppFs)
	if err != nil {
		return err
	}
	newConfig := PruneInactiveWorkspaces(*cfg, activeWorkspacesNames)
	configPath, err := files.GetUserSSHConfigPath()
	if err != nil {
		return err
	}
	return files.OverwriteString(*configPath, newConfig)
}

func PruneInactiveWorkspaces(cfg ssh_config.Config, activeWorkspacesNames []string) string {
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
			for _, name := range activeWorkspacesNames {
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
	return newConfig
}

// todo this should prob return a cfg object, instead make sure your re get the cfg
// after calling this
func CreateBrevSSHConfigEntries(fs afero.Fs,cfg ssh_config.Config, activeWorkspacesNames []string) error {
	brevHostValues := GetBrevHostValues(cfg)
	brevHostValuesSet := make(map[string]bool)
	for _, hostValue := range brevHostValues {
		brevHostValuesSet[hostValue] = true
	}

	for _, workspaceName := range activeWorkspacesNames {
		if !brevHostValuesSet[workspaceName] {
			cfg, err := GetSSHConfig(fs)
			if err != nil {
				return err
			}
			ports, err := GetBrevPorts(*cfg, brevHostValues)
			if err != nil {
				return err
			}
			port := 2222
			for ports[fmt.Sprint(port)] {
				port++
			}
			file, err := files.GetOrCreateSSHConfigFile(fs)
			if err != nil {
				return err
			}
			err = appendBrevEntry(file, workspaceName, fmt.Sprint(port))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func checkIfBrevHost(host ssh_config.Host) bool {
	for _, node := range host.Nodes {
		switch n := node.(type) {
		case *ssh_config.KV:
			if strings.Compare(n.Key, "IdentityFile") == 0 {
				if strings.Compare(files.GetSSHPrivateKeyFilePath(), n.Value) == 0 {
					return true
				}
			}
		}
	}
	return false
}

func GetSSHConfig(fs afero.Fs) (*ssh_config.Config, error) {
	file, err := files.GetOrCreateSSHConfigFile(fs)
	if err != nil {
		return nil, err
	}
	cfg, err := ssh_config.Decode(file)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func GetBrevPorts(cfg ssh_config.Config, hostnames []string) (map[string]bool, error) {
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

// Hostname is a loaded term so using values
func GetBrevHostValues(cfg ssh_config.Config) []string {
	var brevHosts []string
	for _, host := range cfg.Hosts {
		hostname := hostnameFromString(host.String())
		// is this host a brev entry? if not, we don't care, and on to the
		// next one
		if checkIfBrevHost(*host) {
			brevHosts = append(brevHosts, hostname)
		}
	}
	return brevHosts
}

func hostnameFromString(hoststring string) string {
	return strings.Split(strings.Split(hoststring, "\n")[0], " ")[1]
}

// https://stackoverflow.com/questions/37334119/how-to-delete-an-element-from-a-slice-in-golang
func unorderedRemove(s []string, i int) []string {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func appendBrevEntry(file afero.File, workspaceName, port string) error {
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
