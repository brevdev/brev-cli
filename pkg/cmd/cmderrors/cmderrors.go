package cmderrors

import (
	"fmt"
	"os"
	"os/exec"
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
		case *breverrors.NvidiaMigrationError:
			// Handle nvidia migration error
			if nvErr, ok := errors.Cause(err).(*breverrors.NvidiaMigrationError); ok {
				fmt.Println("\n This account has been migrated to NVIDIA Auth. Attempting to log in with NVIDIA account...")
				brevBin, err1 := os.Executable()
				if err1 == nil {
					cmd := exec.Command(brevBin, "login", "--auth", "nvidia") // #nosec G204
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					cmd.Stdin = os.Stdin
					loginErr := cmd.Run() // If automatic login succeeds, we'll exit without showing the original error
					if loginErr == nil {
						// Login successful, don't show the original error
						return
					}
				}
				// Only show the error if automatic login failed or couldn't be attempted
				prettyErr = t.Yellow(nvErr.Error() + "\n" + nvErr.Directive())
			} else {
				// Fallback in case type assertion fails (shouldn't happen but better safe than sorry)
				prettyErr = t.Red(errors.Cause(err).Error())
				er.ReportError(err)
			}
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
