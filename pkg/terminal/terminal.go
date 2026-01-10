// Package terminal is for terminal outputting
package terminal

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

var ProgressBarMax = 100

type ProgressBar struct {
	Bar            *progressbar.ProgressBar
	CurrPercentage int
}

type Terminal struct {
	out     io.Writer
	verbose io.Writer
	err     io.Writer

	Green  func(format string, a ...interface{}) string
	Yellow func(format string, a ...interface{}) string
	Red    func(format string, a ...interface{}) string
	Blue   func(format string, a ...interface{}) string
	White  func(format string, a ...interface{}) string

	Bar ProgressBar

	Spinner *spinner.Spinner
}

func New() (t *Terminal) {
	return &Terminal{
		out:     os.Stdout,
		verbose: os.Stdout,
		err:     os.Stderr,
		Green:   color.New(color.FgGreen).SprintfFunc(),
		Yellow:  color.New(color.FgYellow).SprintfFunc(),
		Red:     color.New(color.FgRed).SprintfFunc(),
		Blue:    color.New(color.FgBlue).SprintfFunc(),
		White:   color.New(color.FgWhite, color.Bold).SprintfFunc(),
	}
}

func (t *Terminal) Print(a string) {
	_, _ = fmt.Fprintln(t.out, a)
}

func (t *Terminal) Vprint(a string) {
	_, _ = fmt.Fprintln(t.verbose, a)
}

func (t *Terminal) Vprintf(format string, a ...interface{}) {
	_, _ = fmt.Fprintf(t.verbose, format, a...)
}

func (t *Terminal) Eprint(a string) {
	_, _ = fmt.Fprintln(t.err, a)
}

func (t *Terminal) Eprintf(format string, a ...interface{}) {
	_, _ = fmt.Fprintf(t.err, format, a...)
}

func (t *Terminal) Errprint(err error, a string) {
	t.Eprint(t.Red("Error: " + err.Error()))
	if a != "" {
		t.Eprint(t.Red(a))
	}
	if brevErr, ok := err.(breverrors.BrevError); ok {
		t.Eprint(t.Red(brevErr.Directive()))
	}
}

func (t *Terminal) NewSpinner() *spinner.Spinner {
	spinner := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithWriter(os.Stderr))
	err := spinner.Color("cyan", "bold")
	if err != nil {
		t.Errprint(err, "")
	}
	spinner.Reverse()

	return spinner
}
