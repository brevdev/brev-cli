package ls

import (
	"fmt"
	"strings"

	utilities "github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	baseStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240"))

	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#76B900")).
		MarginBottom(1)

	caretStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#76B900")).
		SetString("❯ ")

	actionButtonStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1).
			MarginRight(2)

	selectedActionButtonStyle = actionButtonStyle.Copy().
				BorderForeground(lipgloss.Color("#76B900")).
				BorderStyle(lipgloss.DoubleBorder()).
				Foreground(lipgloss.Color("#76B900"))
)

type model struct {
	table         table.Model
	workspaces    []entity.Workspace
	selectedIndex int
	showActions   bool
	selectedAction int
	actions       []string
	userID        string
	term          *terminal.Terminal
	width         int
	height        int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			if m.showActions {
				m.showActions = false
				return m, nil
			}
			return m, tea.Quit
		case "enter":
			if m.showActions {
				switch m.selectedAction {
				case 0: // Start
					return m, tea.Quit
				case 1: // Stop
					return m, tea.Quit
				case 2: // Jupyter
					return m, tea.Quit
				}
			} else {
				m.showActions = true
			}
		case "tab", "right":
			if m.showActions {
				m.selectedAction = (m.selectedAction + 1) % len(m.actions)
			}
		case "shift+tab", "left":
			if m.showActions {
				m.selectedAction = (m.selectedAction - 1 + len(m.actions)) % len(m.actions)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(msg.Width - 4)
		m.table.SetHeight(msg.Height - 6)
	}

	if !m.showActions {
		m.table, cmd = m.table.Update(msg)
		m.selectedIndex = m.table.Cursor()
	}
	return m, cmd
}

func (m model) View() string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("Workspaces"))
	s.WriteString("\n")

	if m.showActions {
		// Show actions for the selected workspace
		workspace := m.workspaces[m.selectedIndex]
		s.WriteString(fmt.Sprintf("Actions for workspace: %s\n\n", workspace.Name))
		
		for i, action := range m.actions {
			style := actionButtonStyle
			if i == m.selectedAction {
				style = selectedActionButtonStyle
			}
			s.WriteString(style.Render(action))
		}
		s.WriteString("\n\n")
		s.WriteString("Press ESC to go back • Enter to select action")
	} else {
		// Add caret to the selected row
		lines := strings.Split(m.table.View(), "\n")
		for i, line := range lines {
			if i == m.selectedIndex+2 { // +2 to account for header and 0-based index
				lines[i] = caretStyle.String() + line
			} else {
				lines[i] = "  " + line // Add padding to align non-selected rows
			}
		}
		s.WriteString(strings.Join(lines, "\n"))
		s.WriteString("\n")
		s.WriteString("Press Enter for actions • q to quit")
	}

	return baseStyle.Render(s.String())
}

func getTableRows(workspaces []entity.Workspace, t *terminal.Terminal, userID string) []table.Row {
	var rows []table.Row
	for _, w := range workspaces {
		isShared := ""
		if w.IsShared(userID) {
			isShared = "(shared)"
		}
		status := getWorkspaceDisplayStatus(w)
		instanceString := utilities.GetInstanceString(w)
		rows = append(rows, table.Row{
			fmt.Sprintf("%s %s", w.Name, isShared),
			getStatusColoredText(t, status),
			getStatusColoredText(t, string(w.VerbBuildStatus)),
			getStatusColoredText(t, getShellDisplayStatus(w)),
			w.ID,
			instanceString,
		})
	}
	return rows
}

func initialModel(workspaces []entity.Workspace, term *terminal.Terminal, userID string) model {
	columns := []table.Column{
		{Title: "Name", Width: 20},
		{Title: "Status", Width: 10},
		{Title: "Build", Width: 10},
		{Title: "Shell", Width: 10},
		{Title: "ID", Width: 12},
		{Title: "Machine", Width: 25},
	}

	rows := getTableRows(workspaces, term, userID)

	tableModel := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(7),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	
	// Use a subtle background color for selected row
	s.Selected = s.Selected.
		Background(lipgloss.Color("236"))

	tableModel.SetStyles(s)

	return model{
		table:      tableModel,
		workspaces: workspaces,
		actions:    []string{"Start", "Stop", "Open Jupyter"},
		userID:     userID,
		term:       term,
	}
}

func RunInteractiveLs(t *terminal.Terminal, workspaces []entity.Workspace, userID string) error {
	m := initialModel(workspaces, t, userID)
	p := tea.NewProgram(m, tea.WithAltScreen())
	
	_, err := p.Run()
	return err
} 