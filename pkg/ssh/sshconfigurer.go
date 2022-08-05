package ssh

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"strings"
	"text/template"

	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/tasks"
	"github.com/hashicorp/go-multierror"
)

type ConfigUpdaterStore interface {
	autostartconf.AutoStartStore
	GetContextWorkspaces() ([]entity.Workspace, error)
	WritePrivateKey(pem string) error
}

type Config interface {
	Update(workspaces []entity.Workspace) error
}

type ConfigUpdater struct {
	Store      ConfigUpdaterStore
	Configs    []Config
	PrivateKey string
}

var _ tasks.Task = ConfigUpdater{}

func (c ConfigUpdater) Run() error {
	err := c.Store.WritePrivateKey(c.PrivateKey)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	workspaces, err := c.Store.GetContextWorkspaces()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	var runningWorkspaces []entity.Workspace
	for _, workspace := range workspaces {
		if workspace.Status == entity.Running {
			runningWorkspaces = append(runningWorkspaces, workspace)
		}
	}

	var res error
	for _, c := range c.Configs {
		err := c.Update(runningWorkspaces)
		if err != nil {
			res = multierror.Append(res, err)
		}
	}

	if res != nil {
		return breverrors.WrapAndTrace(res)
	}
	return nil
}

func (c ConfigUpdater) GetTaskSpec() tasks.TaskSpec {
	return tasks.TaskSpec{RunCronImmediately: true, Cron: "@every 3s"}
}

func (c ConfigUpdater) Configure() error {
	return nil
}

// ConfigUpaterFactoryStore prob a better way to do this
type ConfigUpaterFactoryStore interface {
	ConfigUpdaterStore
	SSHConfigurerV2Store
}

func NewSSHConfigUpdater(store ConfigUpaterFactoryStore) ConfigUpdater {
	return ConfigUpdater{
		Store: store,
		Configs: []Config{
			NewSSHConfigurerV2(
				store,
				true,
			),
		},
	}
}

// SSHConfigurerV2 speciallizes in configuring ssh config with ProxyCommand
type SSHConfigurerV2 struct {
	store        SSHConfigurerV2Store
	runRemoteCMD bool
}

type SSHConfigurerV2Store interface {
	WriteBrevSSHConfig(config string) error
	GetUserSSHConfig() (string, error)
	WriteUserSSHConfig(config string) error
	GetPrivateKeyPath() (string, error)
	GetUserSSHConfigPath() (string, error)
	GetBrevSSHConfigPath() (string, error)
	GetJetBrainsConfigPath() (string, error)
	GetJetBrainsConfig() (string, error)
	WriteJetBrainsConfig(config string) error
	DoesJetbrainsFilePathExist() (bool, error)
}

var _ Config = SSHConfigurerV2{}

func NewSSHConfigurerV2(store SSHConfigurerV2Store, runRemoteCMD bool) *SSHConfigurerV2 {
	return &SSHConfigurerV2{
		store:        store,
		runRemoteCMD: runRemoteCMD,
	}
}

func (s SSHConfigurerV2) Update(workspaces []entity.Workspace) error {
	newConfig, err := s.CreateNewSSHConfig(workspaces)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = s.store.WriteBrevSSHConfig(newConfig)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = s.EnsureConfigHasInclude()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func (s SSHConfigurerV2) CreateNewSSHConfig(workspaces []entity.Workspace) (string, error) {
	configPath, err := s.store.GetUserSSHConfigPath()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	sshConfig := fmt.Sprintf("# included in %s\n", configPath)
	for _, w := range workspaces {
		pk, err := s.store.GetPrivateKeyPath()
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		entry, err := makeSSHConfigEntryV2(string(w.GetLocalIdentifier()), w.ID, pk, w.GetProjectFolderPath(), s.runRemoteCMD)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}

		sshConfig += entry
	}

	return sshConfig, nil
}

const SSHConfigEntryTemplateV2 = `Host {{ .Alias }}
  IdentityFile {{ .IdentityFile }}
  User {{ .User }}
  ProxyCommand {{ .ProxyCommand }}
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  StrictHostKeyChecking no
  PasswordAuthentication no
  RequestTTY yes
{{ if .RunRemoteCMD }}
  RemoteCommand cd {{ .Dir }}; $SHELL
{{ end }}
`

type SSHConfigEntryV2 struct {
	Alias        string
	IdentityFile string
	User         string
	ProxyCommand string
	Dir          string
	RunRemoteCMD bool
}

func makeSSHConfigEntryV2(alias string, workspaceID string, privateKeyPath string, dir string, runRemoteCMD bool) (string, error) {
	proxyCommand := makeProxyCommand(workspaceID)
	entry := SSHConfigEntryV2{
		Alias:        alias,
		IdentityFile: privateKeyPath,
		User:         "brev",
		ProxyCommand: proxyCommand,
		Dir:          dir,
		RunRemoteCMD: runRemoteCMD,
	}

	tmpl, err := template.New(alias).Parse(SSHConfigEntryTemplateV2)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, entry)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	return buf.String(), nil
}

func makeProxyCommand(workspaceID string) string {
	huproxyExec := "brev proxy"
	return fmt.Sprintf("%s %s", huproxyExec, workspaceID)
}

func (s SSHConfigurerV2) EnsureConfigHasInclude() error {
	// openssh-7.3

	brevConfigPath, err := s.store.GetBrevSSHConfigPath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	conf, err := s.store.GetUserSSHConfig()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !doesUserSSHConfigIncludeBrevConfig(conf, brevConfigPath) {
		newConf, err := AddIncludeToUserConfig(conf, brevConfigPath)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = s.store.WriteUserSSHConfig(newConf)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	return nil
}

func AddIncludeToUserConfig(conf string, brevConfigPath string) (string, error) {
	newConf := makeIncludeBrevStr(brevConfigPath) + conf
	return newConf, nil
}

func makeIncludeBrevStr(brevSSHConfigPath string) string {
	return fmt.Sprintf("Include %s\n", brevSSHConfigPath)
}

func doesUserSSHConfigIncludeBrevConfig(conf string, brevConfigPath string) bool {
	return strings.Contains(conf, makeIncludeBrevStr(brevConfigPath))
}

type SSHConfigurerServiceMesh struct {
	store SSHConfigurerV2Store
}

var _ Config = SSHConfigurerServiceMesh{}

func NewSSHConfigurerServiceMesh(store SSHConfigurerV2Store) *SSHConfigurerServiceMesh {
	return &SSHConfigurerServiceMesh{
		store: store,
	}
}

func (s SSHConfigurerServiceMesh) Update(workspaces []entity.Workspace) error {
	newConfig, err := s.CreateNewSSHConfig(workspaces)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = s.store.WriteBrevSSHConfig(newConfig)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = s.EnsureConfigHasInclude()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func (s SSHConfigurerServiceMesh) EnsureConfigHasInclude() error {
	// openssh-7.3

	brevConfigPath, err := s.store.GetBrevSSHConfigPath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	conf, err := s.store.GetUserSSHConfig()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !doesUserSSHConfigIncludeBrevConfig(conf, brevConfigPath) {
		newConf, err := AddIncludeToUserConfig(conf, brevConfigPath)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		err = s.store.WriteUserSSHConfig(newConf)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	return nil
}

func (s SSHConfigurerServiceMesh) CreateNewSSHConfig(workspaces []entity.Workspace) (string, error) {
	log.Print("creating new service mesh ssh config")

	configPath, err := s.store.GetUserSSHConfigPath()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	sshConfig := fmt.Sprintf("# included in %s\n", configPath)
	for _, w := range workspaces {
		pk, err := s.store.GetPrivateKeyPath()
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		entry, err := makeSSHConfigServiceMeshEntry(string(w.GetLocalIdentifier()), w.GetNodeIdentifierForVPN(), pk)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}

		sshConfig += entry
	}

	return sshConfig, nil
}

const SSHConfigEntryTemplateServiceMesh = `Host {{ .Alias }}
  HostName {{ .Host }}
  IdentityFile {{ .IdentityFile }}
  User {{ .User }}
  Port {{ .Port }}
  ServerAliveInterval 30

`

type SSHConfigEntryServiceMesh struct {
	Alias        string
	Host         string
	IdentityFile string
	User         string
	Port         string
}

func makeSSHConfigServiceMeshEntry(alias string, host string, privateKeyPath string) (string, error) {
	entry := SSHConfigEntryServiceMesh{
		Alias:        alias,
		Host:         host,
		IdentityFile: privateKeyPath,
		User:         "brev",
		Port:         "22",
	}

	tmpl, err := template.New(host).Parse(SSHConfigEntryTemplateServiceMesh)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, entry)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	return buf.String(), nil
}

type SSHConfigurerJetBrains struct {
	store SSHConfigurerV2Store
}

var _ Config = SSHConfigurerJetBrains{}

func NewSSHConfigurerJetBrains(store SSHConfigurerV2Store) (*SSHConfigurerJetBrains, error) {
	doesJbPathExist, err := store.DoesJetbrainsFilePathExist()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if !doesJbPathExist {
		return nil, errors.New("jetgbrains dne")
	}

	return &SSHConfigurerJetBrains{
		store: store,
	}, nil
}

func (s SSHConfigurerJetBrains) Update(workspaces []entity.Workspace) error {
	newConfig, err := s.CreateNewSSHConfig(workspaces)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = s.store.WriteJetBrainsConfig(newConfig)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func (s SSHConfigurerJetBrains) CreateNewSSHConfig(workspaces []entity.Workspace) (string, error) {
	log.Print("creating new ssh config")

	config := &JetbrainsGatewayConfigXML{
		Component: JetbrainsGatewayConfigXMLComponent{
			Name: "SshConfigs",
		},
	}
	for _, w := range workspaces {
		pk, errother := s.store.GetPrivateKeyPath()
		if errother != nil {
			return "", breverrors.WrapAndTrace(errother)
		}
		nodeID := w.GetNodeIdentifierForVPN()
		entry := makeJetbrainsConfigEntry(nodeID, pk)
		config.Component.Configs.SSHConfigs = append(config.Component.Configs.SSHConfigs, entry)
	}
	output, err := xml.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return string(output), nil
}

// <application>
//   <component name="SshConfigs">
//     <configs>
//       <sshConfig host="hn"
// 				 id="ee9d8d15-d6cb-43ae-a96b-250caa33c407"
// 				 keyPath="$USER_HOME$/.brev/brev.pem"
// 				 port="22"
// 				 customName="brev@hn"
// 				 nameFormat="CUSTOM"
// 				 username="brev"
// 				 useOpenSSHConfig="true">
//         <option name="customName" value="brev@hn" />
//       </sshConfig>
//     </configs>
//   </component>
// </application>
func makeJetbrainsConfigEntry(host, keypath string) JetbrainsGatewayConfigXMLSSHConfig {
	return JetbrainsGatewayConfigXMLSSHConfig{
		Host:       host,
		Port:       "22",
		KeyPath:    keypath,
		Username:   "brev",
		CustomName: entity.WorkspaceLocalID(host),
		NameFormat: "CUSTOM",
		Options: []JetbrainsGatewayConfigXMLSSHOption{
			{
				Name:  "CustomName",
				Value: host,
			},
		},
	}
}
