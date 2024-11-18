package ssh

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"log"
	"strings"
	"text/template"

	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
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

func NewConfigUpdater(store ConfigUpdaterStore, configs []Config, privateKey string) *ConfigUpdater {
	return &ConfigUpdater{
		Store:      store,
		Configs:    configs,
		PrivateKey: privateKey,
	}
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

// SSHConfigurerV2 speciallizes in configuring ssh config with ProxyCommand
type SSHConfigurerV2 struct {
	store SSHConfigurerV2Store
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
	GetWSLHostUserSSHConfigPath() (string, error)
	GetWindowsDir() (string, error)
	WriteBrevSSHConfigWSL(config string) error
	GetFileAsString(path string) (string, error)
	FileExists(path string) (bool, error)
	GetWSLHostBrevSSHConfigPath() (string, error)
	GetWSLUserSSHConfig() (string, error)
	WriteWSLUserSSHConfig(config string) error
	GetBrevCloudflaredBinaryPath() (string, error)
}

var _ Config = SSHConfigurerV2{}

func NewSSHConfigurerV2(store SSHConfigurerV2Store) *SSHConfigurerV2 {
	return &SSHConfigurerV2{
		store: store,
	}
}

func (s SSHConfigurerV2) Update(workspaces []entity.Workspace) error {
	newConfig, err := s.CreateNewSSHConfig(workspaces)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = s.store.WriteBrevSSHConfig(
		newConfig,
	)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = s.EnsureConfigHasInclude()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// try to write wsl config
	wslConfig, err := s.CreateWSLConfig(workspaces)
	if err != nil {
		return nil // not a fatal error // todo update sentry
	}
	err = s.store.WriteBrevSSHConfigWSL(wslConfig)
	if err != nil {
		return nil // not a fatal error // todo update sentry
	}

	// todo ensure has include
	err = s.EnsureWSLConfigHasInclude()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func (s SSHConfigurerV2) CreateWSLConfig(workspaces []entity.Workspace) (string, error) {
	configPath, err := s.store.GetWSLHostBrevSSHConfigPath()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	homedir, err := s.store.GetWindowsDir()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	pkpath := files.GetSSHPrivateKeyPath(homedir)

	cloudflaredBinaryPath, err := s.store.GetBrevCloudflaredBinaryPath()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	sshConfig, err := makeNewSSHConfig(toWindowsPath(configPath), workspaces, toWindowsPath(pkpath), toWindowsPath(cloudflaredBinaryPath))
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return sshConfig, nil
}

func (s SSHConfigurerV2) CreateNewSSHConfig(workspaces []entity.Workspace) (string, error) {
	configPath, err := s.store.GetUserSSHConfigPath()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	pkPath, err := s.store.GetPrivateKeyPath()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	cloudflaredBinaryPath, err := s.store.GetBrevCloudflaredBinaryPath()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	sshConfig, err := makeNewSSHConfig(configPath, workspaces, pkPath, cloudflaredBinaryPath)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return sshConfig, nil
}

func makeNewSSHConfig(configPath string, workspaces []entity.Workspace, pkpath string, cloudflaredBinaryPath string) (string, error) {
	sshConfig := fmt.Sprintf("# included in %s\n", configPath)
	for _, w := range workspaces {

		entry, err := makeSSHConfigEntryV2(w, pkpath, cloudflaredBinaryPath)
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
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
{{ if .RunRemoteCMD }}
  RemoteCommand cd {{ .Dir }}; $SHELL
{{ end }}
`

const SSHConfigEntryTemplateV3 = `Host {{ .Alias }}
  Hostname {{ .HostName }}
  IdentityFile {{ .IdentityFile }}
  User {{ .User }}
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  Port {{ .Port }}
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
	HostName     string
	Port         int
}

func tmplAndValToString(tmpl *template.Template, val interface{}) (string, error) {
	buf := &bytes.Buffer{}
	err := tmpl.Execute(buf, val)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return buf.String(), nil
}

func makeSSHConfigEntryV2(workspace entity.Workspace, privateKeyPath string, cloudflaredBinaryPath string) (string, error) { //nolint:funlen // ok
	alias := string(workspace.GetLocalIdentifier())
	privateKeyPath = "\"" + privateKeyPath + "\""
	if workspace.IsLegacy() {
		proxyCommand := makeProxyCommand(workspace.ID)
		entry := SSHConfigEntryV2{
			Alias:        alias,
			IdentityFile: privateKeyPath,
			User:         "brev",
			ProxyCommand: proxyCommand,
			Dir:          workspace.GetProjectFolderPath(),
		}
		tmpl, err := template.New(alias).Parse(SSHConfigEntryTemplateV2)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		val, err := tmplAndValToString(tmpl, entry)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		return val, nil
	}

	var sshVal string
	user := workspace.GetSSHUser()
	hostname := workspace.GetHostname()
	if workspace.SSHProxyHostname == "" {
		port := workspace.GetSSHPort()
		entry := SSHConfigEntryV2{
			Alias:        alias,
			IdentityFile: privateKeyPath,
			User:         user,
			Dir:          workspace.GetProjectFolderPath(),
			HostName:     hostname,
			Port:         port,
		}
		tmpl, err := template.New(alias).Parse(SSHConfigEntryTemplateV3)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		sshVal, err = tmplAndValToString(tmpl, entry)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
	} else {
		proxyCommand := makeCloudflareSSHProxyCommand(cloudflaredBinaryPath, workspace.SSHProxyHostname)
		entry := SSHConfigEntryV2{
			Alias:        alias,
			IdentityFile: privateKeyPath,
			User:         user,
			ProxyCommand: proxyCommand,
			Dir:          workspace.GetProjectFolderPath(),
		}
		tmpl, err := template.New(alias).Parse(SSHConfigEntryTemplateV2)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		sshVal, err = tmplAndValToString(tmpl, entry)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
	}

	alias = fmt.Sprintf("%s-host", alias)
	var hostSSHVal string
	hostport := workspace.GetHostSSHPort()
	hostuser := workspace.GetHostSSHUser()
	if workspace.HostSSHProxyHostname == "" {
		entry := SSHConfigEntryV2{
			Alias:        alias,
			IdentityFile: privateKeyPath,
			User:         hostuser,
			Dir:          workspace.GetProjectFolderPath(),
			HostName:     hostname,
			Port:         hostport,
		}
		tmpl, err := template.New(alias).Parse(SSHConfigEntryTemplateV3)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		hostSSHVal, err = tmplAndValToString(tmpl, entry)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
	} else {
		proxyCommand := makeCloudflareSSHProxyCommand(cloudflaredBinaryPath, workspace.HostSSHProxyHostname)
		entry := SSHConfigEntryV2{
			Alias:        alias,
			IdentityFile: privateKeyPath,
			User:         hostuser,
			ProxyCommand: proxyCommand,
			Dir:          workspace.GetProjectFolderPath(),
		}
		tmpl, err := template.New(alias).Parse(SSHConfigEntryTemplateV2)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		hostSSHVal, err = tmplAndValToString(tmpl, entry)
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
	}

	val := fmt.Sprintf("%s%s", sshVal, hostSSHVal)
	return val, nil
}

// func makeSSHConfigEntryV2(workspace entity.Workspace, privateKeyPath string) (string, error) {
// 	alias := string(workspace.GetLocalIdentifier())
// 	privateKeyPath = "\"" + privateKeyPath + "\""
// 	if workspace.IsLegacy() {
// 		proxyCommand := makeProxyCommand(workspace.ID)
// 		entry := SSHConfigEntryV2{
// 			Alias:        alias,
// 			IdentityFile: privateKeyPath,
// 			User:         "brev", // todo param-user
// 			ProxyCommand: proxyCommand,
// 			Dir:          workspace.GetProjectFolderPath(),
// 		}
// 		tmpl, err := template.New(alias).Parse(SSHConfigEntryTemplateV2)
// 		if err != nil {
// 			return "", breverrors.WrapAndTrace(err)
// 		}

// 		buf := &bytes.Buffer{}
// 		err = tmpl.Execute(buf, entry)
// 		if err != nil {
// 			return "", breverrors.WrapAndTrace(err)
// 		}

// 		return buf.String(), nil
// 	} else {
// 		hostname := workspace.GetHostname()
// 		var userName string
// 		port := workspace.GetSSHPort()
// 		if port == 22 {
// 			userName = "ubuntu"
// 		} else {
// 			userName = "root"
// 		}
// 		entry := SSHConfigEntryV2{
// 			Alias:        alias,
// 			IdentityFile: privateKeyPath,
// 			User:         userName, // todo param-user
// 			Dir:          workspace.GetProjectFolderPath(),
// 			HostName:     hostname,
// 			Port:         port,
// 		}
// 		tmpl, err := template.New(alias).Parse(SSHConfigEntryTemplateV3)
// 		if err != nil {
// 			return "", breverrors.WrapAndTrace(err)
// 		}
// 		buf := &bytes.Buffer{}
// 		err = tmpl.Execute(buf, entry)
// 		if err != nil {
// 			return "", breverrors.WrapAndTrace(err)
// 		}

// 		return buf.String(), nil
// 	}
// }

func makeCloudflareSSHProxyCommand(cloudflaredBinaryPath string, hostname string) string {
	return fmt.Sprintf("%s access ssh --hostname %s", cloudflaredBinaryPath, hostname)
}

func makeProxyCommand(workspaceID string) string {
	huproxyExec := "brev proxy"
	return fmt.Sprintf("%s %s", huproxyExec, workspaceID)
}

func (s SSHConfigurerV2) EnsureWSLConfigHasInclude() error {
	// openssh-7.3

	brevConfigPath, err := s.store.GetWSLHostBrevSSHConfigPath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	conf, err := s.store.GetWSLUserSSHConfig()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !doesUserSSHConfigIncludeBrevConfig(conf, brevConfigPath) {
		newConf := WSLAddIncludeToUserConfig(conf, brevConfigPath)
		err := s.store.WriteWSLUserSSHConfig(newConf)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	return nil
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

func toWindowsPath(path string) string {
	path = strings.ReplaceAll(path, "/mnt/c", "C:")
	path = strings.ReplaceAll(path, "/", "\\")
	return path
}

func WSLAddIncludeToUserConfig(conf string, brevConfigPath string) string {
	return makeIncludeBrevStr(toWindowsPath(brevConfigPath)) + conf
}

func makeIncludeBrevStr(brevSSHConfigPath string) string {
	return fmt.Sprintf("Include \"%s\"\n", brevSSHConfigPath)
}

func doesUserSSHConfigIncludeBrevConfig(conf string, brevConfigPath string) bool {
	return strings.Contains(conf, makeIncludeBrevStr(brevConfigPath))
}

// Deprecated: var _ Config = SSHConfigurerServiceMesh{}

// openssh-7.3

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

type SSHConfigurerJetBrains struct {
	store SSHConfigurerV2Store
}

var _ Config = SSHConfigurerJetBrains{}

func NewSSHConfigurerJetBrains(store SSHConfigurerV2Store) (*SSHConfigurerJetBrains, error) {
	return &SSHConfigurerJetBrains{
		store: store,
	}, nil
}

func (s SSHConfigurerJetBrains) Update(workspaces []entity.Workspace) error {
	doesJbPathExist, err := s.store.DoesJetbrainsFilePathExist()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !doesJbPathExist {
		return nil
	}

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
		pk, err := s.store.GetPrivateKeyPath()
		if err != nil {
			return "", breverrors.WrapAndTrace(err)
		}
		entry := makeJetbrainsConfigEntry(w, pk)
		config.Component.Configs.SSHConfigs = append(config.Component.Configs.SSHConfigs, entry)
	}
	output, err := xml.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	return string(output), nil
}

// <application>
//
//	  <component name="SshConfigs">
//	    <configs>
//	      <sshConfig host="hn"
//					 id="ee9d8d15-d6cb-43ae-a96b-250caa33c407"
//					 keyPath="$USER_HOME$/.brev/brev.pem"
//					 port="22"
//					 customName="brev@hn"
//					 nameFormat="CUSTOM"
//					 username="brev"
//					 useOpenSSHConfig="true">
//	        <option name="customName" value="brev@hn" />
//	      </sshConfig>
//	    </configs>
//	  </component>
//
// </application>
func makeJetbrainsConfigEntry(workspace entity.Workspace, privateKeyPath string) JetbrainsGatewayConfigXMLSSHConfig {
	hostname := workspace.GetHostname()
	port := workspace.GetSSHPort()
	// name := workspace.GetLocalIdentifier()
	return JetbrainsGatewayConfigXMLSSHConfig{
		Host:     hostname,
		Port:     fmt.Sprint(port),
		KeyPath:  privateKeyPath,
		Username: workspace.GetUsername(),
		// CustomName:       name,
		NameFormat:       "DESCRIPTIVE",
		ConnectionConfig: `{"hostKeyVerifier":{"stringHostKeyChecking":"NO"},"serverAliveInterval":30}`,
		UseOpenSSHConfig: "false",
		// Options: []JetbrainsGatewayConfigXMLSSHOption{
		// 	{
		// 		Name:  "CustomName",
		// 		Value: string(name),
		// 	},
		// },
	}
}
