package spark

import (
	"fmt"
	"strings"

	"github.com/brevdev/brev-cli/pkg/terminal"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

// SelectHost chooses the correct Host based on user input or interactive prompt.
func SelectHost(hosts []Host, requestedAlias string, prompter Prompter) (Host, error) {
	if len(hosts) == 0 {
		return Host{}, breverrors.WrapAndTrace(fmt.Errorf("no Spark hosts available; open NVIDIA Sync and retry"))
	}

	if requestedAlias != "" {
		for _, h := range hosts {
			if h.Alias == requestedAlias {
				return h, nil
			}
		}
		return Host{}, breverrors.WrapAndTrace(fmt.Errorf("Spark host %s not found. Available: %s", requestedAlias, strings.Join(hostAliases(hosts), ", ")))
	}

	if len(hosts) == 1 {
		return hosts[0], nil
	}

	if prompter == nil {
		return Host{}, breverrors.WrapAndTrace(fmt.Errorf("multiple Spark hosts detected; rerun with an alias (%s)", strings.Join(hostAliases(hosts), ", ")))
	}

	label := "Select Spark host"
	choices := hostChoices(hosts)
	selection, err := prompter.Select(label, choices)
	if err != nil {
		return Host{}, breverrors.WrapAndTrace(err)
	}

	for i, choice := range choices {
		if choice == selection {
			return hosts[i], nil
		}
	}

	return Host{}, breverrors.WrapAndTrace(fmt.Errorf("invalid selection"))
}

// TerminalPrompter bridges to the CLI prompt implementation.
type TerminalPrompter struct{}

func (TerminalPrompter) Select(label string, options []string) (string, error) {
	content := terminal.PromptSelectContent{
		Label: label,
		Items: options,
	}
	return terminal.PromptSelectInput(content), nil
}

func hostAliases(hosts []Host) []string {
	aliases := make([]string, 0, len(hosts))
	for _, h := range hosts {
		aliases = append(aliases, h.Alias)
	}
	return aliases
}

func hostChoices(hosts []Host) []string {
	choices := make([]string, 0, len(hosts))
	for _, h := range hosts {
		choices = append(choices, fmt.Sprintf("%s (%s@%s:%d)", h.Alias, h.User, h.Hostname, h.Port))
	}
	return choices
}
