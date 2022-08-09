package hello

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/afero"
)

func GetFirstName(name string) string {
	appropriatelyCapitalized := strings.Title(strings.ToLower(name))
	split := strings.Split(appropriatelyCapitalized, " ")
	if len(split) > 1 {
		return split[0]
	}
	return name
}

func CanWeOnboard(t *terminal.Terminal, user *entity.User) {
	s := t.Green("\n\nHi " + GetFirstName(user.Name) + "! Looks like it's your first time using Brev!\n")

	TypeItToMe(s)

	res := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label:    "Want a quick tour?",
		ErrorMsg: "Please pick yes or no",
		Items:    []string{"Yes!", "No, I'll read docs later"},
	})
	if res == "Yes!" {
		RunOnboarding(t, user)
	} else {
		t.Vprintf("\nOkay, you can always read the docs at %s\n\n", t.Yellow("https://brev.dev/docs"))
	}
}

func TESTReadAndWriteFile() error {
	fmt.Println("reading file")
	res, err := GetOnboardingObject()
	if err != nil {
		return err
	}
	fmt.Println(res.Step)

	newVal := 1
	fmt.Println("writing " + fmt.Sprint(newVal) + " to file")
	err = SetOnboardingStep(1)
	if err != nil {
		return err
	}

	res, err = GetOnboardingObject()
	if err != nil {
		return err
	}
	fmt.Println(res.Step)

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
	Step            int  `json:"step"`
	HasRunBrevShell bool `json:"hasRunBrevShell"`
	HasRunBrevOpen  bool `json:"hasRunBrevOpen"`
}

func SetupDefaultOnboardingFile() error {
	// get path
	path, err := GetOnboardingFilePath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	exists, err := afero.Exists(files.AppFs, path)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if !exists {
		if err = files.AppFs.MkdirAll(filepath.Dir(path), 0o775); err != nil {
			return breverrors.WrapAndTrace(err)
		}
		_, err = files.AppFs.Create(path)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		oo := OnboardingObject{0, false, false}
		err = files.OverwriteJSON(files.AppFs, path, &oo)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	return nil
}

func GetOnboardingObject() (*OnboardingObject, error) {
	// get path
	path, err := GetOnboardingFilePath()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	// Ensure file exists
	SetupDefaultOnboardingFile()

	// read file
	var oo OnboardingObject
	err = files.ReadJSON(files.AppFs, path, &oo)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	// return data
	return &oo, nil
}

func SetOnboardingObject(oo OnboardingObject) error {
	// get path
	path, err := GetOnboardingFilePath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Ensure file exists
	SetupDefaultOnboardingFile()

	// write file
	err = files.OverwriteJSON(files.AppFs, path, &oo)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// return data
	return nil
}

func SetOnboardingStep(step int) error {
	// get path
	path, err := GetOnboardingFilePath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Ensure file exists
	SetupDefaultOnboardingFile()

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

func SetHasRunShell(hasRunShell bool) error {
	// get path
	path, err := GetOnboardingFilePath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Ensure file exists
	SetupDefaultOnboardingFile()

	// read file
	var oo OnboardingObject
	err = files.ReadJSON(files.AppFs, path, &oo)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// write file
	oo.HasRunBrevShell = hasRunShell
	err = files.OverwriteJSON(files.AppFs, path, &oo)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// return data
	return nil
}

func SetHasRunOpen(hasRunOpen bool) error {
	// get path
	path, err := GetOnboardingFilePath()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Ensure file exists
	SetupDefaultOnboardingFile()

	// read file
	var oo OnboardingObject
	err = files.ReadJSON(files.AppFs, path, &oo)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// write file
	oo.HasRunBrevOpen = hasRunOpen
	err = files.OverwriteJSON(files.AppFs, path, &oo)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// return data
	return nil
}

func stall(t *terminal.Terminal, env entity.Workspace) {
	s := "\nYour dev environment is still deploying. Sometimes this happens and it takes a lot longer than usual."
	s += "\nWe're working really hard to fix that."
	s += "\n\n"
	TypeItToMe(s)

	res := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label:    "In the meantime, I can tell you a bit about us?",
		ErrorMsg: "Please pick yes or no",
		Items:    []string{"Yes!", "Nah, just show me a spinner"},
	})
	if res == "Yes!" {
		tellMeAboutBrevAndPoll(t, env)
	} else {
		Poll(t, env)
	}
}

func tellMeAboutBrevAndPoll(t *terminal.Terminal, env entity.Workspace) {
	s := "In the beginning..."
	TypeItToMe(s)
}

func Poll(t *terminal.Terminal, env entity.Workspace) {
	s := "Got it."
	TypeItToMe(s)
}
