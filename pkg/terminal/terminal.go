// Package terminal is for terminal outputting
package terminal

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"

	"github.com/brevdev/brev-cli/pkg/brev_errors"
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

	Bar ProgressBar
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
	}
}

func (t *Terminal) SetVerbose(verbose bool) {
	if verbose {
		t.out = os.Stdout
	} else {
		t.out = silentWriter{}
	}
}

func (t *Terminal) Print(a string) {
	fmt.Fprintln(t.out, a)
}

func (t *Terminal) Printf(format string, a ...interface{}) {
	fmt.Fprintf(t.out, format, a...)
}

func (t *Terminal) Vprint(a string) {
	fmt.Fprintln(t.verbose, a)
}

func (t *Terminal) Vprintf(format string, a ...interface{}) {
	fmt.Fprintf(t.verbose, format, a...)
}

func (t *Terminal) Eprint(a string) {
	fmt.Fprintln(t.err, a)
}

func (t *Terminal) Eprintf(format string, a ...interface{}) {
	fmt.Fprintf(t.err, format, a...)
}

func (t *Terminal) Errprint(err error, a string) {
	t.Eprint(t.Red("Error: " + err.Error()))
	if a != "" {
		t.Eprint(t.Red(a))
	}
	if brevErr, ok := err.(brev_errors.BrevError); ok {
		t.Eprint(t.Red(brevErr.Directive()))
	}
}

func (t *Terminal) Errprintf(err error, format string, a ...interface{}) {
	t.Eprint(t.Red("Error: " + err.Error()))
	if a != nil {
		t.Eprint(t.Red(format, a))
	}
	if brevErr, ok := err.(brev_errors.BrevError); ok {
		t.Eprint(t.Red(brevErr.Directive()))
	}
}

type silentWriter struct{}

func (w silentWriter) Write(_ []byte) (n int, err error) {
	return 0, nil
}

func (t *Terminal) NewProgressBar(description string, onComplete func()) *ProgressBar {
	bar := progressbar.NewOptions(ProgressBarMax,
		progressbar.OptionOnCompletion(onComplete),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(false),
		progressbar.OptionSetWidth(15),
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]🤙[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}))

	return &ProgressBar{
		Bar:            bar,
		CurrPercentage: 0,
	}
}

func (bar *ProgressBar) AdvanceTo(percentage int) {
	for bar.CurrPercentage < percentage && bar.CurrPercentage <= 100 {
		bar.CurrPercentage++
		err := bar.Bar.Add(1)
		if err != nil {
			panic(err)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func (bar *ProgressBar) Describe(text string) {
	bar.Bar.Describe(text)
}
