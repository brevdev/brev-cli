package create

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/charmbracelet/lipgloss"
)

const nvidiaGreen = "#76B900"

var (
	gpuTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(nvidiaGreen)).
		MarginBottom(1)

	gpuChipStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("white")).
		Width(20).
		Height(3).
		Align(lipgloss.Center).
		Bold(true).
		MarginRight(2)

	gpuSelectedChipStyle = gpuChipStyle.Copy().
		BorderForeground(lipgloss.Color(nvidiaGreen)).
		BorderStyle(lipgloss.DoubleBorder()).
		Foreground(lipgloss.Color(nvidiaGreen))

	gpuMetadataStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("white")).
		Width(20).
		Align(lipgloss.Center)

	gpuSelectedMetadataStyle = gpuMetadataStyle.Copy().
		Foreground(lipgloss.Color(nvidiaGreen))
)

// GPUType represents a unique GPU model (e.g., T4, A100)
type GPUType struct {
	Name         string
	Manufacturer string
	Configs      []store.GPUInstanceType
}

func (g GPUType) Title() string       { return g.Name }
func (g GPUType) Description() string { return g.Manufacturer }
func (g GPUType) FilterValue() string { return g.Name }

// GPUConfig represents a specific configuration for a GPU type
type GPUConfig struct {
	Type     string
	GPUCount int
}

func (g GPUConfig) Title() string       { return fmt.Sprintf("%s (%dx)", g.Type, g.GPUCount) }
func (g GPUConfig) Description() string { return fmt.Sprintf("%d GPU(s)", g.GPUCount) }
func (g GPUConfig) FilterValue() string { return g.Type }

type gpuModel struct {
	gpuTypes     []GPUType
	configs      []GPUConfig
	selectedType *GPUType
	selectedConfig *GPUConfig
	showingConfigs bool
	quitting      bool
	spring        *harmonica.Spring
	x             float64
	xVelocity     float64
	spinner       spinner.Model
	err           error
	cursor        int
}

// Custom delegate for GPU items
type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 5 }
func (d itemDelegate) Spacing() int                           { return 1 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	var chipBox, metadata string
	
	if gpu, ok := listItem.(GPUType); ok {
		if index == m.Index() {
			chipBox = gpuSelectedChipStyle.Render(gpu.Name)
			metadata = gpuSelectedMetadataStyle.Render(gpu.Manufacturer)
		} else {
			chipBox = gpuChipStyle.Render(gpu.Name)
			metadata = gpuMetadataStyle.Render(gpu.Manufacturer)
		}
	} else if config, ok := listItem.(GPUConfig); ok {
		if index == m.Index() {
			chipBox = gpuSelectedChipStyle.Render(fmt.Sprintf("%dx GPU", config.GPUCount))
			metadata = gpuSelectedMetadataStyle.Render(config.Type)
		} else {
			chipBox = gpuChipStyle.Render(fmt.Sprintf("%dx GPU", config.GPUCount))
			metadata = gpuMetadataStyle.Render(config.Type)
		}
	}

	fmt.Fprint(w, lipgloss.JoinVertical(lipgloss.Left, chipBox, metadata))
}

func organizeGPUTypes(types *store.InstanceTypeResponse) []GPUType {
	gpuMap := make(map[string]*GPUType)

	for _, instance := range types.AllInstanceTypes {
		if len(instance.SupportedGPUs) > 0 {
			gpu := instance.SupportedGPUs[0]
			key := gpu.Name
			if _, exists := gpuMap[key]; !exists {
				gpuMap[key] = &GPUType{
					Name:         gpu.Name,
					Manufacturer: gpu.Manufacturer,
				}
			}
			gpuMap[key].Configs = append(gpuMap[key].Configs, instance)
		}
	}

	var result []GPUType
	for _, gpuType := range gpuMap {
		result = append(result, *gpuType)
	}
	return result
}

func getConfigsForType(gpuType *GPUType) []GPUConfig {
	var configs []GPUConfig
	for _, config := range gpuType.Configs {
		configs = append(configs, GPUConfig{
			Type:     config.Type,
			GPUCount: config.SupportedGPUs[0].Count,
		})
	}
	return configs
}

func initialGPUModel(types *store.InstanceTypeResponse) gpuModel {
	gpuTypes := organizeGPUTypes(types)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(nvidiaGreen))

	spring := harmonica.NewSpring(harmonica.FPS(60), 8.0, 0.2)

	return gpuModel{
		gpuTypes: gpuTypes,
		spinner:  s,
		spring:   &spring,
		cursor:   0,
	}
}

type frameMsg struct{}

func frame() tea.Msg {
	return frameMsg{}
}

func animate() tea.Cmd {
	return tea.Tick(time.Second/60, func(time.Time) tea.Msg {
		return frameMsg{}
	})
}

func (m gpuModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		animate(),
	)
}

func (m gpuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "right", "l":
			if !m.showingConfigs {
				if m.cursor < len(m.gpuTypes)-1 {
					m.cursor++
				}
			} else {
				if m.cursor < len(m.configs)-1 {
					m.cursor++
				}
			}
		case "left", "h":
			if m.cursor > 0 {
				m.cursor--
			}
		case "esc":
			if m.showingConfigs {
				m.showingConfigs = false
				m.cursor = 0
				m.selectedType = nil
			}
		case "enter":
			if !m.showingConfigs {
				m.selectedType = &m.gpuTypes[m.cursor]
				m.configs = getConfigsForType(m.selectedType)
				m.showingConfigs = true
				m.cursor = 0
			} else {
				m.selectedConfig = &m.configs[m.cursor]
				return m, tea.Quit
			}
		}

	case frameMsg:
		m.x, m.xVelocity = m.spring.Update(m.x, m.xVelocity, 2.0)
		return m, animate()
	}

	return m, nil
}

func (m gpuModel) View() string {
	if m.quitting {
		return "GPU selection cancelled\n"
	}
	if m.selectedConfig != nil {
		return fmt.Sprintf("Selected GPU configuration: %s\n", m.selectedConfig.Type)
	}

	padding := strings.Repeat(" ", int(m.x))
	var title string
	if !m.showingConfigs {
		title = gpuTitleStyle.Render("Select GPU Type:")
	} else {
		title = gpuTitleStyle.Render(fmt.Sprintf("Select %s Configuration:", m.selectedType.Name))
	}

	var items []string
	if !m.showingConfigs {
		for i, gpu := range m.gpuTypes {
			var style lipgloss.Style
			if i == m.cursor {
				style = gpuSelectedChipStyle
			} else {
				style = gpuChipStyle
			}
			items = append(items, style.Render(gpu.Name))
		}
	} else {
		for i, config := range m.configs {
			var style lipgloss.Style
			if i == m.cursor {
				style = gpuSelectedChipStyle
			} else {
				style = gpuChipStyle
			}
			items = append(items, style.Render(fmt.Sprintf("%dx %s", config.GPUCount, strings.Split(config.Type, ":")[0])))
		}
	}

	content := lipgloss.JoinHorizontal(lipgloss.Top, items...)
	help := "\n↑/↓: Navigate • Enter: Select • ESC: Back • q: Quit"

	return padding + lipgloss.JoinVertical(lipgloss.Left,
		title,
		content,
		help,
	)
}

func RunGPUPicker(types *store.InstanceTypeResponse) (string, error) {
	m := initialGPUModel(types)
	p := tea.NewProgram(&m)
	model, err := p.Run()
	if err != nil {
		return "", fmt.Errorf("error running GPU picker: %v", err)
	}

	if m, ok := model.(gpuModel); ok && m.selectedConfig != nil {
		return m.selectedConfig.Type, nil
	}
	return "", fmt.Errorf("cancelled")
} 