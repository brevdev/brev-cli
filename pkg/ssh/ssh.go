// Package ssh exists to provide an api to configure and read from
// an ssh file
//
// brev ssh host file entry format:
//
// 	Host <workspace-dns-name>
// 		Hostname 0.0.0.0
// 		IdentityFile /home//.brev/brev.pem
//		User brev
//		Port <some-available-port>
//
// also think that file stuff should probably live in files package
// TODO migrate to using dns name for hostname
package ssh

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/kevinburke/ssh_config"
)

const workspaceSSHConfigTemplate = `Host {{ .Host }}
  Hostname {{ .Hostname }}
  IdentityFile {{ .IdentityFile }}
  User brev
  Port {{ .Port }}

`

type (
	BrevPorts          map[string]bool
	BrevHostValuesSet  map[string]bool
	IdentityPortMap    map[string]string
	workspaceSSHConfig struct {
		Host         string
		Hostname     string
		User         string
		IdentityFile string
		Port         string
	}
	SSHStore interface {
		GetSSHConfig() (string, error)
		WriteSSHConfig(config string) error
		CreateNewSSHConfigBackup() error
		WritePrivateKey(pem string) error
		GetPrivateKeyFilePath() string
	}
	Reader interface {
		GetBrevPorts() (BrevPorts, error)
		GetBrevHostValueSet() BrevHostValuesSet
		GetConfiguredWorkspacePort(workspace entity.Workspace) (string, error)
	}
	Writer interface {
		Sync(activeWorkspaces []string, brevHostValues BrevHostValuesSet) error
	}
	SSHConfig struct {
		store      SSHStore
		sshConfig  *ssh_config.Config
		privateKey string
	}
	SSHConfigurer struct {
		Reader
		Writer
		Writers    []Writer
		workspaces []entity.WorkspaceWithMeta
	}
	JetBrainsGatewayConfigStore interface{}
	JetBrainsGatewayConfig      struct {
		Writer
		store JetBrainsGatewayConfigStore
	}
)

func hostnameFromString(hoststring string) string {
	hoststring = strings.TrimSpace(hoststring)
	if hoststring == "" {
		return ""
	}

	newLineSplit := strings.Split(hoststring, "\n")
	if len(newLineSplit) < 1 {
		return ""
	}
	spaceSplit := strings.Split(newLineSplit[0], " ")
	if len(spaceSplit) < 2 {
		return ""
	}

	return spaceSplit[1]
}

func checkIfBrevHost(host ssh_config.Host, privateKeyPath string) bool {
	for _, node := range host.Nodes {
		switch n := node.(type) { //nolint:gocritic // ignoring since want to keep options open for many cases
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

func checkIfHostIsActive(hoststring string, activeWorksSpaces []string) bool {
	maybeHostname := hostnameFromString(hoststring)
	for _, name := range activeWorksSpaces {
		if name == maybeHostname {
			return true
		}
	}
	return false
}

// if a host is not a brev entry, it should stay in the config and there
// is nothing for us to do to it.
// if the host is a brev entry, make sure that it's hostname maps to an
// active workspace, otherwise this host should be deleted.
func createConfigEntry(hoststring string, isBrevHost, isActiveHost bool) string {
	if !isBrevHost {
		return hoststring
	}
	if isBrevHost && isActiveHost {
		return hoststring
	}
	return ""
}

func sshConfigFromString(config string) (*ssh_config.Config, error) {
	sshConfig, err := ssh_config.Decode(strings.NewReader(config))
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return sshConfig, nil
}

func MakeSSHEntry(workspaceName, port, privateKeyPath string) (string, error) {
	wsc := workspaceSSHConfig{
		Host:         workspaceName,
		Hostname:     "0.0.0.0",
		User:         "brev",
		IdentityFile: privateKeyPath,
		Port:         port,
	}

	tmpl, err := template.New(workspaceName).Parse(workspaceSSHConfigTemplate)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, wsc)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	return buf.String(), nil
}

func NewSSHConfig(store SSHStore) (*SSHConfig, error) {
	configStr, err := store.GetSSHConfig()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	sshConfig, err := ssh_config.Decode(strings.NewReader(configStr))
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &SSHConfig{
		store:      store,
		sshConfig:  sshConfig,
		privateKey: store.GetPrivateKeyFilePath(),
	}, nil
}

func (s *SSHConfig) PruneInactiveWorkspaces(activeWorkspaces []string) error {
	newConfig := ""

	privateKeyPath := s.store.GetPrivateKeyFilePath()
	for _, host := range s.sshConfig.Hosts {
		hoststring := host.String()
		isBrevHost := checkIfBrevHost(*host, privateKeyPath)
		isActiveHost := checkIfHostIsActive(hoststring, activeWorkspaces)
		newConfig += createConfigEntry(hoststring, isBrevHost, isActiveHost)
	}

	sshConfig, err := sshConfigFromString(newConfig)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	s.sshConfig = sshConfig
	return nil
}

// Hostname is a loaded term so using values
func (s SSHConfig) GetBrevHostValues() []string {
	privateKeyPath := s.store.GetPrivateKeyFilePath()
	var brevHosts []string
	for _, host := range s.sshConfig.Hosts {
		hostname := hostnameFromString(host.String())
		// is this host a brev entry? if not, we don't care, and on to the
		// next one
		if checkIfBrevHost(*host, privateKeyPath) {
			brevHosts = append(brevHosts, hostname)
		}
	}
	return brevHosts
}

func (s *SSHConfig) Sync(activeWorkspaces []string, brevHostValuesSet BrevHostValuesSet) error {
	sshConfigStr := s.sshConfig.String()

	ports, err := s.GetBrevPorts()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	port := 2222

	identifierPortMapping := make(map[string]string)
	for _, workspaceIdentifier := range activeWorkspaces {
		if !brevHostValuesSet[workspaceIdentifier] {
			for ports[fmt.Sprint(port)] {
				port++
			}
			identifierPortMapping[workspaceIdentifier] = strconv.Itoa(port)
			entry, err2 := MakeSSHEntry(workspaceIdentifier, fmt.Sprint(port), s.privateKey)
			if err2 != nil {
				return breverrors.WrapAndTrace(err2)
			}
			sshConfigStr += entry
			ports[fmt.Sprint(port)] = true
		}
	}
	s.sshConfig, err = ssh_config.Decode(strings.NewReader(sshConfigStr))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = s.PruneInactiveWorkspaces(activeWorkspaces)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = s.store.WriteSSHConfig(s.sshConfig.String())
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (s SSHConfig) GetBrevPorts() (BrevPorts, error) {
	portSet := make(BrevPorts)
	hostnames := s.GetBrevHostValues()
	for _, name := range hostnames {
		port, err := s.sshConfig.Get(name, "Port")
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		portSet[port] = true
	}
	return portSet, nil
}

func (s SSHConfig) GetBrevHostValueSet() BrevHostValuesSet {
	brevHostValuesSet := make(BrevHostValuesSet)
	brevHostValues := s.GetBrevHostValues()
	for _, hostValue := range brevHostValues {
		brevHostValuesSet[hostValue] = true
	}
	return brevHostValuesSet
}

func (s SSHConfig) GetConfiguredWorkspacePort(workspace entity.Workspace) (string, error) {
	config, err := NewSSHConfig(s.store) // forces load from disk
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	port, err := config.sshConfig.Get(workspace.DNS, "Port")
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return port, nil
}

func NewSSHConfigurer(workspaces []entity.WorkspaceWithMeta, reader Reader, writer Writer, writers []Writer) *SSHConfigurer {
	return &SSHConfigurer{
		workspaces: workspaces,
		Reader:     reader,
		Writer:     writer,
		Writers:    writers,
	}
}

func (sshConfigurer *SSHConfigurer) Sync() error {
	activeWorkspaces := sshConfigurer.GetActiveWorkspaceIdentifiers()
	for _, writer := range sshConfigurer.Writers {
		brevHostValuesSet := sshConfigurer.Reader.GetBrevHostValueSet()
		err := writer.Sync(activeWorkspaces, brevHostValuesSet)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

	}

	return nil
}

func (sshConfigurer *SSHConfigurer) GetActiveWorkspaceIdentifiers() []string {
	var workspaceDNSNames []string
	for _, workspace := range sshConfigurer.workspaces {
		workspaceDNSNames = append(workspaceDNSNames, workspace.DNS)
	}
	return workspaceDNSNames
}

func (sshConfigurer SSHConfigurer) GetConfiguredWorkspacePort(workspace entity.Workspace) (string, error) {
	port, err := sshConfigurer.Reader.GetConfiguredWorkspacePort(workspace)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return port, nil
}

func NewJetBrainsGatewayConfig(writer Writer, store JetBrainsGatewayConfigStore) *JetBrainsGatewayConfig {
	return &JetBrainsGatewayConfig{
		Writer: writer,
		store:  store,
	}
}
