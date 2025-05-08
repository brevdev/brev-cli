package errors

import (
	"fmt"
	"runtime"
	"strconv"
	"time"

	stderrors "errors"

	"github.com/getsentry/sentry-go"
	pkgerrors "github.com/pkg/errors"
)

type BrevError interface {
	// Error returns a user-facing string explaining the error
	Error() string

	// Directive returns a user-facing string explaining how to overcome the error
	Directive() string
}

type ErrorUser struct {
	ID       string
	Username string
	Email    string
}

type ErrorReporter interface {
	Setup() func()
	Flush()
	ReportMessage(string) string
	ReportError(error) string
	AddTag(key string, value string)
	SetUser(user ErrorUser)
	AddBreadCrumb(bc ErrReportBreadCrumb)
}

func GetDefaultErrorReporter() ErrorReporter {
	return SentryErrorReporter{}
}

type SentryErrorReporter struct{}

var _ ErrorReporter = SentryErrorReporter{}

func (s SentryErrorReporter) Setup() func() {
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

func (s SentryErrorReporter) SetUser(user ErrorUser) {
	scope := sentry.CurrentHub().Scope()
	scope.SetUser(sentry.User{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	})
}

func (s SentryErrorReporter) Flush() {
	sentry.Flush(time.Second * 2)
}

type ErrReportBreadCrumb struct {
	Type     string
	Category string
	Message  string
	Level    string
}

func (s SentryErrorReporter) AddBreadCrumb(bc ErrReportBreadCrumb) {
	scope := sentry.CurrentHub()
	scope.AddBreadcrumb(&sentry.Breadcrumb{
		Type:     bc.Type,
		Category: bc.Category,
		Message:  bc.Message,
		Level:    sentry.Level(bc.Level),
	}, &sentry.BreadcrumbHint{})
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

var NetworkErrorMessage = "possible internet connection problem"

type SessionExpiredError struct {
	HasPreviousSession bool
}

func (e *SessionExpiredError) Error() string {
	if e.HasPreviousSession {
		return "Your session has expired due to inactivity"
	}
	return "You are not logged in"
}

func (e *SessionExpiredError) Directive() string {
	if e.HasPreviousSession {
		return "Please log in again to continue using the CLI"
	}
	return "Please log in to use the CLI"
}

type CredentialsFileNotFound struct{}

func (e *CredentialsFileNotFound) Directive() string {
	return "run `brev login`"
}

func (e *CredentialsFileNotFound) Error() string {
	return "credentials file not found"
}

type WorkspaceNotRunning struct {
	Status string
}

func (e WorkspaceNotRunning) Error() string {
	return fmt.Sprintf("workspace status %s is not RUNNING", e.Status)
}

var New = stderrors.New

var Errorf = fmt.Errorf

func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return Errorf("%s: %w", msg, err)
}

var As = stderrors.As

var Unwrap = stderrors.Unwrap

func Unwraps(err error) []error {
	u, ok := err.(interface {
		Unwrap() []error
	})
	if !ok {
		return nil
	}
	return u.Unwrap()
}

func Root(err error) error {
	for Unwrap(err) != nil {
		err = Unwrap(err)
	}
	joinedErrs := Unwraps(err)
	if len(joinedErrs) == 0 {
		return err
	}
	return Roots(joinedErrs)
}

func Roots(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	rootedErrs := make([]error, len(errs))
	for i, e := range errs {
		rootedErrs[i] = Root(e)
	}
	return Join(rootedErrs...)
}

// flattens error tree
func Flatten(err error) []error {
	if err == nil {
		return nil
	}
	joinedErrs := Unwraps(err)
	if joinedErrs == nil {
		return []error{err}
	}
	flatErrs := []error{}
	for _, e := range joinedErrs {
		flatErrs = append(flatErrs, Flatten(e)...)
	}
	return flatErrs
}

// var ReturnTrace = errtrace.Wrap

func Join(errs ...error) error {
	noNilErrs := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			noNilErrs = append(noNilErrs, err)
		}
	}
	if len(noNilErrs) == 0 {
		return nil
	}
	if len(noNilErrs) == 1 {
		return noNilErrs[0]
	}
	return stderrors.Join(errs...) //nolint:wrapcheck // this is a wrapper
}

// if multi err, combine similar errors
func CombineByString(err error) error {
	if err == nil {
		return nil
	}
	errs := Flatten(err)
	mapE := make(map[string]error)
	mapEList := []error{}
	for _, e := range errs {
		_, ok := mapE[e.Error()]
		if !ok {
			mapE[e.Error()] = e
			mapEList = append(mapEList, e)
		}
	}
	errsOut := make([]error, 0, len(mapE))
	for _, e := range mapEList { //nolint:gosimple //ok
		errsOut = append(errsOut, e)
	}
	return Join(errsOut...)
}

var Is = stderrors.Is

var WrapAndTrace = WrapAndTraceInMsg

func WrapAndTraceInMsg(err error) error {
	if err == nil {
		return nil
	}
	return pkgerrors.Wrap(err, makeErrorMessage("", 0)) // this wrap also adds a stacktrace which can be nice
}

func makeErrorMessage(message string, skip int) string {
	skip += 2
	pc, file, line, _ := runtime.Caller(skip)

	funcName := "unknown"
	fn := runtime.FuncForPC(pc)
	if fn != nil {
		funcName = fn.Name()
	}

	lineNum := strconv.Itoa(line)
	return fmt.Sprintf("[error] %s\n%s\n%s:%s\n", message, funcName, file, lineNum)
}

// logger.L().Error("", zap.Error(err))

type NvidiaMigrationError struct {
	Message string
}

func (e NvidiaMigrationError) Error() string {
	return e.Message
}

func (e NvidiaMigrationError) Directive() string {
	return "Please run 'brev login --auth nvidia' to log in with your NVIDIA account"
}

func NewNvidiaMigrationError(msg string) *NvidiaMigrationError {
	return &NvidiaMigrationError{Message: msg}
}
