package tasks

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	cron "github.com/robfig/cron/v3"
	"github.com/sevlyar/go-daemon"
)

func RunTaskAsDaemon(tasks []Task, brevHome string) error {
	cntxt := &daemon.Context{
		PidFileName: fmt.Sprintf("%s/daemon.pid", brevHome),
		PidFilePerm: 0o644,
		LogFileName: fmt.Sprintf("%s/daemon.log", brevHome),
		LogFilePerm: 0o640,
		WorkDir:     brevHome,
		Umask:       0o27,
		Args:        []string{},
	}

	d, err := cntxt.Reborn()
	if err != nil {
		if errors.Is(err, daemon.ErrWouldBlock) {
			log.Print("daemon already running")
			return nil
		}
		return breverrors.WrapAndTrace(err)
	}
	if d != nil {
		return nil
	}

	log.Print("- - - - - - - - - - - - - - -")
	log.Print("daemon started")

	err = RunTasks(tasks)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = cntxt.Release()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func RunTasks(tasks []Task) error {
	d := TaskRunner{Tasks: tasks}

	err := d.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

type Task interface {
	Run() error
	GetTaskSpec() TaskSpec
}

type TaskSpec struct {
	Cron               string // can be "" if want to run once // https://pkg.go.dev/github.com/robfig/cron?utm_source=godoc#hdr-CRON_Expression_Format
	RunCronImmediately bool
}

type TaskRunner struct {
	Tasks       []Task
	StopSignals chan os.Signal
}

func NewTaskRunner(tasks []Task) *TaskRunner {
	return &TaskRunner{
		tasks,
		make(chan os.Signal, 1),
	}
}

func LogErr(f func() error) func() {
	return func() {
		err := f()
		if err != nil {
			log.Print(err)
		}
	}
}

func (tr TaskRunner) Run() error {
	c := cron.New()
	for _, t := range tr.Tasks {
		spec := t.GetTaskSpec()
		if spec.Cron != "" {
			e, err := c.AddFunc(spec.Cron, LogErr(t.Run))
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			if spec.RunCronImmediately {
				c.Entry(e).Job.Run()
			}
		} else {
			// we do this so that the context still applies
			e, err := c.AddFunc("@yearly", LogErr(t.Run))
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			c.Entry(e).Job.Run()
			c.Remove(e)
		}
	}

	c.Start()

	tr.WaitTillSignal(c.Stop)

	return nil
}

func (tr TaskRunner) WaitTillSignal(ctxfn func() context.Context) {
	signal.Notify(tr.StopSignals, syscall.SIGQUIT)
	signal.Notify(tr.StopSignals, syscall.SIGTERM)
	signal.Notify(tr.StopSignals, syscall.SIGHUP)
	signal.Notify(tr.StopSignals, syscall.SIGINT)

	defer signal.Stop(tr.StopSignals)
	<-tr.StopSignals
	<-ctxfn().Done()
}

func (tr *TaskRunner) SendStop() {
	tr.StopSignals <- syscall.SIGQUIT
}
