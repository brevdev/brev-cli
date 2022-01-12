package ssh

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"text/template"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/tasks"
	"github.com/hashicorp/go-multierror"
)

type DummyStore struct{}

func (d DummyStore) GetWorkspaces() ([]entity.Workspace, error) {
	return []entity.Workspace{}, nil
}

type DummySSHConfigurerV2Store struct{}

func (d DummySSHConfigurerV2Store) OverrideWriteSSHConfig(_ string) error {
	return nil
}

func (d DummySSHConfigurerV2Store) WriteBrevSSHConfig(_ string) error {
	return nil
}

func (d DummySSHConfigurerV2Store) ReadUserSSHConfig() (string, error) {
	return "", nil
}

func (d DummySSHConfigurerV2Store) WriteUserSSHConfig(_ string) error {
	return nil
}

func (d DummySSHConfigurerV2Store) GetPrivateKeyPath() string {
	return "/my/priv/key.pem"
}

func (d DummySSHConfigurerV2Store) GetUserSSHConfigPath() string {
	return "/my/user/config"
}

func (d DummySSHConfigurerV2Store) GetBrevSSHConfigPath() string {
	return "/my/brev/config"
}

type ConfigUpdaterStore interface {
	GetWorkspaces() ([]entity.Workspace, error)
}

type Config interface {
	Update(workspaces []entity.Workspace) error
}

type ConfigUpdater struct {
	Store   ConfigUpdaterStore
	Configs []Config
}

var _ tasks.Task = ConfigUpdater{}

func (c ConfigUpdater) Run() error {
	workspaces, err := c.Store.GetWorkspaces()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	var res error
	for _, c := range c.Configs {
		err := c.Update(workspaces)
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

// SSHConfigurerV2 speciallizes in configuring ssh config with ProxyCommand
type SSHConfigurerV2 struct {
	store SSHConfigurerV2Store
}

type SSHConfigurerV2Store interface {
	WriteBrevSSHConfig(config string) error
	ReadUserSSHConfig() (string, error)
	WriteUserSSHConfig(config string) error
	GetPrivateKeyPath() string
	GetUserSSHConfigPath() string
	GetBrevSSHConfigPath() string
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
	log.Print("creating new ssh config")

	sshConfig := fmt.Sprintf("# included in %s\n", s.store.GetUserSSHConfigPath())
	for _, w := range workspaces {
		// need to make getlocalidentifier conformal
		entry, err := makeSSHConfigEntry(string(w.GetLocalIdentifier(nil)), w.GetSSHURL(), s.store.GetPrivateKeyPath())
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

`

type SSHConfigEntryV2 struct {
	Alias        string
	IdentityFile string
	User         string
	ProxyCommand string
}

func makeSSHConfigEntry(alias string, url string, privateKeyPath string) (string, error) {
	proxyCommand := makeProxyCommand(url)
	entry := SSHConfigEntryV2{
		Alias:        alias,
		IdentityFile: privateKeyPath,
		User:         "brev",
		ProxyCommand: proxyCommand,
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

func makeProxyCommand(url string) string {
	huproxyExec := "huproxyclient"
	return fmt.Sprintf("%s wss://%s/proxy/localhost/22", huproxyExec, url)
}

func (s SSHConfigurerV2) EnsureConfigHasInclude() error {
	// openssh-7.3
	log.Print("ensuring has include")

	conf, err := s.store.ReadUserSSHConfig()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !s.doesUserSSHConfigIncludeBrevConfig(conf) {
		newConf := s.AddIncludeToUserConfig(conf)
		err := s.store.WriteUserSSHConfig(newConf)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	return nil
}

func (s SSHConfigurerV2) AddIncludeToUserConfig(conf string) string {
	newConf := makeIncludeBrevStr(s.store.GetBrevSSHConfigPath()) + conf

	return newConf
}

func makeIncludeBrevStr(brevSSHConfigPath string) string {
	return fmt.Sprintf("Include %s\n", brevSSHConfigPath)
}

func (s SSHConfigurerV2) doesUserSSHConfigIncludeBrevConfig(conf string) bool {
	return strings.Contains(conf, makeIncludeBrevStr(s.store.GetBrevSSHConfigPath()))
}
