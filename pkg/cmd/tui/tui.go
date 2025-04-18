package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
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
	tabs            []string
	activeTab       int
	spinner         spinner.Model
	loading         bool
	loadingProgress int
	ready          bool
	width          int
	height         int
	store          store.Store
	terminal       *terminal.Terminal
}

func initialModel(s store.Store, t *terminal.Terminal) model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(nvidiaGreen))

	return model{
		tabs:      []string{"List", "Create"},
		activeTab: 0,
		spinner:   sp,
		loading:   true,
		store:     s,
		terminal:  t,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.EnterAltScreen,
		startLoading(),
	)
}

type loadingTickMsg struct{}

func startLoading() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return loadingTickMsg{}
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % len(m.tabs)
		case "shift+tab", "left", "h":
			m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case loadingTickMsg:
		if m.loading {
			m.loadingProgress++
			if m.loadingProgress > 5 {
				m.loading = false
				m.ready = true
				return m, nil
			}
			return m, startLoading()
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	if !m.ready {
		// Loading screen
		s := strings.Builder{}
		s.WriteString("\n\n")
		s.WriteString(logoStyle.Render(NVIDIA_LOGO))
		s.WriteString("\n\n")
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(nvidiaGreen)).Render(
			fmt.Sprintf("%s Loading%s", m.spinner.View(), strings.Repeat(".", m.loadingProgress)),
		))
		return docStyle.Render(s.String())
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
		content = "List view content here"
	case 1:
		content = "Create view content here"
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

func RunMainTUI(s store.Store, t *terminal.Terminal) error {
	p := tea.NewProgram(initialModel(s, t), tea.WithAltScreen())
	_, err := p.Run()
	return err
} 