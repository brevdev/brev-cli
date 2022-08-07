package hello

import (
	"time"

	"github.com/brevdev/brev-cli/pkg/terminal"
)

/*
	Step 1:
		The user just ran brev ls
*/
func Step1(t *terminal.Terminal) {
	// TODO: multithread this
	// allWorkspaces, err := ls.lsStore.GetWorkspaces(org.ID, nil)
	// if err != nil {
	// 	return breverrors.WrapAndTrace(err)
	// }

	s := "\n\nThe command " + t.Yellow("brev ls") + " shows your dev environments"
	s += "\nIf the dev environment is " + t.Green("RUNNING") + ", you can open it."
	s += "\n\nIn a new terminal, try running " + t.Green("brev shell heyooo") + " to get a terminal in your dev environment\n"
	TypeItToMe(s)

	// Reset the onboarding object to walk through the onboarding fresh
	res, err := GetOnboardingObject()
	if err != nil {
		return
	}
	SetOnboardingObject(OnboardingObject{res.Step, false, false})

	// a while loop in golang
	sum := 0
	spinner := t.NewSpinner()
	spinner.Suffix = "☝️ try that, I'll wait"
	spinner.Start()
	for sum > -1 {
		sum += 1
		res, err := GetOnboardingObject()
		if err != nil {
			return
		}
		if res.HasRunBrevShell {
			spinner.Suffix = "🎉 you did it!"
			time.Sleep(100 * time.Millisecond)
			spinner.Stop()
			break
		} else {
			time.Sleep(1 * time.Second)
		}
	}

	s = "\n\nAwesome! Now try opening VS Code in that environment"
	s += "\nIn a new terminal, try running " + t.Green("brev open heyooo") + " to open VS Code in the dev environment\n"
	TypeItToMe(s)

	// a while loop in golang
	sum = 0
	spinner.Suffix = "☝️ try that, I'll wait"
	spinner.Start()
	for sum < 1 {
		sum += sum
		res, err := GetOnboardingObject()
		if err != nil {
			return
		}
		if res.HasRunBrevOpen {
			spinner.Suffix = "🎉 you did it!"
			time.Sleep(100 * time.Millisecond)
			spinner.Stop()
			sum += 1
			break
		} else {
			time.Sleep(1 * time.Second)
		}
	}

	s = "\n\nI think I'm done here. Now you know how to open a dev environment and start coding."
	s += "Head to the console at " + t.Green("https://console.brev.dev") + " to create a new dev environment or share it with people"
	s += "\n\nYou can also read the docs at " + t.Yellow("https://brev.dev/docs") + "\n\n"
	TypeItToMe(s)
}
