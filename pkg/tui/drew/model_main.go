package drew

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
)

var (
	keywordStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Background(lipgloss.Color("235"))
	helpStyleDark  = lipgloss.NewStyle().Foreground(lipgloss.Color("239"))
	helpStyleLight = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))

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
	// viewport viewport.Model

	// Org list Modal
	renderOrgPickList bool
	orgSelection      *OrgSelection
	envSelection      *EnvSelection
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
		// h := m.orgSelection.Height()
		// w := m.orgSelection.Width()
		// marginTop := (m.envSelection.Height() / 2) - (h / 2)
		// marginLeft := (m.envSelection.Width() / 2) - (w / 2)
		// marginBottom := m.envSelection.Height() - marginTop - h - 2
		content = lipgloss.NewStyle().
			Align(lipgloss.Center).
			AlignVertical(lipgloss.Center).
			Height(m.envSelection.Height()). // match background height
			Width(m.envSelection.Width()).   // match background width
			Render(
				lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					Padding(1, 2).
					Render(m.orgSelection.View()),
			)
	} else {
		content = m.envSelection.View()
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
	line := strings.Repeat("─", max(0, m.envSelection.Width()-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m *MainModel) footerView() string {
	helpTextEntries := []string{}
	if m.renderOrgPickList {
		helpTextEntries = append(helpTextEntries, helpStyleLight.Render("o/q/esc")+" "+helpStyleDark.Render("close window"))
	} else if m.orgSelection.Selection() != nil {
		helpTextEntries = append(helpTextEntries, helpStyleLight.Render("q/esc")+" "+helpStyleDark.Render("exit"))
		helpTextEntries = append(helpTextEntries, helpStyleLight.Render("o")+" "+helpStyleDark.Render("select org"))
		helpTextEntries = append(helpTextEntries, helpStyleLight.Render("↑/k")+" "+helpStyleDark.Render("up"))
		helpTextEntries = append(helpTextEntries, helpStyleLight.Render("↓/j")+" "+helpStyleDark.Render("down"))
	} else {
		helpTextEntries = append(helpTextEntries, helpStyleLight.Render("q/esc")+" "+helpStyleDark.Render("exit"))
		helpTextEntries = append(helpTextEntries, helpStyleLight.Render("o")+" "+helpStyleDark.Render("select org"))
	}

	// Join the help text entries with a " • " separator
	helpText := strings.Join(helpTextEntries, helpStyleDark.Render(" • "))

	return footerStyle.Width(m.envSelection.Width()).Render(helpText)
}

type initMsg struct{}

func (m *MainModel) initCmd() tea.Cmd {
	return func() tea.Msg { return initMsg{} }
}

func (m *MainModel) Init() tea.Cmd {
	m.orgSelection = NewOrgSelection()
	m.envSelection = NewEnvSelection()

	// TODO: if not default org is found (read from ~/.brev), submit the init command
	cmd := m.initCmd()
	return cmd
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
		return m, nil

	case initMsg:

		// If the program is being initialized, render the org pick list modal and fetch the orgs
		m.renderOrgPickList = true
		cmd := m.orgSelection.FetchOrgs()
		return m, cmd
	}

	if m.renderOrgPickList {
		// We are currently rendering the org pick list modal -- handle messages from and for its model
		switch msg := msg.(type) {
		case CloseOrgSelectionMsg:

			// Close the org pick list modal without further processing
			m.renderOrgPickList = false
			if m.orgSelection.Selection() != nil {
				cmd := m.envSelection.FetchEnvs(m.orgSelection.Selection().Organization.ID)
				return m, cmd
			}
			return m, nil
		case OrgSelectionErrorMsg:

			// If there was an error fetching the orgs, quit the program
			return m, tea.Quit // TODO: display the error or retry?
		default:

			// By default, pass the message to the org pick list model
			cmd := m.orgSelection.Update(msg)
			return m, cmd
		}
	}

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
			cmd := m.orgSelection.FetchOrgs()
			return m, cmd
		}
	}

	// By default, pass the message to the env selection model
	cmd := m.envSelection.Update(msg)
	return m, cmd
}

func (m *MainModel) onWindowSizeChange(msg tea.WindowSizeMsg) {
	headerHeight := lipgloss.Height(m.headerView())
	footerHeight := lipgloss.Height(m.footerView())
	contentHeight := msg.Height - headerHeight - footerHeight

	m.envSelection.SetWidth(msg.Width)
	m.envSelection.SetHeight(contentHeight)

	m.orgSelection.SetWidth(min(msg.Width, 30))
	m.orgSelection.SetHeight(min(contentHeight-4, 30)) // keep a small amount of padding for the height
}
