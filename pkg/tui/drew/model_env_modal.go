package drew

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
)

type EnvModal struct {
	environment *Environment
	commands    list.Model
}

func NewEnvModal() *EnvModal {
	commands := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	commands.SetFilteringEnabled(false)
	commands.SetShowHelp(false)
	commands.SetShowStatusBar(false)
	commands.SetShowPagination(false)
	commands.SetShowTitle(false)

	return &EnvModal{
		commands: commands,
	}
}

func (m EnvModal) Init() tea.Cmd {
	return nil
}

func (m EnvModal) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	if msg, ok := msg.(tea.KeyMsg); ok {
		if msg.String() == "enter" {
			return &m, cmdEnvCommand(m.commands.SelectedItem().(commandItem).title)
		}
	}

	m.commands, cmd = m.commands.Update(msg)
	return &m, cmd
}

func (m EnvModal) View() string {
	foreStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder(), true).
		BorderForeground(lipgloss.Color("6")).
		Padding(0, 1)

	boldStyle := lipgloss.NewStyle().Bold(true)

	title := boldStyle.Render(m.environment.Name)
	directive := "Select an action to perform on the environment"

	header := fmt.Sprintf("%s\n%s\n\n", title, directive)

	content := lipgloss.JoinVertical(lipgloss.Left, header, m.commands.View())

	return foreStyle.Render(content)
}

func (m *EnvModal) SetEnvironment(environment *Environment) {
	items := []list.Item{}
	if environment.Status == EnvironmentStatusRunning {
		items = append(items, commandItem{title: "Stop"})
	}
	if environment.Status == EnvironmentStatusStopped {
		items = append(items, commandItem{title: "Start"})
	}
	items = append(items, commandItem{title: "Terminate"})

	m.commands.SetItems(items)
	m.environment = environment
}

func (m *EnvModal) SetWidth(width int) {
	m.commands.SetWidth(min(width, 30))
}

func (m *EnvModal) SetHeight(height int) {
	m.commands.SetHeight(min(height, 10))
}

type envCommandMsg struct {
	command string
	err     error
}

func cmdEnvCommand(command string) tea.Cmd {
	return func() tea.Msg {
		return envCommandMsg{command: command, err: nil}
	}
}

type commandItem struct {
	title string
}

func (i commandItem) Title() string       { return i.title }
func (i commandItem) Description() string { return "" }
func (i commandItem) FilterValue() string { return i.title }

// ContentFuncModel is a model that simply returns the result of a function call when its View()
// method is invoked.
type ContentFuncModel struct {
	contentFunc func() string
}

func NewPassthroughModel(contentFunc func() string) *ContentFuncModel {
	return &ContentFuncModel{
		contentFunc: contentFunc,
	}
}

func (m *ContentFuncModel) SetContentFunc(contentFunc func() string) {
	m.contentFunc = contentFunc
}

func (m ContentFuncModel) Init() tea.Cmd {
	return nil
}

func (m ContentFuncModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return &m, nil
}

func (m ContentFuncModel) View() string {
	if m.contentFunc != nil {
		return m.contentFunc()
	}
	return ""
}
