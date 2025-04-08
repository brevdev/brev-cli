package create

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#76B900")).
			MarginBottom(1)

	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("white")).
			Padding(1).
			Width(50)

	selectedInputStyle = inputStyle.Copy().
				BorderForeground(lipgloss.Color("#76B900")).
				BorderStyle(lipgloss.DoubleBorder())
)

type nameModel struct {
	textInput textinput.Model
	err       error
	done      bool
}

func initialNameModel() nameModel {
	ti := textinput.New()
	ti.Placeholder = "my-workspace"
	ti.Focus()
	ti.CharLimit = 30
	ti.Width = 30

	return nameModel{
		textInput: ti,
		err:       nil,
	}
}

func (m nameModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m nameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			if strings.TrimSpace(m.textInput.Value()) != "" {
				m.done = true
				return m, tea.Quit
			}
		case tea.KeyCtrlC:
			return m, tea.Quit
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m nameModel) View() string {
	if m.done {
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Enter a name for your workspace:"))
	b.WriteString("\n")
	b.WriteString(selectedInputStyle.Render(m.textInput.View()))
	b.WriteString("\n\n")
	b.WriteString("Press Enter to confirm • Ctrl+C to quit")

	return b.String()
}

func RunNamePicker() (string, error) {
	p := tea.NewProgram(initialNameModel())
	m, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running name picker: %v", err)
	}

	if m, ok := m.(nameModel); ok && m.done {
		return strings.TrimSpace(m.textInput.Value()), nil
	}
	return "", fmt.Errorf("cancelled")
}
