package ssh

import (
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/errors"
	"github.com/hashicorp/go-multierror"
	cron "github.com/robfig/cron/v3"
	daemon "github.com/sevlyar/go-daemon"
)

type DummyStore struct{}

func (d DummyStore) GetWorkspaces() ([]entity.Workspace, error) {
	return []entity.Workspace{}, nil
}

type DummySSHConfigurerV2Store struct{}

func (d DummySSHConfigurerV2Store) OverrideWriteSSHConfig(config string) error {
	return nil
}

func RunTaskAsDaemon() error {
	cntxt := &daemon.Context{
		PidFileName: "sample.pid",
		PidFilePerm: 0644,
		LogFileName: "sample.log",
		LogFilePerm: 0640,
		WorkDir:     "./",
		Umask:       027,
		Args:        []string{},
	}

	d, err := cntxt.Reborn()
	if err != nil {
		return errors.WrapAndTrace(err)
	}
	if d != nil {
		return nil
	}
	defer cntxt.Release()

	log.Print("- - - - - - - - - - - - - - -")
	log.Print("daemon started")

	err = RunTasks()
	if err != nil {
		return errors.WrapAndTrace(err)
	}

	return nil
}

func RunTasks() error {
	c := ConfigUpdater{
		Store:   DummyStore{},
		Configs: []Config{NewSSHConfigurerV2(DummySSHConfigurerV2Store{})},
	}
	d := TaskRunner{Tasks: []Task{c}}

	err := d.Run()
	if err != nil {
		return errors.WrapAndTrace(err)
	}

	return nil
}

type Task interface {
	Run() error
	CronSpec() string // https://pkg.go.dev/github.com/robfig/cron?utm_source=godoc#hdr-CRON_Expression_Format
}

type TaskRunner struct {
	Tasks []Task
}

func LogErr(f func() error) func() {
	return func() {
		err := f()
		if err != nil {
			fmt.Println(err)
		}
	}
}

var signals = make(chan os.Signal, 1)

func (tr TaskRunner) Run() error {
	c := cron.New()
	for _, t := range tr.Tasks {
		_, err := c.AddFunc(t.CronSpec(), LogErr(t.Run))
		if err != nil {
			return errors.WrapAndTrace(err)
		}
	}

	c.Start()

	// signal.Notify(signals, os.Interrupt)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)
	<-signals
	fmt.Println("interrupt signal")
	<-c.Stop().Done()

	fmt.Println("done")
	return nil
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

var _ Task = ConfigUpdater{}

func (c ConfigUpdater) Run() error {
	workspaces, err := c.Store.GetWorkspaces()
	if err != nil {
		return errors.WrapAndTrace(err)
	}

	var res error
	for _, c := range c.Configs {
		err := c.Update(workspaces)
		if err != nil {
			res = multierror.Append(res, err)
		}
	}

	if res != nil {
		return errors.WrapAndTrace(res)
	}
	return nil
}

func (c ConfigUpdater) CronSpec() string {
	return "@every 3s"
}

// SSHConfigurerV2 speciallizes in configuring ssh config with ProxyCommand
type SSHConfigurerV2 struct {
	store SSHConfigurerV2Store
}

type SSHConfigurerV2Store interface {
	OverrideWriteSSHConfig(config string) error
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
		return errors.WrapAndTrace(err)
	}

	err = s.store.OverrideWriteSSHConfig(newConfig)
	if err != nil {
		return errors.WrapAndTrace(err)
	}

	err = s.EnsureConfigHasInclude()
	if err != nil {
		return errors.WrapAndTrace(err)
	}

	return nil
}

func (s SSHConfigurerV2) CreateNewSSHConfig(workspaces []entity.Workspace) (string, error) {
	fmt.Println("creating new ssh config")
	return "", nil
}

func (s SSHConfigurerV2) EnsureConfigHasInclude() error {
	// openssh-7.3
	fmt.Println("ensuring has include")
	return nil
}
