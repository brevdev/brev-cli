package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/brev-cli/pkg/tui/messages"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	nvidiaGreen = "#76B900"
	NVIDIA_LOGO = `███╗   ██╗██╗   ██╗██╗██████╗ ██╗ █████╗ 
████╗  ██║██║   ██║██║██╔══██╗██║██╔══██╗
██╔██╗ ██║██║   ██║██║██║  ██║██║███████║
██║╚██╗██║╚██╗ ██╔╝██║██║  ██║██║██╔══██║
██║ ╚████║ ╚████╔╝ ██║██████╔╝██║██║  ██║
╚═╝  ╚═══╝  ╚═══╝  ╚═╝╚═════╝ ╚═╝╚═╝  ╚═╝`
)

// Style definitions
var (
	logoStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(nvidiaGreen)).
		Align(lipgloss.Center)

	activeTabBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      " ",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┘",
		BottomRight: "└",
	}

	tabBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┴",
		BottomRight: "┴",
	}

	tab = lipgloss.NewStyle().
		Border(tabBorder, true).
		BorderForeground(lipgloss.Color(nvidiaGreen)).
		Padding(0, 1)

	activeTab = tab.Copy().
		Border(activeTabBorder, true).
		Foreground(lipgloss.Color(nvidiaGreen))

	tabGap = tab.Copy().
		BorderTop(false).
		BorderLeft(false).
		BorderRight(false)

	docStyle = lipgloss.NewStyle().Padding(1, 2, 1, 2)
)

type model struct {
	tabs             []string
	activeTab        int
	spinner          spinner.Model
	loading          bool
	loadingProgress  int
	ready            bool
	width            int
	height           int
	store            *store.AuthHTTPStore
	terminal         *terminal.Terminal
	listModel        listModel
	createModel      createModel
	workspaces       []entity.Workspace
	instanceTypes    *store.InstanceTypeResponse
	err              error
}

type workspacesLoadedMsg struct {
	workspaces []entity.Workspace
	err        error
}

func fetchWorkspaces(s *store.AuthHTTPStore) tea.Cmd {
	return func() tea.Msg {
		org, err := s.GetActiveOrganizationOrDefault()
		if err != nil {
			return workspacesLoadedMsg{err: err}
		}

		user, err := s.GetCurrentUser()
		if err != nil {
			return workspacesLoadedMsg{err: err}
		}

		workspaces, err := s.GetWorkspaces(org.ID, &store.GetWorkspacesOptions{UserID: user.ID})
		if err != nil {
			return workspacesLoadedMsg{err: err}
		}

		return workspacesLoadedMsg{workspaces: workspaces}
	}
}

func initialModel(s *store.AuthHTTPStore, t *terminal.Terminal) model {
	sp := spinner.New()
	sp.Spinner = spinner.Points
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	lm := newListModel()

	return model{
		tabs:        []string{"List", "Create"},
		activeTab:   0,
		spinner:     sp,
		loading:     true,
		store:       s,
		terminal:    t,
		listModel:   lm,
		createModel: newCreateModel(s),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.EnterAltScreen,
		startLoading(),
		fetchWorkspaces(m.store),
		messages.LoadInstanceTypes(m.store),
	)
}

type loadingTickMsg struct{}

func startLoading() tea.Cmd {
	return tea.Tick(time.Millisecond*250, func(t time.Time) tea.Msg {
		return loadingTickMsg{}
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if !m.ready {
			return m, nil // Ignore key presses during loading
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			m.activeTab = (m.activeTab + 1) % len(m.tabs)
			if m.activeTab == 1 && m.instanceTypes != nil { // Create tab
				m.createModel = m.createModel.SetInstanceTypes(m.instanceTypes)
			}
			return m, nil
		case "shift+tab":
			m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
			return m, nil
		}

		// Pass messages to active tab
		if m.activeTab == 0 {
			var cmd tea.Cmd
			newListModel, cmd := m.listModel.Update(msg)
			m.listModel = newListModel
			return m, cmd
		} else if m.activeTab == 1 {
			var cmd tea.Cmd
			newCreateModel, cmd := m.createModel.Update(msg)
			if cm, ok := newCreateModel.(createModel); ok {
				m.createModel = cm
				// If a workspace was created, refresh the list
				if m.createModel.createdWorkspace != nil {
					return m, fetchWorkspaces(m.store)
				}
				return m, cmd
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case loadingTickMsg:
		if m.loading {
			m.loadingProgress++
			if m.loadingProgress > 3 && m.workspaces != nil {
				m.loading = false
				m.ready = true
				return m, nil
			}
			return m, startLoading()
		}

	case workspacesLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.loading = false
			m.ready = true
			return m, nil
		}
		m.workspaces = msg.workspaces
		m.listModel.updateWorkspaces(msg.workspaces)
		if !m.ready {
			return m, startLoading() // Continue loading animation
		}
		return m, nil

	case messages.InstanceTypesLoadedMsg:
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.instanceTypes = msg.Types
		if m.activeTab == 1 { // If we're on the create tab
			m.createModel = m.createModel.SetInstanceTypes(msg.Types)
		}
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, cmd
}

func (m model) View() string {
	if !m.ready {
		// Loading screen
		s := strings.Builder{}
		s.WriteString("\n\n")
		s.WriteString(logoStyle.Render(NVIDIA_LOGO))
		s.WriteString("\n\n")

		loadingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(nvidiaGreen))
		
		var loadingText string
		if m.workspaces == nil {
			loadingText = "Fetching Your Instances"
		} else {
			loadingText = "Entering TUI"
		}
		
		s.WriteString(loadingStyle.Render(
			fmt.Sprintf("%s %s%s", m.spinner.View(), loadingText, strings.Repeat(".", m.loadingProgress)),
		))

		if m.err != nil {
			s.WriteString("\n\n")
			s.WriteString(fmt.Sprintf("Error: %v", m.err))
		}

		return docStyle.Render(s.String())
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	// Tabs
	var renderedTabs []string

	for i, t := range m.tabs {
		var style lipgloss.Style
		isActive := i == m.activeTab
		if isActive {
			style = activeTab
		} else {
			style = tab
		}
		renderedTabs = append(renderedTabs, style.Render(t))
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
	gap := tabGap.Render(strings.Repeat(" ", max(0, m.width-lipgloss.Width(row)-2)))
	row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, gap)

	// Content based on active tab
	var content string
	switch m.activeTab {
	case 0:
		content = m.listModel.View()
	case 1:
		content = m.createModel.View()
	}

	return docStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		row,
		"\n",
		content,
	))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func RunMainTUI(s *store.AuthHTTPStore, t *terminal.Terminal) error {
	p := tea.NewProgram(initialModel(s, t), tea.WithAltScreen())
	_, err := p.Run()
	return err
} 