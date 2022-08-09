package hello

import (
	"fmt"
	"time"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/fatih/color"
)

func Stall(t *terminal.Terminal, workspace entity.Workspace) {
}

const DEFAULT_DEV_ENV_NAME = "first-workspace-react"

/*
	Return nil to exit the onboarding
*/
func GetDevEnvOrStall(t *terminal.Terminal, workspaces []entity.Workspace) *entity.Workspace {
	var firstDevEnv entity.Workspace
	var runningDevEnvs []entity.Workspace
	noneFound := true
	for _, v := range workspaces {
		if v.Name == DEFAULT_DEV_ENV_NAME {
			firstDevEnv = v
			noneFound = false
		}
		if v.Status == "RUNNING" {
			runningDevEnvs = append(runningDevEnvs, v)
		}
	}

	if noneFound {
		s := t.Red("Please create a running dev environment for this walk through. ")
		s += "\n\tYou can do that here: " + t.Yellow("https://console.brev.dev/environments/new")
		s += "\n\t\t-- Or --\n\tRun this in your terminal 👉 " + t.Yellow("brev start https://github.com/brevdev/hello-react --name %s", DEFAULT_DEV_ENV_NAME)
		s += "\n\nRun: " + t.Yellow("brev hello") + " to resume this walk through when your dev env is ready\n"
		TypeItToMe(s)
		return nil
	}

	if firstDevEnv.Status == "RUNNING" {
		// all is good, proceed.
		return &firstDevEnv
	} else if firstDevEnv.Status == "DEPLOYING" {
		s := t.Yellow("Your dev environment is deploying.")
		s += "\nPlease wait for it to finish deploying then run " + t.Yellow("brev hello") + " to resume this walk through when your dev env is ready\n"
		TypeItToMe(s)
		return nil

	} else if firstDevEnv.Status == "UNHEALTHY" {
		s := t.Red("Your dev environment seems stuck. Can you reach out to support?")
		s += "\nMessage us "
		s += "\n\t in discord 👉 " + t.Yellow("https://discord.gg/RpszWaJFRA")
		s += "\n\t via text or call 👉 " + t.Yellow("(415) 237-2247\n")
		s += "\n\nRun " + t.Yellow("brev hello") + " to resume this walk through when your dev env is ready\n"
		TypeItToMe(s)
		return nil

	} else if firstDevEnv.Status == "STOPPED" {
		s := t.Yellow("Your dev environment is stopped.")
		s += "\nRun this in your terminal to start it 👉 " + t.Yellow("brev start %s", DEFAULT_DEV_ENV_NAME)
		s += "\n\nRun " + t.Yellow("brev hello") + " to resume this walk through when your dev env is ready\n"
		TypeItToMe(s)
		return nil

	} else if firstDevEnv.Status == "STOPPING" {
		s := t.Yellow("Your dev environment is stopped.")
		s += "\nRun this in your terminal to start it 👉 " + t.Yellow("brev start %s", DEFAULT_DEV_ENV_NAME)
		s += "\n\nRun " + t.Yellow("brev hello") + " to resume this walk through when your dev env is ready\n"
		TypeItToMe(s)
		return nil

	} else {
		s := t.Red("Please create a running dev environment for this walk through. ")
		s += "\n\tYou can do that here: " + t.Yellow("https://console.brev.dev/environments/new")
		s += "\n\t\t-- Or --\n\tRun this in your terminal 👉 " + t.Yellow("brev start https://github.com/brevdev/hello-react --name %s", DEFAULT_DEV_ENV_NAME)
		s += "\n\nRun " + t.Yellow("brev hello") + " to resume this walk through when your dev env is ready\n"
		TypeItToMe(s)
		return nil
	}
}

/*
	Step 1:
		The user just ran brev ls
*/
func Step1(t *terminal.Terminal, workspaces []entity.Workspace, user *entity.User, store HelloStore) {
	CompletedOnboardingLs(user, store)

	firstWorkspace := GetDevEnvOrStall(t, workspaces)
	if firstWorkspace == nil {
		return
	}

	s := "\n\nThe command " + t.Yellow("brev ls") + " shows your dev environments"
	s += "\nIf the dev environment is " + t.Green("RUNNING") + ", you can open it."
	s += "\n\nIn a new terminal, try running " + t.Green("brev shell %s", firstWorkspace.Name) + " to get a terminal in your dev environment\n"
	TypeItToMe(s)

	// a while loop in golang
	sum := 0
	spinner := t.NewSpinner()
	spinner.Suffix = "👆 try that, I'll wait"
	spinner.Start()
	for sum > -1 {
		sum += 1
		res, err := GetOnboardingObject()
		if err != nil {
			return
		}
		if res.HasRunBrevShell {
			spinner.Suffix = "🎉 you did it!"
			time.Sleep(250 * time.Millisecond)
			spinner.Stop()
			break
		} else {
			time.Sleep(1 * time.Second)
		}
	}

	CompletedOnboardingShell(user, store)

	s = "\nHit enter to continue:"
	TypeItToMe(s)

	fmt.Print("\n")
	bold := color.New(color.Bold).SprintFunc()
	_ = terminal.PromptGetInput(terminal.PromptContent{
		// Label:      "   " + bold("▸") + "    Press " + bold("Enter") + " to continue",
		Label:      "   " + bold("▸"),
		ErrorMsg:   "error",
		AllowEmpty: true,
	})

	s = "\n\nAwesome! Now try opening VS Code in that environment"
	s += "\nIn a new terminal, try running " + t.Green("brev open %s", firstWorkspace.Name) + " to open VS Code in the dev environment\n"
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
			time.Sleep(250 * time.Millisecond)
			spinner.Stop()
			sum += 1
			break
		} else {
			time.Sleep(1 * time.Second)
		}
	}

	CompletedOnboardingOpen(user, store)

	s = "\nHit enter to continue:"
	TypeItToMe(s)

	fmt.Print("\n")
	_ = terminal.PromptGetInput(terminal.PromptContent{
		// Label:      "   " + bold("▸") + "    Press " + bold("Enter") + " to continue",
		Label:      "   " + bold("▸"),
		ErrorMsg:   "error",
		AllowEmpty: true,
	})

	handleLocalhostURLIfDefaultProject(*firstWorkspace, t)

	s = "\n\nI think I'm done here. Now you know how to open a dev environment and start coding."
	s += "\n\nUse the console " + t.Green("(https://console.brev.dev)") + " to create a new dev environment or share it with people"
	s += "\nand use this CLI to code the way you would normally 🤙"
	s += "\n\nCheck out the docs at " + t.Yellow("https://brev.dev/docs") + " and let us know if we can help!\n"
	s += "\n\nIn case you missed it, my cell is " + t.Yellow("(415) 237-2247") + "\n\t-Nader\n"
	TypeItToMe(s)

	CompletedOnboarding(user, store)
}

func handleLocalhostURLIfDefaultProject(ws entity.Workspace, t *terminal.Terminal) {
	if ws.Name == DEFAULT_DEV_ENV_NAME {
		s := "\n\nOne last thing, since you're coding in the cloud, you can get a public URL to your localhost."
		s += "\nFrom within that Brev dev environment,\n\tRun " + t.Yellow("npm run start") + " to spin up the service"
		s += "\nThen instead of going to localhost:3000, \n\tGo to " + t.Yellow("https://3000-%s", ws.DNS)

		TypeItToMe(s)

		// TODO: Give that a shot then press enter
		bold := color.New(color.Bold).SprintFunc()

		s = "\n\nGive that a shot then press enter👆:"
		TypeItToMe(s)

		fmt.Print("\n")
		_ = terminal.PromptGetInput(terminal.PromptContent{
			// Label:      "   " + bold("▸") + "    Press " + bold("Enter") + " to continue",
			Label:      "   " + bold("▸"),
			ErrorMsg:   "error",
			AllowEmpty: true,
		})

		fmt.Print("\n")
	}
}
