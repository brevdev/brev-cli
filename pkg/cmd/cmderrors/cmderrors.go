package cmderrors

import (
	"fmt"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/terminal"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

// determines if should print error stack trace and/or send to crash monitor

func DisplayAndHandleError(err error) {
	er := breverrors.GetDefaultErrorReporter()
	er.AddBreadCrumb(breverrors.ErrReportBreadCrumb{
		Type:     "default",
		Category: "stacktrace",
		Level:    string(sentry.LevelError),
		Message:  err.Error(),
	})
	if err != nil {
		t := terminal.New()
		prettyErr := ""
		switch errors.Cause(err).(type) {
		case breverrors.ValidationError:
			// do not report error
			prettyErr = (t.Yellow(errors.Cause(err).Error()))
		case breverrors.WorkspaceNotRunning: // report error to track when this occurs, but don't print stacktrace to user unless in dev mode
			er.ReportError(err)
			prettyErr = (t.Yellow(errors.Cause(err).Error()))
		default:
			if isSneakyValidationErr(err) {
				prettyErr = (t.Yellow(errors.Cause(err).Error()))
			} else {
				er.ReportError(err)
				prettyErr = (t.Red(errors.Cause(err).Error()))
			}
		}
		if featureflag.Debug() || featureflag.IsDev() {
			fmt.Println(err)
		} else {
			fmt.Println(prettyErr)
		}
	}
}

func isSneakyValidationErr(err error) bool {
	return strings.Contains(err.Error(), "unknown flag:") || strings.Contains(err.Error(), "unknown command")
}

func TransformToValidationError(pa cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		err := pa(cmd, args)
		if err != nil {
			return breverrors.NewValidationError(err.Error())
		}
		return nil
	}
}
