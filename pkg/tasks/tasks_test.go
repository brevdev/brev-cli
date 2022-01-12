package tasks

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type DummyTask struct {
	Ran      int
	TaskSpec TaskSpec
}

func (d *DummyTask) Run() error {
	d.Ran++
	return nil
}

func (d DummyTask) GetTaskSpec() TaskSpec {
	return d.TaskSpec
}

func TestRunImmediateAndCron(t *testing.T) {
	dt := DummyTask{TaskSpec: TaskSpec{
		RunCronImmediately: true,
		Cron:               "@every 1s",
	}}
	tr := NewTaskRunner([]Task{&dt})
	go func() {
		time.Sleep(time.Millisecond * 1500)
		tr.SendStop()
	}()
	err := tr.Run()
	assert.Nil(t, err)
	assert.Equal(t, 2, dt.Ran)
}

func TestRunImmediate(t *testing.T) {
	dt := DummyTask{TaskSpec: TaskSpec{
		RunCronImmediately: true,
		Cron:               "",
	}}
	tr := NewTaskRunner([]Task{&dt})
	go func() {
		time.Sleep(time.Millisecond * 50)
		tr.SendStop()
	}()
	err := tr.Run()
	assert.Nil(t, err)
	assert.Equal(t, 1, dt.Ran)
}

func TestNotRunImmediateAndCron(t *testing.T) {
	dt := DummyTask{TaskSpec: TaskSpec{
		RunCronImmediately: false,
		Cron:               "@every 1s",
	}}
	tr := NewTaskRunner([]Task{&dt})
	go func() {
		time.Sleep(time.Millisecond * 1500)
		tr.SendStop()
	}()
	err := tr.Run()
	assert.Nil(t, err)
	assert.Equal(t, 1, dt.Ran)
}
