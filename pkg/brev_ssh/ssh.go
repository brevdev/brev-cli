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
	"strconv"
	"strings"
	"text/template"

	"github.com/brevdev/brev-cli/pkg/brev_errors"
	"github.com/brevdev/brev-cli/pkg/brevapi"
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

type SSHStore interface {
	GetSSHConfig() (string, error)
	WriteSSHConfig(config string) error
	CreateNewSSHConfigBackup() error
	WritePrivateKey(pem string) error

	GetPrivateKeyFilePath() string
}

type DefaultSSHConfigurer struct {
	sshStore   SSHStore
	privateKey string

	workspaces []brevapi.WorkspaceWithMeta
	sshConfig  ssh_config.Config

	getActiveOrg func(fs afero.Fs) (*brevapi.Organization, error)
}

func NewDefaultSSHConfigurer(workspaces []brevapi.WorkspaceWithMeta, sshStore SSHStore, privateKey string) *DefaultSSHConfigurer {
	return &DefaultSSHConfigurer{
		workspaces: workspaces,
		sshStore:   sshStore,
		privateKey: privateKey,
	}
}

// ConfigureSSH
// inject active org id
// maybe just inject workspaces
// use project name instead of dns

// 	[x] 0. writes private key to disk
// 	[x] 1. gets a list of the current user's workspaces
// 	[x] 2. finds the user's ssh config file,
// 	[x] 3. looks at entries in the ssh config file and:
//         for each active workspace from brev delpoy
//            create ssh config entry if it does not exist
// 	[x] 4. After creating the ssh config entries, prune entries from workspaces
//        that exist in the ssh config but not as active workspaces.
// 	[ ] 5. Check for and remove duplicates?
// 	[x] 6. truncate old config and write new config back to disk (making backup of original copy first)
func (s *DefaultSSHConfigurer) Config() error {
	err := s.sshStore.WritePrivateKey(s.privateKey)
	if err != nil {
		return brev_errors.WrapAndTrace(err)
	}

	// before doing potentially destructive work, backup the config
	err = s.sshStore.CreateNewSSHConfigBackup()
	if err != nil {
		return brev_errors.WrapAndTrace(err)
	}

	configStr, err := s.sshStore.GetSSHConfig()
	if err != nil {
		return brev_errors.WrapAndTrace(err)
	}

	cfg, err := ssh_config.Decode(strings.NewReader(configStr))
	if err != nil {
		return brev_errors.WrapAndTrace(err)
	}

	var workspaceNames []string
	for _, workspace := range s.workspaces {
		workspaceNames = append(workspaceNames, workspace.Name)
	}

	cfg, err = s.CreateBrevSSHConfigEntries(*cfg, workspaceNames)
	if err != nil {
		return brev_errors.WrapAndTrace(err)
	}

	cfg, err = s.PruneInactiveWorkspaces(cfg, workspaceNames)
	if err != nil {
		return brev_errors.WrapAndTrace(err)
	}

	err = s.sshStore.WriteSSHConfig(cfg.String())
	if err != nil {
		return brev_errors.WrapAndTrace(err)
	}

	s.sshConfig = *cfg

	return nil
}

func (s DefaultSSHConfigurer) GetConfiguredWorkspacePort(workspace brevapi.Workspace) (string, error) {
	port, err := s.sshConfig.Get(workspace.DNS, "Port")
	if err != nil {
		return "", brev_errors.WrapAndTrace(err)
	}
	return port, nil
}

func (s DefaultSSHConfigurer) PruneInactiveWorkspaces(cfg *ssh_config.Config, activeWorkspacesNames []string) (*ssh_config.Config, error) {
	newConfig := ""

	privateKeyPath := s.sshStore.GetPrivateKeyFilePath()

	for _, host := range cfg.Hosts {
		// if a host is not a brev entry, it should stay in the config and there
		// is nothing for us to do to it.
		// if the host is a brev entry, make sure that it's hostname maps to an
		// active workspace, otherwise this host should be deleted.
		isBrevHost := checkIfBrevHost(*host, privateKeyPath)
		if isBrevHost {
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
				newConfig += host.String()
			}
		} else {
			newConfig += host.String()
		}
	}

	cfg, err := ssh_config.Decode(strings.NewReader(newConfig))
	if err != nil {
		return nil, brev_errors.WrapAndTrace(err)
	}

	return cfg, nil
}

func (s DefaultSSHConfigurer) CreateBrevSSHConfigEntries(cfg ssh_config.Config, activeWorkspacesIdentifiers []string) (*ssh_config.Config, error) {
	brevHostValues := s.GetBrevHostValues(cfg)
	brevHostValuesSet := make(map[string]bool)
	for _, hostValue := range brevHostValues {
		brevHostValuesSet[hostValue] = true
	}

	sshConfigStr := cfg.String()

	ports, err := GetBrevPorts(cfg, brevHostValues)
	if err != nil {
		return nil, brev_errors.WrapAndTrace(err)
	}
	port := 2222

	identifierPortMapping := make(map[string]string)
	for _, workspaceIdentifier := range activeWorkspacesIdentifiers {
		if !brevHostValuesSet[workspaceIdentifier] {
			for ports[fmt.Sprint(port)] {
				port++
			}
			identifierPortMapping[workspaceIdentifier] = strconv.Itoa(port)
			entry, err := s.makeSSHEntry(workspaceIdentifier, fmt.Sprint(port))
			if err != nil {
				return nil, brev_errors.WrapAndTrace(err)
			}
			sshConfigStr += entry
			ports[fmt.Sprint(port)] = true
		}
	}

	return ssh_config.Decode(strings.NewReader(sshConfigStr))
}

func checkIfBrevHost(host ssh_config.Host, privateKeyPath string) bool {
	for _, node := range host.Nodes {
		switch n := node.(type) {
		case *ssh_config.KV:
			if strings.Compare(n.Key, "IdentityFile") == 0 {
				if strings.Compare(privateKeyPath, n.Value) == 0 {
					return true
				}
			}
		}
	}
	return false
}

func GetBrevPorts(cfg ssh_config.Config, hostnames []string) (map[string]bool, error) {
	portSet := make(map[string]bool)

	for _, name := range hostnames {
		port, err := cfg.Get(name, "Port")
		if err != nil {
			return nil, brev_errors.WrapAndTrace(err)
		}
		portSet[port] = true
	}
	return portSet, nil
}

// Hostname is a loaded term so using values
func (s DefaultSSHConfigurer) GetBrevHostValues(cfg ssh_config.Config) []string {
	privateKeyPath := s.sshStore.GetPrivateKeyFilePath()
	var brevHosts []string
	for _, host := range cfg.Hosts {
		hostname := hostnameFromString(host.String())
		// is this host a brev entry? if not, we don't care, and on to the
		// next one
		if checkIfBrevHost(*host, privateKeyPath) {
			brevHosts = append(brevHosts, hostname)
		}
	}
	return brevHosts
}

func hostnameFromString(hoststring string) string {
	switch hoststring {
	case "":
		return hoststring
	case "\n":
		return hoststring
	}
	return strings.Split(strings.Split(hoststring, "\n")[0], " ")[1]
}

func (s DefaultSSHConfigurer) makeSSHEntry(workspaceName, port string) (string, error) {
	wsc := workspaceSSHConfig{
		Host:         workspaceName,
		Hostname:     "0.0.0.0",
		User:         "brev",
		IdentityFile: s.sshStore.GetPrivateKeyFilePath(),
		Port:         port,
	}

	tmpl, err := template.New(workspaceName).Parse(workspaceSSHConfigTemplate)
	if err != nil {
		return "", brev_errors.WrapAndTrace(err)
	}
	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, wsc)
	if err != nil {
		return "", brev_errors.WrapAndTrace(err)
	}

	return buf.String(), nil
}
