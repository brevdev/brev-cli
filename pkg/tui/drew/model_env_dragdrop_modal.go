package drew

import (
	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
)

type EnvDragDropModal struct {
	environment *Environment
	width      int
	height     int
}

func NewEnvDragDropModal() *EnvDragDropModal {
	return &EnvDragDropModal{}
}

func (m EnvDragDropModal) Init() tea.Cmd {
	return nil
}

func (m EnvDragDropModal) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		if msg.String() == " " {
			// Close modal on space
			return &m, nil
		}
	}
	return &m, nil
}

func (m EnvDragDropModal) View() string {
	foreStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true).
		BorderForeground(lipgloss.Color("6")).
		Padding(1, 2)

	boldStyle := lipgloss.NewStyle().Bold(true)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	title := boldStyle.Render("File Transfer")
	subtitle := dimStyle.Render("Drag and drop files here to SCP them to " + m.environment.Name)

	content := lipgloss.JoinVertical(lipgloss.Center, 
		title,
		"",
		subtitle,
		"",
		dimStyle.Render("Press SPACE to close"),
	)

	return foreStyle.Render(content)
}

func (m *EnvDragDropModal) SetEnvironment(environment *Environment) {
	m.environment = environment
}

func (m *EnvDragDropModal) SetWidth(width int) {
	m.width = width
}

func (m *EnvDragDropModal) SetHeight(height int) {
	m.height = height
} 