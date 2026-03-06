package names

import (
	"fmt"
	"regexp"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

var validNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

const maxNameLen = 63

func ValidateNodeName(name string) error {
	if name == "" {
		return breverrors.NewValidationError("name is required")
	}
	if len(name) > maxNameLen {
		return breverrors.NewValidationError(
			fmt.Sprintf("name must be %d characters or fewer (got %d)", maxNameLen, len(name)))
	}
	if !validNameRe.MatchString(name) {
		return breverrors.NewValidationError(
			"name must start with a letter or digit and contain only letters, digits, hyphens, underscores, and dots")
	}
	return nil
}
