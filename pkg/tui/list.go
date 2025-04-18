package tui

import (
	"fmt"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	baseStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(nvidiaGreen))

	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(nvidiaGreen)).
		MarginBottom(1)

	caretStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(nvidiaGreen)).
		SetString("❯ ")

	buttonStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFF7DB")).
		Background(lipgloss.Color("#888B7E")).
		Padding(0, 3).
		MarginRight(2)

	activeButtonStyle = buttonStyle.Copy().
		Foreground(lipgloss.Color("#FFF7DB")).
		Background(lipgloss.Color(nvidiaGreen)).
		MarginRight(2).
		Bold(true)
)

type listModel struct {
	table          table.Model
	workspaces     []entity.Workspace
	selectedIndex  int
	showActions    bool
	selectedAction int
	userID         string
	store          *store.AuthHTTPStore
	confirming     bool
	confirmMsg     string
	loading        bool
	err            error
}

func newListModel() listModel {
	columns := []table.Column{
		{Title: "NAME", Width: 20},
		{Title: "STATUS", Width: 15},
		{Title: "INSTANCE", Width: 25},
		{Title: "SHELL", Width: 10},
		{Title: "ID", Width: 20},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(nvidiaGreen)).
		BorderBottom(true).
		Bold(true).
		Foreground(lipgloss.Color(nvidiaGreen))
	s.Selected = s.Selected.
		Background(lipgloss.Color("236"))

	t.SetStyles(s)

	return listModel{
		table:   t,
		loading: true,
	}
}

// getAvailableActions returns the list of available actions based on workspace status
func (m listModel) getAvailableActions() []string {
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

func getShellDisplayStatus(w entity.Workspace) string {
	if w.Status == entity.Running {
		return "READY"
	}
	return "NOT READY"
}

func getWorkspaceDisplayStatus(w entity.Workspace) string {
	if w.Status == entity.Running {
		return "RUNNING"
	} else if w.Status == entity.Starting {
		return "STARTING"
	} else if w.Status == entity.Stopping {
		return "STOPPING"
	} else if w.Status == entity.Stopped {
		return "STOPPED"
	} else if w.Status == entity.Deploying {
		return "DEPLOYING"
	} else if w.Status == entity.Deleting {
		return "DELETING"
	} else if w.Status == entity.Failure {
		return "FAILURE"
	}
	return string(w.Status)
}

func (m *listModel) updateWorkspaces(workspaces []entity.Workspace) {
	m.workspaces = workspaces
	var rows []table.Row
	for _, w := range workspaces {
		rows = append(rows, table.Row{
			w.Name,
			getWorkspaceDisplayStatus(w),
			util.GetInstanceString(w),
			getShellDisplayStatus(w),
			w.ID,
		})
	}
	m.table.SetRows(rows)
	m.loading = false
}

func (m listModel) Update(msg tea.Msg) (listModel, tea.Cmd) {
	newModel := m // Create a copy to modify

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle action-specific keys first
		if newModel.showActions {
			switch msg.String() {
			case "esc":
				newModel.showActions = false
				return newModel, nil
			case "left", "right":
				actions := newModel.getAvailableActions()
				if msg.String() == "right" {
					newModel.selectedAction = (newModel.selectedAction + 1) % len(actions)
				} else {
					newModel.selectedAction = (newModel.selectedAction - 1 + len(actions)) % len(actions)
				}
				return newModel, nil
			case "enter":
				// TODO: Handle action selection
				return newModel, nil
			}
		} else {
			switch msg.String() {
			case "enter":
				if len(newModel.workspaces) > 0 {
					newModel.showActions = true
					newModel.selectedAction = 0
					return newModel, nil
				}
			}
		}
	}

	// Let the table handle all other input
	var tableCmd tea.Cmd
	newModel.table, tableCmd = newModel.table.Update(msg)
	
	// Keep our selection in sync with table
	newModel.selectedIndex = newModel.table.Cursor()

	return newModel, tableCmd
}

func (m listModel) View() string {
	if m.loading {
		return "Loading workspaces..."
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	var s strings.Builder

	// Process table lines and insert buttons after selected row
	lines := strings.Split(m.table.View(), "\n")
	for i, line := range lines {
		if i == 0 { // Header row
			s.WriteString("  " + line + "\n")
			continue
		}
		
		rowIndex := i - 1 // Adjust for header row
		if rowIndex == m.selectedIndex {
			// Add the selected row with caret
			s.WriteString(caretStyle.String() + strings.TrimPrefix(line, "  ") + "\n")
			
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
		s.WriteString(helpStyle.Render("  Press Enter for actions • Tab to switch views"))
	}

	return baseStyle.Render(s.String())
}