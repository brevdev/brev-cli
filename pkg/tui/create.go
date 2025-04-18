package tui

import (
	"fmt"
	"strings"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	focusedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(nvidiaGreen)).
			Bold(true)

	blurredStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	cursorStyle = focusedStyle.Copy()

	noStyle = lipgloss.NewStyle()

	helpStyle = blurredStyle.Copy().
			Italic(true)
)

type createModel struct {
	nameInput     textinput.Model
	gpuTypeInput  textinput.Model
	focusIndex    int
	err           error
	store         *store.AuthHTTPStore
	creating      bool
	createdWorkspace *entity.Workspace
}

func newCreateModel(s *store.AuthHTTPStore) createModel {
	nameInput := textinput.New()
	nameInput.Placeholder = "Enter instance name"
	nameInput.Focus()
	nameInput.CharLimit = 32
	nameInput.Width = 40

	gpuTypeInput := textinput.New()
	gpuTypeInput.Placeholder = "Enter GPU type (e.g. A100, H100, L40S)"
	gpuTypeInput.CharLimit = 32
	gpuTypeInput.Width = 40

	return createModel{
		nameInput:    nameInput,
		gpuTypeInput: gpuTypeInput,
		store:        s,
		focusIndex:   0,
	}
}

type workspaceCreatedMsg struct {
	workspace *entity.Workspace
	err       error
}

func createWorkspace(s *store.AuthHTTPStore, name, gpuType string) tea.Cmd {
	return func() tea.Msg {
		org, err := s.GetActiveOrganizationOrDefault()
		if err != nil {
			return workspaceCreatedMsg{err: err}
		}

		options := store.NewCreateWorkspacesOptions(
			"", // clusterID
			name,
			&store.GPUConfig{
				Type:     gpuType,
				Provider: "nvidia",
			},
		)

		workspace, err := s.CreateWorkspace(org.ID, options)
		if err != nil {
			return workspaceCreatedMsg{err: err}
		}

		return workspaceCreatedMsg{workspace: workspace}
	}
}

func (m createModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m createModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			if s == "enter" && m.focusIndex == len(m.inputs())-1 {
				if !m.creating {
					m.creating = true
					return m, createWorkspace(m.store, m.nameInput.Value(), m.gpuTypeInput.Value())
				}
				return m, nil
			}

			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}

			if m.focusIndex > len(m.inputs())-1 {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs()) - 1
			}

			cmds := make([]tea.Cmd, len(m.inputs()))
			for i := 0; i < len(m.inputs()); i++ {
				if i == m.focusIndex {
					cmds[i] = m.inputs()[i].Focus()
					continue
				}
				m.inputs()[i].Blur()
			}

			return m, tea.Batch(cmds...)
		}

	case workspaceCreatedMsg:
		m.creating = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.createdWorkspace = msg.workspace
		return m, nil
	}

	// Handle character input and blinking
	cmd = m.updateInputs(msg)

	return m, cmd
}

func (m *createModel) inputs() []textinput.Model {
	return []textinput.Model{m.nameInput, m.gpuTypeInput}
}

func (m *createModel) updateInputs(msg tea.Msg) tea.Cmd {
	var cmds = make([]tea.Cmd, len(m.inputs()))

	for i := range m.inputs() {
		m.inputs()[i], cmds[i] = m.inputs()[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

func (m createModel) View() string {
	if m.createdWorkspace != nil {
		return fmt.Sprintf("Created instance %s!\n\nPress 'tab' to switch back to list view.", m.createdWorkspace.Name)
	}

	var b strings.Builder

	b.WriteString("Create a new GPU instance\n\n")

	for i := range m.inputs() {
		input := m.inputs()[i]

		if i == m.focusIndex {
			b.WriteString(focusedStyle.Render("> "))
		} else {
			b.WriteString("  ")
		}

		if i == 0 {
			b.WriteString("Name: ")
		} else {
			b.WriteString("GPU Type: ")
		}

		b.WriteString(input.View())
		b.WriteString("\n")
	}

	if m.err != nil {
		b.WriteString(fmt.Sprintf("\nError: %v\n", m.err))
	}

	if m.creating {
		b.WriteString("\nCreating instance...")
	} else {
		button := "\n[ Create Instance ]"
		if m.focusIndex == len(m.inputs())-1 {
			button = focusedStyle.Render(button)
		}
		b.WriteString(button)
	}

	b.WriteString(helpStyle.Render("\n\nPress tab to switch fields • Enter to submit"))

	return b.String()
} 