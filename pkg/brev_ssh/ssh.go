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
// 	[ ] 6. truncate old config and write new config back to disk (making backup of original copy first)
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
	namesToCreate, existingNames := SplitWorkspaceByConfigMembership(workspaceNames, *cfg)
	for _, name := range namesToCreate {
		// re get ssh config from disk at begining of loop b/c it's modified
		// at the end of the loop
		cfg, err = getSSHConfig()
		if err != nil {
			return err
		}
		// TODO getPort func?
		ports, err := GetBrevPorts(*cfg, existingNames)
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

func getSSHConfig() (*ssh_config.Config, error) {
	file, err := files.GetOrCreateSSHConfigFile()
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

// SplitWorkspaceByConfigMembership given a list of
func SplitWorkspaceByConfigMembership(workspaceNames []string, cfg ssh_config.Config) ([]string, []string) {
	var members []string
	var brevHosts []string
	memberMap := make(map[string]bool)
	for _, host := range cfg.Hosts {

		hostname := hostnameFromString(host.String())
		// is this host a brev entry? if not, we don't care, and on to the
		// next one
		if checkIfBrevHost(*host) {
			brevHosts = append(brevHosts, hostname)
		}
		// TODO maybe not brute force here?
		for _, name := range workspaceNames {
			if strings.Compare(name, hostname) == 0 {
				members = append(members, name)
				memberMap[name] = true
				break
			}
		}
	}
	var excluded []string
	for _, name := range brevHosts {
		if !memberMap[name] {
			excluded = append(excluded, name)
		}
	}
	return members, excluded
}

func hostnameFromString(hoststring string) string {
	return strings.Split(strings.Split(hoststring, "\n")[0], " ")[1]
}

// https://stackoverflow.com/questions/37334119/how-to-delete-an-element-from-a-slice-in-golang
func unorderedRemove(s []string, i int) []string {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func appendBrevEntry(workspaceName, port string) error {
	file, err := files.GetOrCreateSSHConfigFile()
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

// given a workspace name string give me a port
// will be changing to workspaceDNSName shortly
func GetWorkspaceLocalSSHPort(workspaceName string) (string, error) {
	cfg, err := getSSHConfig()
	if err != nil {
		return "", err
	}
	return cfg.Get(workspaceName, "Port")
}
