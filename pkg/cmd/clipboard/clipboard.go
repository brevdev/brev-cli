package clipboard

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
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
		Args:                  cmderrors.TransformToValidationError(cobra.ExactArgs(0)),
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

func SaveToClipboard(output string) {
	// copy to clipboard
	command := fmt.Sprintf("echo %s | pbcopy", output)
	cmd := exec.Command("bash", "-c", command) //nolint:gosec // testing
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		fmt.Println(err)
	}
	err1 := cmd.Wait()
	if err1 != nil {
		fmt.Println(err1)
	}
}

// Step 2
func ForwardPort(t *terminal.Terminal, clipboardStore ClipboardStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "remote-forward <ssh-name>",
		DisableFlagsInUseLine: true,
		Short:                 "remote forward port",
		Long:                  "remote forward port",
		Example:               "remote-forward",
		Args:                  cmderrors.TransformToValidationError(cobra.ExactArgs(1)),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(clipboardStore, t),
		Run: func(cmd *cobra.Command, args []string) {
			// Portforward
			_, sshError := portforward.RunSSHPortForward("-R", "6969", "6969", args[0])
			if sshError != nil {
				t.Errprint(sshError, "Failed to connect to local")
				return
			}
		},
	}
	return cmd
}

// Step 3
func SendToClipboard(t *terminal.Terminal, clipboardStore ClipboardStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"clipboard": ""},
		Use:                   "clipboard",
		DisableFlagsInUseLine: true,
		Short:                 "Copies clipboard from remote environment to local clipboard",
		Long:                  "Copies clipboard from remote environment to local clipboard",
		Example:               "clipboard",
		Args:                  cmderrors.TransformToValidationError(cobra.ExactArgs(0)),
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
