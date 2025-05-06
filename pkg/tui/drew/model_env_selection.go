package drew

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
)

// NewEnvSelection creates a new environment pick list model.
func NewEnvSelection() *EnvSelection {
	envSelection := &EnvSelection{}

	columns := []table.Column{
		{Title: "ID", Width: 10},
		{Title: "Name", Width: 20},
		{Title: "Description", Width: 30},
	}
	rows := []table.Row{}

	envTable := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
	)
	envTable.SetStyles(table.Styles{
		Header: lipgloss.NewStyle().
			Foreground(textColorNormalTitle).
			Bold(true).
			Padding(0, 0, 0, 2),
		Selected: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(borderColorSelected).
			Foreground(textColorSelectedTitle).
			Padding(0, 0, 0, 1),
		Cell: table.DefaultStyles().Cell,
	})

	envSpinner := spinner.New()
	envSpinner.Spinner = spinner.Points
	envSpinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748b"))

	envSelection.loadingSpinner = envSpinner
	envSelection.envTable = envTable
	return envSelection
}

// EnvSelection is a model that represents the environment pick list. Note that this is not a complete
// charmbracelet/bubbles/list.Model, but rather a wrapper around it that adds some additional functionality
// while allowing for simplified use of the wrapped list.
type EnvSelection struct {
	envTable    table.Model
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
	return e.envTable.Width()
}

func (e *EnvSelection) SetWidth(width int) {
	e.envTable.SetWidth(width)
}

// Height returns the height of the organization pick list.
func (e *EnvSelection) Height() int {
	return e.envTable.Height()
}

func (e *EnvSelection) SetHeight(height int) {
	e.envTable.SetHeight(height)
}

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
			Height(e.envTable.Height()). // Match the table height
			Width(e.envTable.Width()).   // Match the table width
			Align(lipgloss.Center).
			AlignVertical(lipgloss.Center).
			Render(spinner)

		return loadingBox
	}

	selected := e.envTable.SelectedRow()

	left := lipgloss.NewStyle().
		Width(e.envTable.Width() / 2).
		Render(e.envTable.View())

	right := lipgloss.NewStyle().
		Width(e.envTable.Width()/2).
		Padding(1, 0, 0, 1).
		Border(lipgloss.RoundedBorder()).
		Render(renderEnvDetails(selected))

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func renderEnvDetails(selected []string) string {
	if len(selected) == 0 {
		return ""
	}
	return fmt.Sprintf("ID: %s\nName: %s\nDescription: %s", selected[0], selected[1], selected[2])
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
		envTableItems := make([]table.Row, len(msg.environments))
		for i, env := range msg.environments {
			envTableItems[i] = table.Row{env.ID, env.Name, env.Description}
		}

		// Update the env pick list model with the new items
		e.envTable.SetRows(envTableItems)

		return nil

	case tea.KeyMsg:
		// Pass the key event to the env pick list model
		e.envTable, cmd = e.envTable.Update(msg)
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
	e.envTable.SetRows([]table.Row{})

	// Fetch the organizations
	fetchEnvsCmd := cmdFetchEnvs(organizationID)

	// Start the spinner
	e.showLoadingSpinner = true
	spinnerCmd := e.loadingSpinner.Tick

	return tea.Batch(fetchEnvsCmd, spinnerCmd)
}

type environment struct {
	ID          string
	Name        string
	Description string
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
			{ID: "1", Name: "Environment 1", Description: "First environment"},
			{ID: "2", Name: "Environment 2", Description: "Second environment"},
			{ID: "3", Name: "Environment 3", Description: "Third environment"},
			{ID: "4", Name: "Environment 4", Description: "Fourth environment"},
			{ID: "5", Name: "Environment 5", Description: "Fifth environment"},
			{ID: "6", Name: "Environment 6", Description: "Sixth environment"},
			{ID: "7", Name: "Environment 7", Description: "Seventh environment"},
			{ID: "8", Name: "Environment 8", Description: "Eighth environment"},
			{ID: "9", Name: "Environment 9", Description: "Ninth environment"},
			{ID: "10", Name: "Environment 10", Description: "Tenth environment"},
			{ID: "11", Name: "Environment 11", Description: "Eleventh environment"},
			{ID: "12", Name: "Environment 12", Description: "Twelfth environment"},
		}, err: nil}
	}
}
