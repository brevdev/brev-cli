package hello

import (
	"os"
	"path/filepath"
	"strings"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
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

func GetOnboardingFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	path := files.GetOnboardingStepPath(home)
	return path, nil
}

type OnboardingObject struct {
	Step            int  `json:"step"`
	HasRunBrevShell bool `json:"hasRunBrevShell"`
	HasRunBrevOpen  bool `json:"hasRunBrevOpen"`
}

func shellOnboardingPollDone(res *OnboardingObject) bool {
	return res != nil && res.HasRunBrevShell
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

// get path

// Ensure file exists

// write file

// return data

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
