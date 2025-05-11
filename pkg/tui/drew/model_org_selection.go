package drew

import (
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// NewOrgSelection creates a new organization pick list model.
func NewOrgSelection() *OrgSelection {
	orgSelection := &OrgSelection{}

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

	// Create a new list with no data yet
	list := list.New([]list.Item{}, delegate, 0, 0)

	// Style the organization pick list
	list.Title = "Select Organization"
	list.Styles.Title = lipgloss.NewStyle().
		Background(backgroundColorHeader).
		Foreground(textColorHeader).
		Bold(true)

	list.SetShowStatusBar(false)
	list.SetStatusBarItemName("organization", "organizations")
	list.SetFilteringEnabled(false)
	list.SetShowHelp(true)
	list.DisableQuitKeybindings()
	list.SetSpinner(spinner.Points)

	orgSelection.orgPickListModel = list
	return orgSelection
}

// OrgSelection is a model that represents the organization pick list. Note that this is not a complete
// charmbracelet/bubbles/list.Model, but rather a wrapper around it that adds some additional functionality
// while allowing for simplified use of the wrapped list.Model.
type OrgSelection struct {
	orgPickListModel list.Model
	orgSelected      *orgListItem
}

func (o *OrgSelection) SetHeight(height int) {
	o.orgPickListModel.SetHeight(height)
}

func (o *OrgSelection) SetWidth(width int) {
	o.orgPickListModel.SetWidth(width)
}

// Selection returns the currently selected organization.
func (o *OrgSelection) Selection() *orgListItem {
	return o.orgSelected
}

// Width returns the width of the organization pick list.
func (o *OrgSelection) Width() int {
	return o.orgPickListModel.Width()
}

// Height returns the height of the organization pick list.
func (o *OrgSelection) Height() int {
	return o.orgPickListModel.Height()
}

func (e *OrgSelection) HelpTextEntries() [][]string {
	return [][]string{
		{"o/q/esc", "close window"},
	}
}

type orgListItem struct {
	Organization organization
}

func (i orgListItem) Title() string       { return i.Organization.Name }
func (i orgListItem) Description() string { return i.Organization.Description }
func (i orgListItem) FilterValue() string { return i.Organization.Name }

type (
	// OrgSelectionErrorMsg is a message that indicates an error occurred while fetching organizations.
	OrgSelectionErrorMsg struct{ err error }

	// CloseOrgSelectionMsg is a message that indicates the organization pick list should be closed.
	CloseOrgSelectionMsg struct{}
)

func orgSelectionErrorCmd(err error) tea.Cmd {
	return func() tea.Msg { return OrgSelectionErrorMsg{err} }
}
func orgSelectionCloseCmd() tea.Cmd { return func() tea.Msg { return CloseOrgSelectionMsg{} } }

func (o *OrgSelection) View() string {
	return o.orgPickListModel.View()
}

func (o *OrgSelection) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case spinner.TickMsg:
		// The org pick list spinner is still running, so we need to update the org pick list model to render the next frame
		o.orgPickListModel, cmd = o.orgPickListModel.Update(msg)
		return cmd

	case fetchOrgsMsg:
		// The orgs have been fetched, so we need to update the org pick list model
		if msg.err != nil {
			return orgSelectionErrorCmd(msg.err)
		}

		// Insert the orgs into the org pick list model
		pickListItems := make([]list.Item, len(msg.organizations))
		for i, org := range msg.organizations {
			pickListItems[i] = orgListItem{Organization: org}
		}

		// Update the org pick list model with the new items
		updatePickListCmd := o.orgPickListModel.SetItems(pickListItems)
		if len(pickListItems) > 0 {
			o.orgPickListModel.SetShowStatusBar(true)
		}
		o.orgPickListModel.StopSpinner()

		return updatePickListCmd

	case tea.KeyMsg:
		switch msg.String() {

		// Close the org list
		case "esc", "o", "q":
			return orgSelectionCloseCmd()

		// Select an org
		case "enter":
			if selected, ok := o.orgPickListModel.SelectedItem().(orgListItem); ok {
				o.orgSelected = &selected
				return orgSelectionCloseCmd()
			}

		// For all other key events, pass them to the org pick list model
		default:
			o.orgPickListModel, cmd = o.orgPickListModel.Update(msg)
		}
	}

	return cmd
}

// FetchOrgs fetches the organizations and updates the org pick list model. This function automatically
// starts the spinner and returns a command that will update the org pick list model when the organizations
// are fetched. The returned command should be used to render the next frame for the spinner, and should
// also be used to update the org pick list model when the organizations are fetched.
func (o *OrgSelection) FetchOrgs() tea.Cmd {
	// Start the spinner
	startSpinnerCmd := o.orgPickListModel.StartSpinner()

	// Fetch the organizations
	fetchOrgsCmd := cmdFetchOrgs()

	return tea.Batch(startSpinnerCmd, fetchOrgsCmd)
}

type organization struct {
	ID          string
	Name        string
	Description string
}

type fetchOrgsMsg struct {
	organizations []organization
	err           error
}

func cmdFetchOrgs() tea.Cmd {
	return func() tea.Msg {
		organizations := fetchOrgs()

		// Sort the organizations by ID
		sort.Slice(organizations, func(i, j int) bool {
			return organizations[i].ID < organizations[j].ID
		})

		return fetchOrgsMsg{organizations: organizations, err: nil}
	}
}

func fetchOrgs() []organization {
	// simulate loading
	time.Sleep(time.Second * 1)

	return []organization{
		{ID: "1", Name: "Organization 1", Description: "First organization"},
		{ID: "2", Name: "Organization 2", Description: "Second organization"},
		{ID: "3", Name: "Organization 3", Description: "Third organization"},
		{ID: "4", Name: "Organization 4", Description: "Fourth organization"},
		{ID: "5", Name: "Organization 5", Description: "Fifth organization"},
		{ID: "6", Name: "Organization 6", Description: "Sixth organization"},
		{ID: "7", Name: "Organization 7", Description: "Seventh organization"},
		{ID: "8", Name: "Organization 8", Description: "Eighth organization"},
		{ID: "9", Name: "Organization 9", Description: "Ninth organization"},
		{ID: "10", Name: "Organization 10", Description: "Tenth organization"},
		{ID: "11", Name: "Organization 11", Description: "Eleventh organization"},
		{ID: "12", Name: "Organization 12", Description: "Twelfth organization"},
	}
}
