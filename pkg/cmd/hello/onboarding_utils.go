package hello

import (
	"fmt"
	"os"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

func CanWeOnboard(t *terminal.Terminal) {
	terminal.DisplayBrevLogo(t)

	t.Vprintf(t.Green("\n\nHey! Looks like it's your first time using Brev!\n"))

	res := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label:    "Want a quick tour?",
		ErrorMsg: "Please pick yes or no",
		Items:    []string{"Yes!", "No, I'll read docs later"},
	})
	if res == "Yes!" {
		RunOnboarding(t)
	} else {
		t.Vprintf("\nOkay, you can always read the docs at %s\n\n", t.Yellow("https://brev.dev/docs"))
	}
}

func TESTReadAndWriteFile() error {
	fmt.Println("reading file")
	res, err := GetOnboardingStep()
	if err != nil {
		return err
	}
	fmt.Println(res)

	newVal := 1
	fmt.Println("writing " + fmt.Sprint(newVal) + " to file")
	err = SetOnboardingStep(1)
	if err != nil {
		return err
	}

	res, err = GetOnboardingStep()
	if err != nil {
		return err
	}
	fmt.Println(res)

	return nil
}

func GetOnboardingFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	path, err := files.GetOnboardingStepPath(home)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	return path, nil
}

type OnboardingObject struct {
	Step int `json:"step"`
}

func GetOnboardingStep() (int, error) {
	// get path
	path, err := GetOnboardingFilePath()
	if err != nil {
		return 0, breverrors.WrapAndTrace(err)
	}

	// BANANA: ensure path exists
	// exists, err := afero.Exists(s.fs, path)
	// if err != nil {
	// 	return 0, breverrors.WrapAndTrace(err)
	// }
	// if !exists {
	// 	return nil, &breverrors.CredentialsFileNotFound{}
	// }

	// read file
	var oo OnboardingObject
	err = files.ReadJSON(files.AppFs, path, &oo)
	if err != nil {
		return 0, breverrors.WrapAndTrace(err)
	}

	// return data
	return oo.Step, nil
}

func SetOnboardingStep(step int) error {
	// get path
	path, err := GetOnboardingFilePath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// BANANA: ensure path exists
	// exists, err := afero.Exists(s.fs, path)
	// if err != nil {
	// 	return 0, breverrors.WrapAndTrace(err)
	// }
	// if !exists {
	// 	return nil, &breverrors.CredentialsFileNotFound{}
	// }

	// write file
	oo := OnboardingObject{
		Step: step,
	}
	err = files.OverwriteJSON(files.AppFs, path, &oo)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// return data
	return nil
}
