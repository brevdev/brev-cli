package clipboard

import (
	"github.com/brevdev/brev-cli/pkg/cmd/completions"

	"github.com/brevdev/brev-cli/pkg/cmd/portforward"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

type ClipboardStore interface {
	completions.CompletionStore
}

// Step 1
func EstablishConnection(t *terminal.Terminal, clipboardStore ClipboardStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "connect",
		DisableFlagsInUseLine: true,
		Short:                 "Connects to remote",
		Long:                  "Connects to remote",
		Example:               "connect",
		Args:                  cobra.ExactArgs(0),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(clipboardStore, t),
		Run: func(cmd *cobra.Command, args []string) {
			// Listen to Port
			listener := CreateListener(&Config{
				Host: "127.0.0.1",
				Port: "6969",
			})
			listener.Run()
		},
	}
	return cmd
}

// Step 2
func ForwardPort(t *terminal.Terminal, clipboardStore ClipboardStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "forward",
		DisableFlagsInUseLine: true,
		Short:                 "forward port",
		Long:                  "forward port",
		Example:               "forward",
		Args:                  cobra.ExactArgs(0),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(clipboardStore, t),
		Run: func(cmd *cobra.Command, args []string) {
			// Portforward
			_, sshError := portforward.RunSSHPortForward("-R", "6969", "6969", "peertopeer2")
			if sshError != nil {
				t.Errprint(sshError, "Failed to connect to local")
				return
			}
		},
	}
	return cmd
}

// Step 3
func UseClipboard(t *terminal.Terminal, clipboardStore ClipboardStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"clipboard": ""},
		Use:                   "clipboard",
		DisableFlagsInUseLine: true,
		Short:                 "Copies clipboard from remote environment to local clipboard",
		Long:                  "Copies clipboard from remote environment to local clipboard",
		Example:               "clipboard",
		Args:                  cobra.ExactArgs(0),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(clipboardStore, t),
		Run: func(cmd *cobra.Command, args []string) {
			// Send output
			err := SendRequest("localhost:6969", "hello world")
			if err != nil {
				t.Errprint(err, "Failed to copy to clipboard")
				return
			}
		},
	}

	return cmd
}
