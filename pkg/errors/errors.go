package errors

import (
	"fmt"
	"runtime"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/version"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/getsentry/sentry-go"
	"github.com/pkg/errors"
)

type BrevError interface {
	// Error returns a user-facing string explaining the error
	Error() string

	// Directive returns a user-facing string explaining how to overcome the error
	Directive() string
}

type ErrorReporter interface {
	Setup() func()
	Flush()
	ReportMessage(string) string
	ReportError(error) string
	AddTag(key string, value string)
}

func GetDefaultErrorReporter() ErrorReporter {
	return SentryErrorReporter{}
}

type SentryErrorReporter struct{}

var _ ErrorReporter = SentryErrorReporter{}

func (s SentryErrorReporter) Setup() func() {
	if !featureflag.IsDev() {
		err := sentry.Init(sentry.ClientOptions{
			Dsn:     config.GlobalConfig.GetSentryURL(),
			Release: version.Version,
		})
		if err != nil {
			fmt.Println(err)
		}
	}
	return func() {
		err := recover()
		if err != nil {
			sentry.CurrentHub().Recover(err)
			sentry.Flush(time.Second * 5)
			panic(err)
		}
		sentry.Flush(2 * time.Second)
	}
}

func (s SentryErrorReporter) Flush() {
	sentry.Flush(time.Second * 2)
}

func (s SentryErrorReporter) ReportMessage(msg string) string {
	event := sentry.CaptureMessage(msg)
	if event != nil {
		return string(*event)
	} else {
		return ""
	}
}

func (s SentryErrorReporter) ReportError(e error) string {
	event := sentry.CaptureException(e)
	if event != nil {
		return string(*event)
	} else {
		return ""
	}
}

func (s SentryErrorReporter) AddTag(key string, value string) {
	scope := sentry.CurrentHub().Scope()
	scope.SetTag(key, value)
}

type ValidationError struct {
	Message string
}

func NewValidationError(message string) ValidationError {
	return ValidationError{Message: message}
}

var _ error = ValidationError{}

func (v ValidationError) Error() string {
	return v.Message
}

type DeclineToLoginError struct{}

func (d *DeclineToLoginError) Error() string     { return "declined to login" }
func (d *DeclineToLoginError) Directive() string { return "log in to run this command" }

func WrapAndTrace(err error, messages ...string) error {
	message := ""
	for _, m := range messages {
		message += fmt.Sprintf(" %s", m)
	}
	return errors.Wrap(err, MakeErrorMessage(message))
}

func MakeErrorMessage(message string) string {
	_, fn, line, _ := runtime.Caller(2)
	return fmt.Sprintf("[error] %s:%d %s\n\t", fn, line, message)
}

var NetworkErrorMessage = "possible internet connection problem"

type CredentialsFileNotFound struct{}

func (e *CredentialsFileNotFound) Directive() string {
	return "run `brev login`"
}

func (e *CredentialsFileNotFound) Error() string {
	return "credentials file not found"
}
