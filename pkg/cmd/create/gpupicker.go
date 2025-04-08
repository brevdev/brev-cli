package create

import (
	"fmt"
	"io"
	"strings"

	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const nvidiaGreen = "#76B900"

var (
	gpuTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(nvidiaGreen))

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

	configBoxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("white")).
		Width(70).
		Align(lipgloss.Left).
		PaddingLeft(1).
		MarginBottom(0)

	configSelectedBoxStyle = configBoxStyle.Copy().
		BorderForeground(lipgloss.Color(nvidiaGreen)).
		BorderStyle(lipgloss.DoubleBorder()).
		Foreground(lipgloss.Color(nvidiaGreen))

	configHeaderStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color(nvidiaGreen)).
		Bold(true).
		MarginTop(0).
		MarginBottom(0)

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
	gpuTypes       []GPUType
	configs        []GPUConfig
	selectedType   *GPUType
	selectedConfig *GPUConfig
	showingConfigs bool
	quitting       bool
	cursor         int
	viewport       viewport.Model
	ready          bool
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
			// Only include NVIDIA GPUs
			if strings.ToUpper(gpu.Manufacturer) != "NVIDIA" {
				continue
			}
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

	// Create ordered result with priority GPUs first
	priorityGPUs := []string{"H100", "A100", "L40S"}
	var result []GPUType
	
	// Add priority GPUs first in specified order
	for _, name := range priorityGPUs {
		if gpu, exists := gpuMap[name]; exists {
			result = append(result, *gpu)
			delete(gpuMap, name)
		}
	}
	
	// Add remaining GPUs
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
	return gpuModel{
		gpuTypes: gpuTypes,
		cursor:   0,
		viewport: viewport.New(70, 20), // Initialize with reasonable defaults
	}
}

func (m gpuModel) Init() tea.Cmd {
	return nil
}

func (m gpuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if !m.ready {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - 4 // Leave room for title and help
			m.ready = true
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.showingConfigs {
				if m.cursor < len(m.configs)-1 {
					m.cursor++
				}
			} else {
				if m.cursor < len(m.gpuTypes)-1 {
					m.cursor++
				}
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
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m gpuModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.quitting {
		return "GPU selection cancelled\n"
	}
	if m.selectedConfig != nil {
		return fmt.Sprintf("Selected GPU configuration: %s\n", m.selectedConfig.Type)
	}

	var title string
	if !m.showingConfigs {
		title = gpuTitleStyle.Render("Select GPU Type:")
	} else {
		title = gpuTitleStyle.Render(fmt.Sprintf("Select %s Configuration:", m.selectedType.Name))
	}

	var content string
	if !m.showingConfigs {
		var rows []string
		var currentRow []string
		for i, gpu := range m.gpuTypes {
			var style lipgloss.Style
			if i == m.cursor {
				style = gpuSelectedChipStyle
			} else {
				style = gpuChipStyle
			}
			currentRow = append(currentRow, style.Render(gpu.Name))
			
			if len(currentRow) == 3 || i == len(m.gpuTypes)-1 {
				rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, currentRow...))
				currentRow = nil
			}
		}
		content = lipgloss.JoinVertical(lipgloss.Left, rows...)
	} else {
		configsByCount := make(map[int][]GPUConfig)
		for _, config := range m.configs {
			configsByCount[config.GPUCount] = append(configsByCount[config.GPUCount], config)
		}

		var sections []string
		counts := []int{1, 2, 4, 8}
		currentIndex := 0

		for _, count := range counts {
			if configs, exists := configsByCount[count]; exists {
				if currentIndex > 0 {
					sections = append(sections, "") // Add minimal spacing between sections
				}
				header := configHeaderStyle.Render(fmt.Sprintf("%dx GPUs:", count))
				sections = append(sections, header)

				for _, config := range configs {
					var style lipgloss.Style
					if currentIndex == m.cursor {
						style = configSelectedBoxStyle
					} else {
						style = configBoxStyle
					}

					parts := strings.Split(config.Type, ":")
					instanceType := parts[0]
					provider := "AWS"
					price := "$X.XX/hr"

					displayText := fmt.Sprintf("%-30s  %-10s  %s", instanceType, provider, price)
					sections = append(sections, style.Render(displayText))
					currentIndex++
				}
			}
		}
		content = lipgloss.JoinVertical(lipgloss.Left, sections...)
	}

	m.viewport.SetContent(content)
	
	help := "\n↑/↓: Navigate • Enter: Select • ESC: Back • q: Quit"
	
	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		m.viewport.View(),
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