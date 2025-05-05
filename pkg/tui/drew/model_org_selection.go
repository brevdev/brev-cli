package drew

import (
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// NewOrgSelection creates a new organization pick list model.
func NewOrgSelection() *OrgSelection {
	// Create a custom delegate that doesn't quit on escape
	delegate := list.NewDefaultDelegate()

	// Create a new list with no data yet
	l := list.New([]list.Item{}, delegate, 50, 30)

	// Style the organization pick list
	l.Title = "Select Organization"
	l.SetShowStatusBar(true)
	l.SetStatusBarItemName("organization", "organizations")
	l.SetFilteringEnabled(false)
	l.SetShowHelp(true)
	l.DisableQuitKeybindings()
	l.SetSpinner(spinner.Points)

	return &OrgSelection{orgPickListModel: l}
}

// OrgSelection is a model that represents the organization pick list. Note that this is not a complete
// charmbracelet/bubbles/list.Model, but rather a wrapper around it that adds some additional functionality
// while allowing for simplified use of the wrapped list.Model.
type OrgSelection struct {
	orgPickListModel list.Model
	orgSelected      *orgListItem
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

type orgListItem struct {
	title, desc string
}

func (i orgListItem) Title() string       { return i.title }
func (i orgListItem) Description() string { return i.desc }
func (i orgListItem) FilterValue() string { return i.title }

type (
	// OrgSelectionErrorMsg is a message that indicates an error occurred while fetching organizations.
	OrgSelectionErrorMsg struct{ err error }

	// CloseOrgSelectionMsg is a message that indicates the organization pick list should be closed.
	CloseOrgSelectionMsg struct{}
)

func errorCmd(err error) tea.Cmd { return func() tea.Msg { return OrgSelectionErrorMsg{err} } }
func closeCmd() tea.Cmd          { return func() tea.Msg { return CloseOrgSelectionMsg{} } }

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
			return errorCmd(msg.err)
		}

		// Insert the orgs into the org pick list model
		pickListItems := make([]list.Item, len(msg.organizations))
		for i, org := range msg.organizations {
			pickListItems[i] = orgListItem{title: org.Name, desc: org.Description}
		}

		// Update the org pick list model with the new items
		updatePickListCmd := o.orgPickListModel.SetItems(pickListItems)
		o.orgPickListModel.StopSpinner()

		return updatePickListCmd

	case tea.KeyMsg:
		switch msg.String() {

		// Close the org list
		case "esc", "o", "q":
			return closeCmd()

		// Select an org
		case "enter":
			if selected, ok := o.orgPickListModel.SelectedItem().(orgListItem); ok {
				o.orgSelected = &selected
				return closeCmd()
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
	startSpinnerCmd := o.orgPickListModel.StartSpinner()
	fetchOrgsCmd := cmdFetchOrgs()
	return tea.Batch(startSpinnerCmd, fetchOrgsCmd)
}

type organization struct {
	Name        string
	Description string
}

type fetchOrgsMsg struct {
	organizations []organization
	err           error
}

func cmdFetchOrgs() tea.Cmd {
	return func() tea.Msg {
		// simulate loading
		time.Sleep(time.Second * 3)

		return fetchOrgsMsg{organizations: []organization{
			{Name: "Organization 1", Description: "First organization"},
			{Name: "Organization 2", Description: "Second organization"},
			{Name: "Organization 3", Description: "Third organization"},
			{Name: "Organization 4", Description: "Fourth organization"},
			{Name: "Organization 5", Description: "Fifth organization"},
			{Name: "Organization 6", Description: "Sixth organization"},
			{Name: "Organization 7", Description: "Seventh organization"},
			{Name: "Organization 8", Description: "Eighth organization"},
			{Name: "Organization 9", Description: "Ninth organization"},
			{Name: "Organization 10", Description: "Tenth organization"},
			{Name: "Organization 11", Description: "Eleventh organization"},
			{Name: "Organization 12", Description: "Twelfth organization"},
		}, err: nil}
	}
}
