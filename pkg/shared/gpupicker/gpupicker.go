package gpupicker

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	NVIDIA_LOGO = `███╗   ██╗██╗   ██╗██╗██████╗ ██╗ █████╗ 
████╗  ██║██║   ██║██║██╔══██╗██║██╔══██╗
██╔██╗ ██║██║   ██║██║██║  ██║██║███████║
██║╚██╗██║╚██╗ ██╔╝██║██║  ██║██║██╔══██║
██║ ╚████║ ╚████╔╝ ██║██████╔╝██║██║  ██║
╚═╝  ╚═══╝  ╚═══╝  ╚═╝╚═════╝ ╚═╝╚═╝  ╚═╝`
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#76B900")).
			MarginBottom(1)

	logoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#76B900")).
			Align(lipgloss.Center).
			MarginBottom(2)

	chipStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("white")).
			Width(20).
			Height(3).
			Align(lipgloss.Center).
			AlignVertical(lipgloss.Center).
			Bold(true).
			Padding(0, 1).
			MarginRight(4).
			MarginBottom(1)

	selectedChipStyle = chipStyle.Copy().
				BorderForeground(lipgloss.Color("#76B900")).
				BorderStyle(lipgloss.DoubleBorder()).
				Foreground(lipgloss.Color("#76B900"))

	configHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#76B900")).
				Bold(true).
				MarginTop(1).
				MarginBottom(1)

	metadataStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("white")).
				Width(20).
				Align(lipgloss.Center)

	selectedMetadataStyle = metadataStyle.Copy().
					Foreground(lipgloss.Color("#76B900"))

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true).
			MarginTop(1)

	configStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("white")).
			Width(140).
			Height(1).
			Padding(0, 2).
			MarginBottom(0)

	selectedConfigStyle = configStyle.Copy().
				BorderForeground(lipgloss.Color("#76B900")).
				BorderStyle(lipgloss.DoubleBorder()).
				Foreground(lipgloss.Color("#76B900"))

	gpuGroupStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#76B900")).
			Bold(true).
			MarginTop(1).
			MarginBottom(0)

	providerPriceStyle = lipgloss.NewStyle().
			Align(lipgloss.Right).
			Width(25)

	specsStyle = lipgloss.NewStyle().
			Width(90).
			Align(lipgloss.Left)

	spacerStyle = lipgloss.NewStyle().
			Width(2)
)

type Model struct {
	gpuTypes       []store.GPUType
	configs        []store.GPUConfig
	selectedType   *store.GPUType
	selectedConfig *store.GPUConfig
	showingConfigs bool
	cursor         int
	viewport       viewport.Model
	ready          bool
	width          int
	height         int
	quitting       bool
}

func New(types *store.InstanceTypeResponse) Model {
	gpuTypes := organizeGPUTypes(types)
	vp := viewport.New(150, 30)
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#76B900"))

	return Model{
		gpuTypes: gpuTypes,
		cursor:   0,
		viewport: vp,
		ready:    true,
	}
}

func organizeGPUTypes(types *store.InstanceTypeResponse) []store.GPUType {
	gpuMap := make(map[string]*store.GPUType)

	for _, instance := range types.AllInstanceTypes {
		if len(instance.SupportedGPUs) > 0 {
			gpu := instance.SupportedGPUs[0]
			if strings.ToUpper(gpu.Manufacturer) != "NVIDIA" {
				continue
			}
			key := gpu.Name
			if _, exists := gpuMap[key]; !exists {
				gpuMap[key] = &store.GPUType{
					Name:         gpu.Name,
					Manufacturer: gpu.Manufacturer,
				}
			}
			gpuMap[key].Configs = append(gpuMap[key].Configs, instance)
		}
	}

	priorityGPUs := []string{"H100", "A100", "L40S"}
	var result []store.GPUType

	for _, name := range priorityGPUs {
		if gpu, exists := gpuMap[name]; exists {
			result = append(result, *gpu)
			delete(gpuMap, name)
		}
	}

	for _, gpuType := range gpuMap {
		result = append(result, *gpuType)
	}

	return result
}

func getConfigsForType(instanceTypes []store.GPUInstanceType, gpuType string) []store.GPUConfig {
	var configs []store.GPUConfig
	for _, config := range instanceTypes {
		for _, gpu := range config.SupportedGPUs {
			if gpu.Name == gpuType {
				configs = append(configs, store.GPUConfig{
					Type:            config.Type,
					Count:           gpu.Count,
					Provider:        config.Provider,
					Price:           config.BasePrice,
					WorkspaceGroups: config.WorkspaceGroups,
				})
			}
		}
	}

	validCounts := []int{1, 2, 4, 8}

	configsByCount := make(map[int][]store.GPUConfig)
	for _, config := range configs {
		if !slices.Contains(validCounts, config.Count) {
			continue
		}
		configsByCount[config.Count] = append(configsByCount[config.Count], config)
	}

	orderedConfigs := []store.GPUConfig{}
	for _, count := range validCounts {
		orderedConfigs = append(orderedConfigs, configsByCount[count]...)
	}
	return orderedConfigs
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(viewport.Sync(m.viewport))
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 4 // Leave room for status line
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if !m.showingConfigs {
				if m.cursor >= 3 {
					m.cursor -= 3
				}
			} else {
				if m.cursor > 0 {
					m.cursor--
				}
			}

		case "down", "j":
			if !m.showingConfigs {
				if m.cursor+3 < len(m.gpuTypes) {
					m.cursor += 3
				}
			} else {
				if m.cursor < len(m.configs)-1 {
					m.cursor++
				}
			}

		case "left", "h":
			if !m.showingConfigs {
				if m.cursor > 0 {
					m.cursor--
				}
			}

		case "right", "l":
			if !m.showingConfigs {
				if m.cursor < len(m.gpuTypes)-1 {
					m.cursor++
				}
			}

		case "enter":
			if !m.showingConfigs {
				m.selectedType = &m.gpuTypes[m.cursor]
				m.configs = getConfigsForType(m.selectedType.Configs, m.selectedType.Name)
				m.showingConfigs = true
				m.cursor = 0
			} else {
				m.selectedConfig = &m.configs[m.cursor]
				return m, tea.Quit
			}

		case "esc":
			if m.showingConfigs {
				m.showingConfigs = false
				m.configs = nil
				m.cursor = 0
			}
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var content string
	if !m.showingConfigs {
		content = m.renderGPUTypes()
	} else {
		content = m.renderConfigs()
	}

	m.viewport.SetContent(content)
	return fmt.Sprintf("%s\n%s", m.viewport.View(), helpStyle.Render("↑/↓/←/→ to select • enter to choose • esc to go back"))
}

func (m Model) renderGPUTypes() string {
	var b strings.Builder

	b.WriteString(logoStyle.Render(NVIDIA_LOGO))
	b.WriteString(titleStyle.Render("Select GPU Type:"))
	b.WriteString("\n")

	// Calculate grid layout
	const itemsPerRow = 3
	rows := (len(m.gpuTypes) + itemsPerRow - 1) / itemsPerRow // Ceiling division

	for row := 0; row < rows; row++ {
		var rowItems []string
		for col := 0; col < itemsPerRow; col++ {
			idx := row*itemsPerRow + col
			if idx >= len(m.gpuTypes) {
				// Add empty space to maintain grid alignment
				rowItems = append(rowItems, strings.Repeat(" ", chipStyle.GetWidth()))
				continue
			}

			gpu := m.gpuTypes[idx]
			var renderedGPU string
			if idx == m.cursor {
				renderedGPU = selectedChipStyle.Render(gpu.Name)
			} else {
				renderedGPU = chipStyle.Render(gpu.Name)
			}
			rowItems = append(rowItems, renderedGPU)
		}
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, rowItems...))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("↑/↓/←/→ to select • enter to choose • esc to go back"))

	return b.String()
}

func (m Model) renderConfigs() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("Select %s Configuration:", m.selectedType.Name)))
	b.WriteString("\n")

	// Group configs by GPU count
	configsByCount := make(map[int][]store.GPUConfig)
	for _, config := range m.configs {
		configsByCount[config.Count] = append(configsByCount[config.Count], config)
	}

	currentIndex := 0
	validCounts := []int{1, 2, 4, 8}
	
	for _, count := range validCounts {
		configs := configsByCount[count]
		if len(configs) > 0 {
			b.WriteString(gpuGroupStyle.Render(fmt.Sprintf("%dx GPUs:", count)))
			b.WriteString("\n")

			for _, config := range configs {
				var style lipgloss.Style
				if currentIndex == m.cursor {
					style = selectedConfigStyle
				} else {
					style = configStyle
				}

				// Get the instance type for this config to access memory and CPU info
				var instanceType *store.GPUInstanceType
				for _, it := range m.selectedType.Configs {
					if it.Type == config.Type {
						instanceType = &it
						break
					}
				}

				// Format the main configuration details
				var details string
				if instanceType != nil && len(instanceType.SupportedGPUs) > 0 {
					gpu := instanceType.SupportedGPUs[0]
					details = specsStyle.Render(fmt.Sprintf("%s VRAM • %s RAM x %d CPUs",
						gpu.Memory,
						instanceType.Memory,
						instanceType.VCPU,
					))
				} else {
					details = specsStyle.Render(fmt.Sprintf("%dx GPU", config.Count))
				}

				// Parse price amount from string to float
				priceAmount, _ := strconv.ParseFloat(config.Price.Amount, 64)
				
				// Format provider and price
				providerPrice := providerPriceStyle.Render(
					fmt.Sprintf("%s • $%.2f/hr", strings.ToLower(config.Provider), priceAmount),
				)

				// Combine details and provider/price with proper spacing
				content := lipgloss.JoinHorizontal(
					lipgloss.Center,
					details,
					spacerStyle.Render(""),
					providerPrice,
				)

				b.WriteString(style.Render(content))
				b.WriteString("\n")
				currentIndex++
			}
		}
	}

	b.WriteString(helpStyle.Render("↑/↓ to select • enter to choose • esc to go back"))

	return b.String()
}

func (m Model) SelectedConfig() *store.GPUConfig {
	return m.selectedConfig
}

func (m Model) Quitting() bool {
	return m.quitting
} 