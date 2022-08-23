package hello

import (
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

func ShouldWeRunOnboarding(s HelloStore) bool {
	workspaceID, err := s.GetCurrentWorkspaceID()
	if err != nil {
		return false
	}
	if workspaceID != "" {
		return false
	}

	oo, err := GetOnboardingObject()
	if err != nil {
		return true
	}
	if oo.Step == 0 && !oo.HasRunBrevOpen && !oo.HasRunBrevShell {
		return true
	} else {
		return false
	}
}

func CanWeOnboard(t *terminal.Terminal, user *entity.User, store HelloStore) error {
	s := t.Green("\n\nHi " + GetFirstName(user.Name) + "! Looks like it's your first time using Brev!\n")

	TypeItToMeUnskippable(s)

	res := terminal.PromptSelectInput(terminal.PromptSelectContent{
		Label:    "Want a quick tour?",
		ErrorMsg: "Please pick yes or no",
		Items:    []string{"Yes!", "No, I'll read docs later"},
	})
	if res == "Yes!" {
		err := RunOnboarding(t, user, store)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	} else {
		_ = SetOnboardingObject(OnboardingObject{
			Step:            1,
			HasRunBrevOpen:  true,
			HasRunBrevShell: true,
		})

		_ = SkippedOnboarding(user, store)

		t.Vprintf("\nOkay, you can always read the docs at %s\n\n", t.Yellow("https://brev.dev/docs"))
	}
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
	err = SetupDefaultOnboardingFile()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

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
	err = SetupDefaultOnboardingFile()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

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
	err = SetupDefaultOnboardingFile()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

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
	err = SetupDefaultOnboardingFile()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

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
	err = SetupDefaultOnboardingFile()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

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

func Poll() {
	s := "Got it."
	TypeItToMe(s)
}
