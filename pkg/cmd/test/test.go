package test

import (
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	startLong    = "[internal] test"
	startExample = "[internal] test"
)

type TestStore interface {
	completions.CompletionStore
	ResetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetAllWorkspaces(options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetWorkspace(id string) (*entity.Workspace, error)
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
}

func NewCmdTest(t *terminal.Terminal, _ TestStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"devonly": ""},
		Use:                   "test",
		DisableFlagsInUseLine: true,
		Short:                 "[internal] Test random stuff.",
		Long:                  startLong,
		Example:               startExample,
		Run: func(cmd *cobra.Command, args []string) {
			t.Vprint("\ntest cmd\n")

			// Check IDE requirements
			// doneAddingKey := terminal.PromptSelectInput(terminal.PromptSelectContent{
			// 	Label:    "Done adding your SSH key?",
			// 	ErrorMsg: "error",
			// 	Items:    []string{"yes", "no", "skip"},
			// })

			// if doneAddingKey == "skip" {
			// 	t.Vprint(t.Yellow("\nFeel free to proceed but you will not be able to pull or push your private repos. Run 'brev ssh-key' to do this step later."))
			// }

			// t.Vprint(args[0] + "\n")
			// wsmeta, err := getWorkspaceFromNameOrID(args[0], store)
			// if err != nil {
			// 	t.Vprint(err.Error())
			// }

			// t.Vprintf("%s %s %s", wsmeta.Name, wsmeta.DNS, wsmeta.ID)

			// var err error

			// // SSH Keys
			// terminal.DisplayBrevLogo(t)
			// t.Vprintf("\n")
			// spinner := t.NewSpinner()
			// spinner.Suffix = " fetching your public key"
			// spinner.Start()
			// terminal.DisplaySSHKeys(t, "blahhhh fake key")
			// spinner.Stop()
			// if err != nil {
			// 	t.Vprintf(t.Red(err.Error()))
			// }

			// hasVSCode := terminal.PromptSelectInput(terminal.PromptSelectContent{
			// 	Label:    "Do you use VS Code?",
			// 	ErrorMsg: "error",
			// 	Items:    []string{"yes", "no"},
			// })
			// if hasVSCode == "yes" {
			// 	// TODO: check if user uses VSCode and intall extension for user
			// 	t.Vprintf(t.Yellow("Please install the following VS Code extension: ms-vscode-remote.remote-ssh\n"))
			// }

			// brevapi.InstallVSCodeExtension(t)

			// NOTE: this only works on Mac
			// err = beeep.Notify("Title", "Message body", "assets/information.png")
			// t.Vprintf("we just setn the beeeep")
			// if err != nil {
			// 	fmt.Println(err)
			// }

			// err = beeep.Beep(beeep.DefaultFreq, beeep.DefaultDuration)
			// if err != nil {
			// 	fmt.Println(err)
			// }

			// err = beeep.Alert("Title", "Message body", "assets/warning.png")
			// if err != nil {
			// 	fmt.Println(err)
			// }
			// err := ssh.RunTasks()
			// err := ssh.RunTaskAsDaemon(".")
			// if err != nil {
			// 	fmt.Println(err)
			// }
		},
	}

	return cmd
}

func SlashToDash(s string) string {
	splitBySlash := strings.Split(s, "/")

	concatenated := strings.Join(splitBySlash, "-")

	splitByColon := strings.Split(concatenated, ":")

	emacsSafeString := strings.Join(splitByColon, "-")

	return emacsSafeString
}
