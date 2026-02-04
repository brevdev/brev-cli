package util

import (
	"bufio"
	"os"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

// IsStdoutPiped returns true if stdout is being piped to another command
// Enables command chaining like: brev ls | grep RUNNING | brev stop
func IsStdoutPiped() bool {
	stat, _ := os.Stdout.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// IsStdinPiped returns true if stdin is being piped from another command
func IsStdinPiped() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// GetInstanceNames gets instance names from args or stdin (supports piping)
// Returns error if no names are provided
func GetInstanceNames(args []string) ([]string, error) {
	names, _ := GetInstanceNamesWithPipeInfo(args)
	if len(names) == 0 {
		return nil, breverrors.NewValidationError("instance name required: provide as argument or pipe from another command")
	}
	return names, nil
}

// GetInstanceNamesWithPipeInfo gets instance names from args or stdin and
// returns whether stdin was piped. Useful when you need to know if input came
// from a pipe vs args.
func GetInstanceNamesWithPipeInfo(args []string) ([]string, bool) {
	var names []string
	names = append(names, args...)

	stdinPiped := IsStdinPiped()
	if stdinPiped {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			name := strings.TrimSpace(scanner.Text())
			if name != "" {
				names = append(names, name)
			}
		}
	}

	return names, stdinPiped
}
