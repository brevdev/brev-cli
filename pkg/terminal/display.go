package terminal

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/manifoldco/promptui"
)

type PromptSelectContent struct {
	ErrorMsg string
	Label    string
	Items    []string
}

func PromptSelectInput(pc PromptSelectContent) string {
	// templates := &promptui.SelectTemplates{
	// 	Label:  "{{ . }} ",
	// 	Selected:   "{{ . | green }} ",
	// 	Active: "{{ . | cyan }} ",
	// }

	prompt := promptui.Select{
		Label: pc.Label,
		Items: pc.Items,
		// Templates: templates,
	}

	_, result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		os.Exit(1)
	}

	return result
}

func DisplaySSHKeys(t *Terminal, publicKey string) {
	t.Eprint(t.Yellow("\nCreate an SSH Key with your git provider. Copy your public key: \n"))
	t.Vprintf(publicKey)
	t.Eprintf(t.Yellow("\n\nand save it to your git provider."))
	t.Eprintf(t.Yellow("\n\tClick here for Github: https://github.com/settings/keys"))
	t.Eprintf(t.Yellow("\n\tClick here for Gitlab: https://gitlab.com/-/profile/keys\n"))
}

func DisplayBrevLogo(t *Terminal) {
	t.Vprint("                               _  ")
	t.Vprint("              _        ,-.    / ) ")
	t.Vprint("             ( `.     // /-._/ / ")
	t.Vprint("              `\\ \\   /(_/ / / /  ")
	t.Vprint("                ; `-`  (_/ / /  ")
	t.Vprint("                |       (_/ /     ")
	t.Vprint("                \\          /      ")
	t.Vprint("Welcome to       )       /`      ")
	t.Vprint("      Brev.Dev   /      /`       ")
}

func InstallVSCodeExtension(t *Terminal) {
	cmdd := exec.Command("which code")
	output, _ := cmdd.Output()
	t.Vprintf("%b", output)
	_, err := cmdd.Output()

	if err != nil {
		t.Vprintf(t.Yellow("Please install the following VS Code extension: ms-vscode-remote.remote-ssh\n"))
	} else {
		install := exec.Command("code --install-extension ms-vscode-remote.remote-ssh\n")
		_, err := install.Output()
		if err != nil {
			t.Vprintf("Please install the following VS Code extension: ms-vscode-remote.remote-ssh\n")
		}

	}
}

type PromptContent struct {
	ErrorMsg string
	Label    string
	Default  string
}

func PromptGetInput(pc PromptContent) string {
	validate := func(input string) error {
		if len(input) == 0 {
			return breverrors.WrapAndTrace(errors.New(pc.ErrorMsg))
		}
		return nil
	}

	templates := &promptui.PromptTemplates{
		Prompt:  "{{ . }} ",
		Valid:   "{{ . | green }} ",
		Invalid: "{{ . | yellow }} ",
		Success: "{{ . | bold }} ",
	}

	prompt := promptui.Prompt{
		Label:     pc.Label,
		Templates: templates,
		Validate:  validate,
		Default:   pc.Default,
		AllowEdit: true,
	}

	result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		os.Exit(1)
	}

	return result
}
