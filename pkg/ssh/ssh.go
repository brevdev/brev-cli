// Package ssh exists to provide an api to configure and read from
// an ssh file
//
// brev ssh host file entry format:
//
//	Host <workspace-dns-name>
//		Hostname 0.0.0.0
//		IdentityFile /home//.brev/brev.pem
//		User brev
//		Port <some-available-port>
//
// also think that file stuff should probably live in files package
// TODO migrate to using dns name for hostname
package ssh

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/kevinburke/ssh_config"
)

const workspaceSSHConfigTemplate = `Host {{ .Host }}
  Hostname {{ .Hostname }}
  IdentityFile {{ .IdentityFile }}
  User brev
  Port {{ .Port }}
  UserKnownHostsFile /dev/null
  StrictHostKeyChecking no
  PasswordAuthentication no
  RequestTTY yes
  RemoteCommand cd {{ .Dir }}; $SHELL

`

type (
	BrevPorts          map[string]bool
	BrevHostValuesSet  map[entity.WorkspaceLocalID]bool
	IdentityPortMap    map[entity.WorkspaceLocalID]string
	workspaceSSHConfig struct {
		Host         entity.WorkspaceLocalID
		Hostname     string
		User         string
		IdentityFile string
		Port         string
		Dir          string
	}
	SSHStore interface {
		GetUserSSHConfig() (string, error)
		WriteUserSSHConfig(config string) error
		CreateNewSSHConfigBackup() error
		GetPrivateKeyPath() (string, error)
	}
	Reader interface {
		GetBrevPorts() (BrevPorts, error)
		GetBrevHostValueSet() BrevHostValuesSet
		GetConfiguredWorkspacePort(workspaceIdentifier entity.WorkspaceLocalID) (string, error)
	}
	Writer interface {
		Sync(IdentityPortMap) error
	}
	SSHConfig struct {
		store      SSHStore
		sshConfig  *ssh_config.Config
		privateKey string
	}
	SSHConfigurerStore interface {
		WritePrivateKey(pem string) error
	}
	SSHConfigurer struct {
		store      SSHConfigurerStore
		Reader     Reader
		Writers    []Writer
		workspaces []entity.WorkspaceWithMeta
		privateKey string
	}
	JetBrainsGatewayConfigStore interface {
		GetJetBrainsConfigPath() (string, error)
		GetJetBrainsConfig() (string, error)
		WriteJetBrainsConfig(config string) error
		GetPrivateKeyPath() (string, error)
	}
	JetBrainsGatewayConfig struct {
		config *JetbrainsGatewayConfigXML
		Reader
		Writer
		store JetBrainsGatewayConfigStore
	}
	JetbrainsGatewayConfigXMLSSHOption struct {
		Name  string `xml:"name,attr,omitempty"`
		Value string `xml:"value,attr,omitempty"`
	}
	JetbrainsGatewayConfigXMLSSHConfig struct {
		ID               string                               `xml:"id,attr,omitempty"`
		CustomName       entity.WorkspaceLocalID              `xml:"customName,attr,omitempty"`
		NameFormat       string                               `xml:"nameFormat,attr,omitempty"`
		UseOpenSSHConfig string                               `xml:"useOpenSSHConfig,attr,omitempty"`
		Host             string                               `xml:"host,attr,omitempty"`
		Port             string                               `xml:"port,attr,omitempty"`
		KeyPath          string                               `xml:"keyPath,attr,omitempty"`
		Username         string                               `xml:"username,attr,omitempty"`
		Options          []JetbrainsGatewayConfigXMLSSHOption `xml:"option,omitempty"`
	}
	JetbrainsGatewayConfigXMLConfigs struct {
		SSHConfigs []JetbrainsGatewayConfigXMLSSHConfig `xml:"sshConfig"`
	}
	JetbrainsGatewayConfigXMLComponent struct {
		Configs JetbrainsGatewayConfigXMLConfigs `xml:"configs"`
		Name    string                           `xml:"name,attr,omitempty"`
	}

	JetbrainsGatewayConfigXML struct {
		XMLName   xml.Name                           `xml:"application"`
		Component JetbrainsGatewayConfigXMLComponent `xml:"component"`
	}
)

func workspaceIdentifierFromHost(hoststring string) entity.WorkspaceLocalID {
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

	return entity.WorkspaceLocalID(spaceSplit[1])
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

func checkIfHostIsActive(hoststring string, activeWorksSpaces []entity.WorkspaceLocalID) bool {
	maybeHostname := workspaceIdentifierFromHost(hoststring)
	for _, workspaceIdentifier := range activeWorksSpaces {
		if workspaceIdentifier == maybeHostname {
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

func MakeSSHEntry(workspaceIdentifier entity.WorkspaceLocalID, port string, privateKeyPath string, dir string) (string, error) {
	wsc := workspaceSSHConfig{
		Host:         workspaceIdentifier,
		Hostname:     "localhost",
		User:         "brev",
		IdentityFile: privateKeyPath,
		Port:         port,
		Dir:          dir,
	}

	tmpl, err := template.New(string(workspaceIdentifier)).Parse(workspaceSSHConfigTemplate)
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

func ParseJetbrainsGatewayXML(config string) (*JetbrainsGatewayConfigXML, error) {
	var jetbrainsGatewayConfigXML JetbrainsGatewayConfigXML
	if err := xml.Unmarshal([]byte(config), &jetbrainsGatewayConfigXML); err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &jetbrainsGatewayConfigXML, nil
}

var (
	_ Reader = SSHConfig{}
	_ Writer = &SSHConfig{}
)

func NewSSHConfig(store SSHStore) (*SSHConfig, error) {
	configStr, err := store.GetUserSSHConfig()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	sshConfig, err := ssh_config.Decode(strings.NewReader(configStr))
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	privateKeyPath, err := store.GetPrivateKeyPath()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &SSHConfig{
		store:      store,
		sshConfig:  sshConfig,
		privateKey: privateKeyPath,
	}, nil
}

func (s *SSHConfig) PruneInactiveWorkspaces(identityPortMap IdentityPortMap) error {
	newConfig := ""
	var activeWorkspaces []entity.WorkspaceLocalID
	for key := range identityPortMap {
		activeWorkspaces = append(activeWorkspaces, key)
	}

	privateKeyPath, err := s.store.GetPrivateKeyPath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
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
func (s SSHConfig) GetBrevHostValues() []entity.WorkspaceLocalID {
	privateKeyPath := s.privateKey
	var brevHosts []entity.WorkspaceLocalID
	for _, host := range s.sshConfig.Hosts {
		hostname := workspaceIdentifierFromHost(host.String())
		// is this host a brev entry? if not, we don't care, and on to the
		// next one
		if checkIfBrevHost(*host, privateKeyPath) {
			brevHosts = append(brevHosts, hostname)
		}
	}
	return brevHosts
}

func (s *SSHConfig) Sync(identifierPortMapping IdentityPortMap) error {
	sshConfigStr := s.sshConfig.String()
	brevhosts := s.GetBrevHostValueSet()

	for key, value := range identifierPortMapping {
		if !brevhosts[key] {
			entry, err2 := MakeSSHEntry(key, value, s.privateKey, "/home/brev/workspace")
			if err2 != nil {
				return breverrors.WrapAndTrace(err2)
			}
			sshConfigStr += entry
		}
	}

	var err error
	s.sshConfig, err = ssh_config.Decode(strings.NewReader(sshConfigStr))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = s.PruneInactiveWorkspaces(identifierPortMapping)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = s.store.WriteUserSSHConfig(s.sshConfig.String())
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (s SSHConfig) GetBrevPorts() (BrevPorts, error) {
	portSet := make(BrevPorts)
	hostnames := s.GetBrevHostValues()
	for _, name := range hostnames {
		port, err := s.sshConfig.Get(string(name), "Port")
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

func (s SSHConfig) GetConfiguredWorkspacePort(workspaceIdentifier entity.WorkspaceLocalID) (string, error) {
	config, err := NewSSHConfig(s.store) // forces load from disk
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	port, err := config.sshConfig.Get(string(workspaceIdentifier), "Port")
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return port, nil
}

func (s SSHConfig) GetPrivateKeyFilePath() string {
	return s.privateKey
}

func NewSSHConfigurer(workspaces []entity.WorkspaceWithMeta, reader Reader, writers []Writer, store SSHConfigurerStore, privateKey string) *SSHConfigurer {
	return &SSHConfigurer{
		workspaces: workspaces,
		Reader:     reader,
		Writers:    writers,
		store:      store,
		privateKey: privateKey,
	}
}

func (sshConfigurer SSHConfigurer) GetPrivateKeyPath() (string, error) {
	return sshConfigurer.privateKey, nil
}

func (sshConfigurer *SSHConfigurer) GetIdentityPortMap() (IdentityPortMap, error) {
	activeWorkspaces := sshConfigurer.GetActiveWorkspaceIdentifiers()
	brevHostValuesSet := sshConfigurer.Reader.GetBrevHostValueSet()
	ports, err := sshConfigurer.Reader.GetBrevPorts()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	port := 2222
	identifierPortMapping := make(map[entity.WorkspaceLocalID]string)
	for _, workspaceIdentifier := range activeWorkspaces {
		if !brevHostValuesSet[workspaceIdentifier] {
			for ports[fmt.Sprint(port)] {
				port++
			}
			identifierPortMapping[workspaceIdentifier] = strconv.Itoa(port)
			ports[fmt.Sprint(port)] = true
		} else {
			if err != nil {
				return nil, breverrors.WrapAndTrace(err)
			}
			p, err := sshConfigurer.GetConfiguredWorkspacePort(workspaceIdentifier)
			if err != nil {
				return nil, breverrors.WrapAndTrace(err)
			}
			identifierPortMapping[workspaceIdentifier] = p
			ports[fmt.Sprint(port)] = true

		}
	}
	return identifierPortMapping, nil
}

func (sshConfigurer *SSHConfigurer) Sync() error {
	err := sshConfigurer.store.WritePrivateKey(sshConfigurer.privateKey)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	identityPortMap, err := sshConfigurer.GetIdentityPortMap()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	for _, writer := range sshConfigurer.Writers {
		err := writer.Sync(identityPortMap)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

	}

	return nil
}

func (sshConfigurer *SSHConfigurer) GetActiveWorkspaceIdentifiers() []entity.WorkspaceLocalID {
	var workspaceIdentifiers []entity.WorkspaceLocalID
	for _, workspace := range sshConfigurer.workspaces {
		fmt.Println(workspace.Name)
		workspaceIdentifiers = append(workspaceIdentifiers, workspace.GetLocalIdentifier())
	}
	return workspaceIdentifiers
}

func (sshConfigurer SSHConfigurer) GetConfiguredWorkspacePort(workspaceIdentifier entity.WorkspaceLocalID) (string, error) {
	port, err := sshConfigurer.Reader.GetConfiguredWorkspacePort(workspaceIdentifier)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return port, nil
}

var (
	_ Reader = &JetBrainsGatewayConfig{}
	_ Writer = &JetBrainsGatewayConfig{}
)

func NewJetBrainsGatewayConfig(store JetBrainsGatewayConfigStore) (*JetBrainsGatewayConfig, error) {
	config, err := store.GetJetBrainsConfig()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if config == "" {
		return &JetBrainsGatewayConfig{
			config: &JetbrainsGatewayConfigXML{},
			store:  store,
		}, nil
	}
	jetbrainsGatewayConfigXML, err := ParseJetbrainsGatewayXML(config)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &JetBrainsGatewayConfig{
		config: jetbrainsGatewayConfigXML,
		store:  store,
	}, nil
}

func (jbgc *JetBrainsGatewayConfig) Sync(identifierPortMapping IdentityPortMap) error {
	brevhosts := jbgc.GetBrevHostValueSet()
	activeWorkspaces := make(map[entity.WorkspaceLocalID]bool)
	privateKeyPath, err := jbgc.store.GetPrivateKeyPath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	for key, value := range identifierPortMapping {
		if !brevhosts[key] {
			jbgc.config.Component.Configs.SSHConfigs = append(jbgc.config.Component.Configs.SSHConfigs, JetbrainsGatewayConfigXMLSSHConfig{
				Host:       "localhost",
				Port:       value,
				KeyPath:    privateKeyPath,
				Username:   "brev",
				CustomName: (key),
				NameFormat: "CUSTOM",
				Options: []JetbrainsGatewayConfigXMLSSHOption{
					{
						Name:  "CustomName",
						Value: string(key),
					},
				},
			})
		}
		activeWorkspaces[(key)] = true
	}
	var sshConfigs []JetbrainsGatewayConfigXMLSSHConfig
	for _, conf := range jbgc.config.Component.Configs.SSHConfigs {
		isBrevHost := conf.KeyPath == privateKeyPath
		isActiveWorkspace := activeWorkspaces[conf.CustomName]
		if !isBrevHost {
			sshConfigs = append(sshConfigs, conf)
		} else if isBrevHost && isActiveWorkspace {
			sshConfigs = append(sshConfigs, conf)
		}
	}
	jbgc.config.Component.Configs.SSHConfigs = sshConfigs
	output, err := xml.MarshalIndent(jbgc.config, "", "  ")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = jbgc.store.WriteJetBrainsConfig(string(output))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (jbgc *JetBrainsGatewayConfig) GetBrevPorts() (BrevPorts, error) {
	ports := make(BrevPorts)
	for _, sshConf := range jbgc.config.Component.Configs.SSHConfigs {
		ports[sshConf.Port] = true
	}
	return ports, nil
}

func (jbgc *JetBrainsGatewayConfig) GetBrevHostValueSet() BrevHostValuesSet {
	brevHostValueSet := make(BrevHostValuesSet)
	for _, sshConf := range jbgc.config.Component.Configs.SSHConfigs {
		brevHostValueSet[sshConf.CustomName] = true
	}
	return brevHostValueSet
}

func (jbgc *JetBrainsGatewayConfig) GetConfiguredWorkspacePort(workspaceIdentifier entity.WorkspaceLocalID) (string, error) {
	// load config from disk
	config, err := jbgc.store.GetJetBrainsConfig()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	jetbrainsGatewayConfigXML, err := ParseJetbrainsGatewayXML(config)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	for _, sshConf := range jetbrainsGatewayConfigXML.Component.Configs.SSHConfigs {
		if sshConf.CustomName == workspaceIdentifier {
			return sshConf.Port, nil
		}
	}
	return "", nil
}


func WaitForSSHToBeAvailable(sshAlias string) error {
	counter := 0
	for {
		cmd := exec.Command("ssh", "-o", "ConnectTimeout=1", sshAlias, "echo", " ")
		out, err := cmd.CombinedOutput()
		// fmt.Println("golang err is: ", err)

		outputStr := string(out)
		// stringsOfInterest := [2]string{"Operation timed out", "Connection refused"}
		stdErr := strings.Split(outputStr, "\n")[1]
		// fmt.Println("stdErr is: ", stdErr)
		if err == nil {
			return nil
		}
		if counter == 10 {
			fmt.Println("\nPerforming final checks...")
		}
		if counter == 60 {
			return breverrors.WrapAndTrace(errors.New("\n" + stdErr))
		}
		counter++
		time.Sleep(1 * time.Second)
	}
}