package drew

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
)

var (
	keywordStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Background(lipgloss.Color("235"))
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	titleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	infoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		// b.Left = "┤"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	footerStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "├"
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("204")).
			Border(lipgloss.NormalBorder()).
			BorderTop(true).BorderBottom(false).BorderLeft(false).BorderRight(false).
			BorderForeground(lipgloss.Color("241")).
			Height(1)
	}()
)

type MainModel struct {
	// General states
	quitting   bool
	suspending bool

	// General viewport
	viewport viewport.Model

	// Org list Modal
	renderOrgPickList bool
	orgSelection      *OrgSelection
}

func (m *MainModel) View() string {
	if m.quitting {
		return "Quitting..."
	}
	if m.suspending {
		return ""
	}

	var content string
	if m.renderOrgPickList {
		// We are rendering the org pick list modal, which should be centered in the viewport
		// TODO: figure out how to render this "on top" of the viewport, rather than replacing it
		h := m.orgSelection.Height()
		w := m.orgSelection.Width()
		marginTop := (m.viewport.Height / 2) - (h / 2)
		marginLeft := (m.viewport.Width / 2) - (w / 2)
		marginBottom := m.viewport.Height - marginTop - h - 2
		content = lipgloss.NewStyle().
			Height(h).
			Width(w).
			MarginTop(marginTop).
			MarginLeft(marginLeft).
			MarginBottom(marginBottom).
			Border(lipgloss.RoundedBorder()).
			Render(m.orgSelection.View())
	} else {
		content = m.viewport.View()
	}

	/**
	 * Render the main view, which is always:
	 *
	 * [header]
	 * [content]
	 * [footer]
	 */
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), content, m.footerView())
}

func (m *MainModel) headerView() string {
	titleStr := "NVIDIA Brev 🤙"
	if m.orgSelection.Selection() != nil {
		titleStr = titleStr + " | " + m.orgSelection.Selection().Title()
	}
	title := titleStyle.Render(titleStr)
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m *MainModel) footerView() string {
	help := helpStyle.Render("q/esc: exit • o: select org")
	return footerStyle.Width(m.viewport.Width).Render(
		help,
	)
}

func (m *MainModel) Init() tea.Cmd {
	m.orgSelection = NewOrgSelection()

	return nil
}

func (m *MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle messages that are common to any view mode
	switch msg := msg.(type) {
	case tea.ResumeMsg:

		// Allow for resuming the program, if it was suspended to the background
		m.suspending = false
		return m, nil
	case tea.QuitMsg:

		// Handle quitting the program
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {

		// Mark the program as having been suspended
		case "ctrl+z":
			m.suspending = true
			return m, tea.Suspend

		// Allow for quitting, even when the org list modal is open
		case "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:

		// Update the model's viewport on window size change
		m.onWindowSizeChange(msg)
	}

	var cmd tea.Cmd

	if m.renderOrgPickList {
		// We are currently rendering the org pick list modal -- handle messages from and for its model
		switch msg := msg.(type) {
		case CloseOrgSelectionMsg:
			// Close the org pick list modal without further processing
			m.renderOrgPickList = false
			return m, nil
		case OrgSelectionErrorMsg:
			// If there was an error fetching the orgs, quit the program
			return m, tea.Quit // TODO: display the error or retry?
		default:
			// By default, pass the message to the org pick list model
			cmd = m.orgSelection.Update(msg)
			return m, cmd
		}
	} else {
		// We are not rendering the org pick list modal -- handle messages from and for the viewport
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc", "q":
				// Quit the program
				return m, tea.Quit
			case "o":
				// Indicate that we want to render the org pick list modal, and trigger the fetching of orgs
				m.renderOrgPickList = true
				cmd = m.orgSelection.FetchOrgs()

				return m, cmd
			}
		default:
			// By default, pass the message to the viewport
			m.viewport, cmd = m.viewport.Update(msg)
		}
		return m, tea.Batch(cmd)
	}
}

func (m *MainModel) onWindowSizeChange(msg tea.WindowSizeMsg) {
	headerHeight := lipgloss.Height(m.headerView())
	footerHeight := lipgloss.Height(m.footerView())
	verticalMarginHeight := headerHeight + footerHeight

	m.viewport.Width = msg.Width
	m.viewport.Height = msg.Height - verticalMarginHeight
}
