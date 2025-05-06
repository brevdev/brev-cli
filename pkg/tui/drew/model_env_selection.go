package drew

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
)

// NewEnvSelection creates a new environment pick list model.
func NewEnvSelection() *EnvSelection {
	envSelection := &EnvSelection{}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(textColorNormalTitle).
		Padding(0, 0, 0, 2)

	delegate.Styles.NormalDesc = delegate.Styles.NormalTitle.
		Foreground(textColorNormalDescription)

	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(borderColorSelected).
		Foreground(textColorSelectedTitle).
		Padding(0, 0, 0, 1)

	delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle.
		Foreground(textColorSelectedDescription)

	delegate.Styles.DimmedTitle = lipgloss.NewStyle().
		Foreground(textColorDimmedTitle).
		Padding(0, 0, 0, 2)

	delegate.Styles.DimmedDesc = delegate.Styles.DimmedTitle.
		Foreground(textColorDimmedDescription)

	delegate.Styles.FilterMatch = lipgloss.NewStyle().Underline(true)

	list := list.New([]list.Item{}, delegate, 40, 20)

	list.SetShowStatusBar(false)
	list.SetShowTitle(false)
	list.SetStatusBarItemName("environment", "environments")
	list.SetFilteringEnabled(false)
	list.SetShowHelp(false)
	list.DisableQuitKeybindings()

	envSpinner := spinner.New()
	envSpinner.Spinner = spinner.Points
	envSpinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748b"))

	envSelection.loadingSpinner = envSpinner
	envSelection.envList = list
	return envSelection
}

// EnvSelection is a model that represents the environment pick list. Note that this is not a complete
// charmbracelet/bubbles/list.Model, but rather a wrapper around it that adds some additional functionality
// while allowing for simplified use of the wrapped list.
type EnvSelection struct {
	envList     list.Model
	envSelected *environment

	showLoadingSpinner bool
	loadingSpinner     spinner.Model
}

// Selection returns the currently selected environment.
func (e *EnvSelection) Selection() *environment {
	return e.envSelected
}

// Width returns the width of the organization pick list.
func (e *EnvSelection) Width() int {
	return e.envList.Width()
}

func (e *EnvSelection) SetWidth(width int) {
	e.envList.SetWidth(width)
}

// Height returns the height of the organization pick list.
func (e *EnvSelection) Height() int {
	return e.envList.Height()
}

func (e *EnvSelection) SetHeight(height int) {
	e.envList.SetHeight(height)
}

type envListItem struct {
	environment environment
}

func (e envListItem) Title() string { return fmt.Sprintf("%s", e.environment.Name) }
func (e envListItem) Description() string {
	return fmt.Sprintf("%s • %s • %s", e.environment.GPU, e.environment.CPU, e.environment.RAM)
}
func (e envListItem) FilterValue() string { return e.environment.Name }

type (
	// EnvSelectionErrorMsg is a message that indicates an error occurred while fetching environments.
	EnvSelectionErrorMsg struct{ err error }
)

func envSelectionErrorCmd(err error) tea.Cmd {
	return func() tea.Msg { return EnvSelectionErrorMsg{err} }
}

func (e *EnvSelection) View() string {
	if e.showLoadingSpinner {
		spinner := fmt.Sprintf("Loading environments %s", e.loadingSpinner.View())

		// Create a vertically centered spinner box with full height
		loadingBox := lipgloss.NewStyle().
			Height(e.envList.Height()). // Match the table height
			Width(e.envList.Width()).   // Match the table width
			Align(lipgloss.Center).
			AlignVertical(lipgloss.Center).
			Render(spinner)

		return loadingBox
	}

	var selected *environment
	if e.envList.SelectedItem() == nil {
		selected = nil
	} else {
		if selectedItem, ok := e.envList.SelectedItem().(envListItem); ok {
			selected = &selectedItem.environment
		} else {
			selected = nil
		}
	}

	left := lipgloss.NewStyle().
		Width(e.envList.Width() / 2).
		Render(e.envList.View())

	right := lipgloss.NewStyle().
		Width(e.envList.Width()/2).
		Padding(1, 0, 0, 1).
		Border(lipgloss.RoundedBorder()).
		Render(renderEnvDetails(selected))

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func renderEnvDetails(environment *environment) string {
	if environment == nil {
		return ""
	}
	return lipgloss.NewStyle().
		// Border(lipgloss.RoundedBorder()).
		// BorderForeground(lipgloss.Color("#76b900")).
		Padding(1, 2).
		Width(60).
		Render(fmt.Sprintf(`
ID:     %s
Name:   %s
GPU:    %s
CPU:    %s vCPU
RAM:    %s
Status: %s
`, environment.ID, environment.Name, environment.GPU, environment.CPU, environment.RAM, environment.Status))
}

func (e *EnvSelection) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	case fetchEnvsMsg:
		// The orgs have been fetched, so we need to update the org pick list model

		// Disable the loading spinner
		e.showLoadingSpinner = false

		if msg.err != nil {
			return envSelectionErrorCmd(msg.err)
		}

		// Insert the orgs into the org pick list model
		envListItems := make([]list.Item, len(msg.environments))
		for i, env := range msg.environments {
			envListItems[i] = envListItem{environment: env}
		}

		// Update the env pick list model with the new items
		e.envList.SetItems(envListItems)

		return nil

	case tea.KeyMsg:
		// Pass the key event to the env pick list model
		e.envList, cmd = e.envList.Update(msg)
		return cmd

	default:
		// If the loading spinner is enabled, update it
		if e.showLoadingSpinner {
			e.loadingSpinner, cmd = e.loadingSpinner.Update(msg)
			return cmd
		}
		return nil
	}
}

// FetchEnvs fetches the environments and updates the env pick list model. This function automatically
// starts the spinner and returns a command that will update the env pick list model when the environments
// are fetched. The returned command should be used to render the next frame for the spinner, and should
// also be used to update the env pick list model when the environments are fetched.
func (e *EnvSelection) FetchEnvs(organizationID string) tea.Cmd {
	e.envList.SetItems([]list.Item{})

	// Fetch the organizations
	fetchEnvsCmd := cmdFetchEnvs(organizationID)

	// Start the spinner
	e.showLoadingSpinner = true
	spinnerCmd := e.loadingSpinner.Tick

	return tea.Batch(fetchEnvsCmd, spinnerCmd)
}

type environment struct {
	ID   string
	Name string
	GPU  string
	CPU  string
	RAM  string
	Status string
}

type fetchEnvsMsg struct {
	environments []environment
	err          error
}

func cmdFetchEnvs(organizationID string) tea.Cmd {
	return func() tea.Msg {
		// simulate loading
		time.Sleep(time.Second * 2)

		return fetchEnvsMsg{environments: []environment{
			{ID: "1", Name: "Environment 1", GPU: "NVIDIA A100", CPU: "Intel(R) Xeon(R) CPU @ 2.20GHz", RAM: "128GB", Status: "Running"},
			{ID: "2", Name: "Environment 2", GPU: "NVIDIA A100", CPU: "Intel(R) Xeon(R) CPU @ 2.20GHz", RAM: "128GB", Status: "Running"},
			{ID: "3", Name: "Environment 3", GPU: "NVIDIA A100", CPU: "Intel(R) Xeon(R) CPU @ 2.20GHz", RAM: "128GB", Status: "Running"},
			{ID: "4", Name: "Environment 4", GPU: "NVIDIA A100", CPU: "Intel(R) Xeon(R) CPU @ 2.20GHz", RAM: "128GB", Status: "Running"},
			{ID: "5", Name: "Environment 5", GPU: "NVIDIA A100", CPU: "Intel(R) Xeon(R) CPU @ 2.20GHz", RAM: "128GB", Status: "Running"},
			{ID: "6", Name: "Environment 6", GPU: "NVIDIA A100", CPU: "Intel(R) Xeon(R) CPU @ 2.20GHz", RAM: "128GB", Status: "Running"},
			{ID: "7", Name: "Environment 7", GPU: "NVIDIA A100", CPU: "Intel(R) Xeon(R) CPU @ 2.20GHz", RAM: "128GB", Status: "Running"},
			{ID: "8", Name: "Environment 8", GPU: "NVIDIA A100", CPU: "Intel(R) Xeon(R) CPU @ 2.20GHz", RAM: "128GB", Status: "Running"},
			{ID: "9", Name: "Environment 9", GPU: "NVIDIA A100", CPU: "Intel(R) Xeon(R) CPU @ 2.20GHz", RAM: "128GB", Status: "Running"},
			{ID: "10", Name: "Environment 10", GPU: "NVIDIA A100", CPU: "Intel(R) Xeon(R) CPU @ 2.20GHz", RAM: "128GB", Status: "Running"},
			{ID: "11", Name: "Environment 11", GPU: "NVIDIA A100", CPU: "Intel(R) Xeon(R) CPU @ 2.20GHz", RAM: "128GB", Status: "Running"},
			{ID: "12", Name: "Environment 12", GPU: "NVIDIA A100", CPU: "Intel(R) Xeon(R) CPU @ 2.20GHz", RAM: "128GB", Status: "Running"},
		}, err: nil}
	}
}
