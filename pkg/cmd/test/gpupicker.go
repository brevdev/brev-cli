package test

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/lipgloss"
)

const nvidiaGreen = "#76B900"

var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(nvidiaGreen))

	chipStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("white")).
		Width(20).
		Height(3).
		Align(lipgloss.Center).
		Bold(true)

	selectedChipStyle = chipStyle.Copy().
		BorderForeground(lipgloss.Color(nvidiaGreen)).
		BorderStyle(lipgloss.DoubleBorder()).
		Foreground(lipgloss.Color(nvidiaGreen))

	metadataStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("white")).
		MarginLeft(2)

	selectedMetadataStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(nvidiaGreen)).
		MarginLeft(2)

	gpuStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(nvidiaGreen)).
		Padding(0).
		MarginTop(1).
		Height(20).
		Width(70)

	infoStyle = lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#666666"))
)

type GPU struct {
	name        string
	memory      string
	performance string
	price       string
}

func (g GPU) Title() string       { return g.name }
func (g GPU) Description() string { return fmt.Sprintf("Memory: %s | Performance: %s | Price: %s", g.memory, g.performance, g.price) }
func (g GPU) FilterValue() string { return g.name }

type model struct {
	gpus       list.Model
	selected   *GPU
	quitting   bool
	spring     *harmonica.Spring
	x          float64
	xVelocity  float64
	spinner    spinner.Model
	err        error
}

// Custom delegate for GPU items
type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 5 }
func (d itemDelegate) Spacing() int                           { return 1 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	gpu, ok := listItem.(GPU)
	if !ok {
		return
	}

	var chipBox, metadata string
	if index == m.Index() {
		// Selected item
		chipBox = selectedChipStyle.Render(strings.TrimPrefix(gpu.name, "NVIDIA "))
		metadata = selectedMetadataStyle.Render(fmt.Sprintf("Memory: %s\nPerformance: %s\nPrice: %s", 
			gpu.memory, gpu.performance, gpu.price))
	} else {
		// Unselected item
		chipBox = chipStyle.Render(strings.TrimPrefix(gpu.name, "NVIDIA "))
		metadata = metadataStyle.Render(fmt.Sprintf("Memory: %s\nPerformance: %s\nPrice: %s", 
			gpu.memory, gpu.performance, gpu.price))
	}

	// Join the chip and metadata vertically
	fmt.Fprint(w, lipgloss.JoinVertical(lipgloss.Left, chipBox, metadata))
}

func initialModel() model {
	gpus := []list.Item{
		GPU{name: "NVIDIA A100", memory: "80GB", performance: "High", price: "$4.50/hr"},
		GPU{name: "NVIDIA H100", memory: "80GB", performance: "Ultra", price: "$5.50/hr"},
		GPU{name: "NVIDIA L40S", memory: "48GB", performance: "Medium", price: "$2.50/hr"},
	}

	l := list.New(gpus, itemDelegate{}, 0, 0)
	l.Title = "Select GPU"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.SetShowHelp(false)
	l.SetHeight(20)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(nvidiaGreen))

	spring := harmonica.NewSpring(harmonica.FPS(60), 8.0, 0.2)

	return model{
		gpus:    l,
		spinner: s,
		spring:  &spring,
	}
}

// Frame is a message that triggers an animation frame
type frameMsg struct{}

func frame() tea.Msg {
	return frameMsg{}
}

func animate() tea.Cmd {
	return tea.Tick(time.Second/60, func(time.Time) tea.Msg {
		return frameMsg{}
	})
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		animate(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			m.quitting = true
			return m, tea.Quit
		}

		if msg.String() == "enter" {
			i, ok := m.gpus.SelectedItem().(GPU)
			if ok {
				m.selected = &i
				return m, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		h, _ := gpuStyle.GetFrameSize()
		m.gpus.SetSize(msg.Width-h, 20) // Fix height to prevent excess space

	case frameMsg:
		m.x, m.xVelocity = m.spring.Update(m.x, m.xVelocity, 2.0)
		return m, animate()
	}

	var cmd tea.Cmd
	m.gpus, cmd = m.gpus.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return "Thanks for using GPU Picker!\n"
	}
	if m.selected != nil {
		return fmt.Sprintf("Selected GPU: %s\n", m.selected.name)
	}

	// Create horizontal padding based on spring animation
	padding := strings.Repeat(" ", int(m.x))

	return gpuStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Left,
			padding+titleStyle.Render("🚀 GPU Instance Picker"),
			m.gpus.View(),
			infoStyle.Render("Press 'q' to quit, 'enter' to select"),
		),
	)
}

func RunGPUPicker() (*GPU, error) {
	p := tea.NewProgram(initialModel())
	m, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running gpu picker: %v", err)
	}

	if m.(model).selected != nil {
		return m.(model).selected, nil
	}
	return nil, nil
} 