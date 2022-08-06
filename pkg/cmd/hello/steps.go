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

	// a while loop in golang
	sum := 0
	spinner := t.NewSpinner()
	spinner.Suffix = "☝️ try that, I'll wait"
	spinner.Start()
	for sum < 1 {
		sum += sum
		res, err := GetOnboardingObject()
		if err != nil {
			return
		}
		if res.HasRunBrevShell {
			spinner.Stop()
			sum += 1
			break
		} else {
			time.Sleep(1 * time.Second)
		}
	}
	s = "\n\nYou did it!\n"
	TypeItToMe(s)
}
