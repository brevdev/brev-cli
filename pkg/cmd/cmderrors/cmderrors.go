package cmderrors

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/terminal"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

// determines if should print error stack trace and/or send to crash monitor
func DisplayAndHandleCmdError(name string, cmdFunc func() error) error {
	er := breverrors.GetDefaultErrorReporter()
	er.AddTag("command", name)
	err := cmdFunc()
	if err != nil {
		er.ReportMessage(err.Error())
		er.ReportError(err)
		if featureflag.Debug() || featureflag.IsDev() {
			return err
		} else {
			return errors.Cause(err) //nolint:wrapcheck //no check
		}
	}
	return nil
}

func DisplayAndHandleError(err error) {
	if err != nil {
		t := terminal.New()
		prettyErr := ""
		switch err.(type) {
		case breverrors.ValidationError:
			// do not report error
			prettyErr = (t.Yellow(errors.Cause(err).Error()))
		default:
			if isSneakyValidationErr(err) {
				prettyErr = (t.Yellow(errors.Cause(err).Error()))
			} else {
				er := breverrors.GetDefaultErrorReporter()
				er.ReportMessage(err.Error())
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
	return strings.Contains(err.Error(), "unknown flag:")
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
