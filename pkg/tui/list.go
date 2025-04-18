package tui

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

var (
	tableStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(nvidiaGreen))

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(nvidiaGreen))

	selectedRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(nvidiaGreen)).
			Bold(true)
)

type listModel struct {
	table   table.Model
	loading bool
	err     error
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
		Foreground(lipgloss.Color(nvidiaGreen)).
		Bold(true)
	t.SetStyles(s)

	return listModel{
		table:   t,
		loading: true,
	}
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

func (m listModel) View() string {
	if m.loading {
		return "Loading workspaces..."
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}
	return tableStyle.Render(m.table.View())
} 