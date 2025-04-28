package tui

import (
	"fmt"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/create"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/shared/gpupicker"
	"github.com/brevdev/brev-cli/pkg/store"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	focusedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#76B900")).
			Bold(true)

	blurredStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	cursorStyle = focusedStyle.Copy()

	noStyle = lipgloss.NewStyle()

	helpStyle = blurredStyle.Copy().
			Italic(true)

	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#76B900")).
			Bold(true).
			MarginBottom(1)
)

type createState int

const (
	loadingState createState = iota
	gpuPickerState
	namePickerState
	confirmationState
)

type createModel struct {
	currentState     createState
	err             error
	store           *store.AuthHTTPStore
	creating        bool
	createdWorkspace *entity.Workspace
	selectedGPUConfig *store.GPUConfig
	instanceName     string
	gpuPicker       gpupicker.Model
}

func newCreateModel(s *store.AuthHTTPStore) createModel {
	return createModel{
		store:        s,
		currentState: loadingState,
	}
}

type workspaceCreatedMsg struct {
	workspace *entity.Workspace
	err       error
}

type nameSelectedMsg struct {
	name string
	err  error
}

func runNamePicker() tea.Cmd {
	return func() tea.Msg {
		name, err := create.RunNamePicker()
		return nameSelectedMsg{name: name, err: err}
	}
}

func createWorkspace(s *store.AuthHTTPStore, name string, config *store.GPUConfig) tea.Cmd {
	return func() tea.Msg {
		org, err := s.GetActiveOrganizationOrDefault()
		if err != nil {
			return workspaceCreatedMsg{err: err}
		}

		options := store.NewCreateWorkspacesOptions(
			"", // clusterID
			name,
			config,
		)

		workspace, err := s.CreateWorkspace(org.ID, options)
		if err != nil {
			return workspaceCreatedMsg{err: err}
		}

		return workspaceCreatedMsg{workspace: workspace}
	}
}

func (m createModel) SetInstanceTypes(types *store.InstanceTypeResponse) createModel {
	m.gpuPicker = gpupicker.New(types)
	m.currentState = gpuPickerState
	return m
}

func (m createModel) Init() tea.Cmd {
	return nil
}

func (m createModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if m.currentState == gpuPickerState {
			var cmd tea.Cmd
			m.gpuPicker, cmd = m.gpuPicker.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.currentState > gpuPickerState {
				m.currentState--
				if m.currentState == gpuPickerState {
					return m, nil
				} else if m.currentState == namePickerState {
					return m, runNamePicker()
				}
			}
		case "enter":
			if m.currentState == confirmationState && !m.creating {
				m.creating = true
				return m, createWorkspace(m.store, m.instanceName, m.selectedGPUConfig)
			}
		}

		if m.currentState == gpuPickerState {
			var cmd tea.Cmd
			m.gpuPicker, cmd = m.gpuPicker.Update(msg)
			
			if m.gpuPicker.SelectedConfig() != nil {
				m.selectedGPUConfig = m.gpuPicker.SelectedConfig()
				m.currentState = namePickerState
				return m, runNamePicker()
			}
			
			if m.gpuPicker.Quitting() {
				return m, tea.Quit
			}
			
			return m, cmd
		}

	case nameSelectedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.instanceName = msg.name
		m.currentState = confirmationState
		return m, nil

	case workspaceCreatedMsg:
		m.creating = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.createdWorkspace = msg.workspace
		return m, nil
	}

	return m, nil
}

func (m createModel) View() string {
	if m.createdWorkspace != nil {
		return fmt.Sprintf("Created instance %s!\n\nPress 'tab' to switch back to list view.", m.createdWorkspace.Name)
	}

	if m.err != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("203")).
			Render(fmt.Sprintf("Error: %v\n\nPress ESC to go back or Tab to switch views", m.err))
	}

	switch m.currentState {
	case loadingState:
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#76B900")).
			Render("Loading available GPU configurations...\n\nThis may take a few moments. If it takes too long, press ESC to go back.")

	case gpuPickerState:
		if m.gpuPicker.View() == "" {
			return "Error: GPU picker view is empty. Press ESC to go back."
		}
		return m.gpuPicker.View()

	case namePickerState:
		return "Entering instance name..."

	case confirmationState:
		var b strings.Builder
		b.WriteString(promptStyle.Render("Please confirm your instance details:"))
		b.WriteString("\n\n")
		b.WriteString("Name: " + focusedStyle.Render(m.instanceName))
		b.WriteString("\n")
		b.WriteString("GPU: " + focusedStyle.Render(fmt.Sprintf("%dx %s", m.selectedGPUConfig.Count, m.selectedGPUConfig.Type)))
		b.WriteString("\n\n")
		
		if m.creating {
			b.WriteString("Creating instance...")
		} else {
			b.WriteString(focusedStyle.Render("[ Create Instance ]"))
			b.WriteString("\n\n")
			b.WriteString(helpStyle.Render("Press Enter to create • ESC to go back • Tab to switch views"))
		}
		return b.String()
	}

	return "Unknown state"
} 