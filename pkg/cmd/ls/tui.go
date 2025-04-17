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

	buttonStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFF7DB")).
		Background(lipgloss.Color("#888B7E")).
		Padding(0, 3).
		MarginRight(2)

	activeButtonStyle = buttonStyle.Copy().
		Foreground(lipgloss.Color("#FFF7DB")).
		Background(lipgloss.Color("#76B900")).
		MarginRight(2).
		Bold(true)

	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		MarginTop(1)
)

type model struct {
	table         table.Model
	workspaces    []entity.Workspace
	selectedIndex int
	showActions   bool
	selectedAction int
	userID        string
	term          *terminal.Terminal
	width         int
	height        int
	store         LsStore
	confirming    bool
	confirmMsg    string
}

// getAvailableActions returns the list of available actions based on workspace status
func (m model) getAvailableActions() []string {
	workspace := m.workspaces[m.selectedIndex]
	status := getWorkspaceDisplayStatus(workspace)
	
	var actions []string
	
	switch status {
	case entity.Running:
		actions = []string{"Stop", "Open Jupyter", "Delete"}
	case entity.Starting:
		actions = []string{"Stop", "Delete"}
	case entity.Stopping:
		actions = []string{"Delete"}
	case entity.Failure, entity.Unhealthy:
		actions = []string{"Delete"}
	default: // Stopped or any other state
		actions = []string{"Start", "Delete"}
	}
	
	return actions
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
			if m.confirming {
				m.confirming = false
				return m, nil
			}
			if m.showActions {
				m.showActions = false
				return m, nil
			}
			return m, tea.Quit
		case "enter":
			if m.confirming {
				actions := m.getAvailableActions()
				selectedWorkspace := m.workspaces[m.selectedIndex]
				
				switch actions[m.selectedAction] {
				case "Start":
					m.term.Vprintf("Starting workspace %s...\n", selectedWorkspace.Name)
					startedWorkspace, err := m.store.StartWorkspace(selectedWorkspace.ID)
					if err != nil {
						m.term.Errprint(err, "Failed to start workspace")
					} else {
						m.term.Vprintf("Instance %s is starting. Run 'brev ls' to check status\n", startedWorkspace.Name)
					}
					return m, tea.Quit
				case "Stop":
					m.term.Vprintf("Stopping workspace %s...\n", selectedWorkspace.Name)
					stoppedWorkspace, err := m.store.StopWorkspace(selectedWorkspace.ID)
					if err != nil {
						m.term.Errprint(err, "Failed to stop workspace")
					} else {
						m.term.Vprintf("Instance %s is stopping. Run 'brev ls' to check status\n", stoppedWorkspace.Name)
					}
					return m, tea.Quit
				case "Delete":
					m.term.Vprintf("Deleting workspace %s...\n", selectedWorkspace.Name)
					deletedWorkspace, err := m.store.DeleteWorkspace(selectedWorkspace.ID)
					if err != nil {
						m.term.Errprint(err, "Failed to delete workspace")
					} else {
						m.term.Vprintf("Deleting instance %s. This can take a few minutes. Run 'brev ls' to check status\n", deletedWorkspace.Name)
					}
					return m, tea.Quit
				}
			} else if m.showActions {
				actions := m.getAvailableActions()
				selectedWorkspace := m.workspaces[m.selectedIndex]
				action := actions[m.selectedAction]
				
				switch action {
				case "Delete":
					m.confirming = true
					m.confirmMsg = fmt.Sprintf("Are you sure you want to delete %s? (Enter to confirm, ESC to cancel)", selectedWorkspace.Name)
				case "Stop":
					m.confirming = true
					m.confirmMsg = fmt.Sprintf("Are you sure you want to stop %s? (Enter to confirm, ESC to cancel)", selectedWorkspace.Name)
				case "Start":
					m.confirming = true
					m.confirmMsg = fmt.Sprintf("Are you sure you want to start %s? (Enter to confirm, ESC to cancel)", selectedWorkspace.Name)
				case "Open Jupyter":
					// No confirmation needed for non-destructive actions
					m.term.Vprintf("Opening Jupyter for workspace %s...\n", selectedWorkspace.Name)
					return m, tea.Quit
				}
			} else {
				m.showActions = true
			}
		case "tab", "right":
			if m.showActions && !m.confirming {
				actions := m.getAvailableActions()
				m.selectedAction = (m.selectedAction + 1) % len(actions)
			}
		case "shift+tab", "left":
			if m.showActions && !m.confirming {
				actions := m.getAvailableActions()
				m.selectedAction = (m.selectedAction - 1 + len(actions)) % len(actions)
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

	// Process table lines and insert buttons after selected row
	lines := strings.Split(m.table.View(), "\n")
	for i, line := range lines {
		if i == m.selectedIndex+2 { // +2 to account for header and 0-based index
			// Add the selected row with caret
			s.WriteString(caretStyle.String() + line + "\n")
			
			// If in action mode, insert buttons right after the selected row
			if m.showActions {
				// Get available actions for current workspace state
				actions := m.getAvailableActions()
				
				// Create buttons with proper styling
				var buttons []string
				for i, action := range actions {
					style := buttonStyle
					if i == m.selectedAction {
						style = activeButtonStyle
					}
					buttons = append(buttons, style.Render(action))
				}
				
				// Add padding to align with content and join buttons
				s.WriteString("  ")
				row := lipgloss.JoinHorizontal(lipgloss.Center, buttons...)
				s.WriteString(row + "\n")

				// If confirming, show confirmation message
				if m.confirming {
					s.WriteString("\n  " + lipgloss.NewStyle().
						Foreground(lipgloss.Color("203")).
						Render(m.confirmMsg) + "\n")
				}
			}
		} else {
			s.WriteString("  " + line + "\n") // Add padding to align non-selected rows
		}
	}

	// Add help text at the bottom
	if m.confirming {
		s.WriteString(helpStyle.Render("  Press Enter to confirm • ESC to cancel"))
	} else if m.showActions {
		s.WriteString(helpStyle.Render("  Press ESC to go back • Enter to select action"))
	} else {
		s.WriteString(helpStyle.Render("  Press Enter for actions • q to quit"))
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

func initialModel(workspaces []entity.Workspace, term *terminal.Terminal, userID string, store LsStore) model {
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
		userID:     userID,
		term:       term,
		store:      store,
		confirming: false,
	}
}

func RunInteractiveLs(t *terminal.Terminal, workspaces []entity.Workspace, userID string, store LsStore) error {
	m := initialModel(workspaces, t, userID, store)
	p := tea.NewProgram(m, tea.WithAltScreen())
	
	_, err := p.Run()
	return err
} 