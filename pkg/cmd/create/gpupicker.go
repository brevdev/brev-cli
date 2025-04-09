package create

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	NVIDIA_LOGO_2 = `███╗   ██╗██╗   ██╗██╗██████╗ ██╗ █████╗ 
████╗  ██║██║   ██║██║██╔══██╗██║██╔══██╗
██╔██╗ ██║██║   ██║██║██║  ██║██║███████║
██║╚██╗██║╚██╗ ██╔╝██║██║  ██║██║██╔══██║
██║ ╚████║ ╚████╔╝ ██║██████╔╝██║██║  ██║
╚═╝  ╚═══╝  ╚═══╝  ╚═╝╚═════╝ ╚═╝╚═╝  ╚═╝`
)

var (
	gpuTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#76B900"))

	logoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#76B900")).
			Align(lipgloss.Left)

	gpuChipStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("white")).
			Width(25).
			Height(5).
			Align(lipgloss.Center).
			AlignVertical(lipgloss.Center).
			Bold(true).
			MarginRight(2)

	gpuSelectedChipStyle = gpuChipStyle.Copy().
				BorderForeground(lipgloss.Color("#76B900")).
				BorderStyle(lipgloss.DoubleBorder()).
				Foreground(lipgloss.Color("#76B900"))

	configHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#76B900")).
				Bold(true).
				MarginTop(0).
				MarginBottom(0)

	gpuMetadataStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("white")).
				Width(20).
				Align(lipgloss.Center)

	gpuSelectedMetadataStyle = gpuMetadataStyle.Copy().
					Foreground(lipgloss.Color("#76B900"))

	leftSpecStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Bold(true)

	rightSpecStyle = lipgloss.NewStyle().
			Align(lipgloss.Right).
			Bold(true)
)

type gpuModel struct {
	gpuTypes       []store.GPUType
	configs        []store.GPUConfig
	selectedType   *store.GPUType
	selectedConfig *store.GPUConfig
	showingConfigs bool
	quitting       bool
	cursor         int
	viewport       viewport.Model
	ready          bool
	width          int    // Track terminal width
	height         int    // Track terminal height
}

// Custom delegate for GPU items
type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 5 }
func (d itemDelegate) Spacing() int                            { return 1 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	var chipBox, metadata string

	if gpu, ok := listItem.(store.GPUType); ok {
		if index == m.Index() {
			chipBox = gpuSelectedChipStyle.Render(gpu.Name)
			metadata = gpuSelectedMetadataStyle.Render(gpu.Manufacturer)
		} else {
			chipBox = gpuChipStyle.Render(gpu.Name)
			metadata = gpuMetadataStyle.Render(gpu.Manufacturer)
		}
	} else if config, ok := listItem.(store.GPUConfig); ok {
		if index == m.Index() {
			chipBox = gpuSelectedChipStyle.Render(fmt.Sprintf("%dx GPU", config.Count))
			metadata = gpuSelectedMetadataStyle.Render(config.Type)
		} else {
			chipBox = gpuChipStyle.Render(fmt.Sprintf("%dx GPU", config.Count))
			metadata = gpuMetadataStyle.Render(config.Type)
		}
	}

	fmt.Fprint(w, lipgloss.JoinVertical(lipgloss.Left, chipBox, metadata))
}

func organizeGPUTypes(types *store.InstanceTypeResponse) []store.GPUType {
	gpuMap := make(map[string]*store.GPUType)

	for _, instance := range types.AllInstanceTypes {
		if len(instance.SupportedGPUs) > 0 {
			gpu := instance.SupportedGPUs[0]
			// Only include NVIDIA GPUs
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

	// Create ordered result with priority GPUs first
	priorityGPUs := []string{"H100", "A100", "L40S"}
	var result []store.GPUType

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
		// skip if count is invalid
		if !slices.Contains(validCounts, config.Count) {
			continue
		}
		configsByCount[config.Count] = append(configsByCount[config.Count], config)
	}

	// order by count
	orderedConfigs := []store.GPUConfig{}
	for _, count := range validCounts {
		orderedConfigs = append(orderedConfigs, configsByCount[count]...)
	}
	return orderedConfigs
}

func initialGPUModel(types *store.InstanceTypeResponse) gpuModel {
	gpuTypes := organizeGPUTypes(types)
	return gpuModel{
		gpuTypes: gpuTypes,
		cursor:   0,
	}
}

func (m gpuModel) Init() tea.Cmd {
	return tea.EnterAltScreen
}

func (m gpuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if !m.ready {
			m.width = msg.Width
			m.height = msg.Height
			headerHeight := 12 // Increased from 10 to 12 for more top space
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight)
			m.viewport.YPosition = headerHeight
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
				// Scroll up if needed
				if m.showingConfigs {
					lineHeight := 2 // Account for headers and spacing
					cursorPos := m.cursor * lineHeight
					if cursorPos < m.viewport.YOffset {
						m.viewport.LineUp(lineHeight)
					}
				}
			}
		case "down", "j":
			var maxCursor int
			if m.showingConfigs {
				maxCursor = len(m.configs) - 1
			} else {
				maxCursor = len(m.gpuTypes) - 1
			}

			if m.cursor < maxCursor {
				m.cursor++
				// Scroll down if needed
				if m.showingConfigs {
					lineHeight := 2 // Account for headers and spacing
					cursorPos := m.cursor * lineHeight
					viewportBottom := m.viewport.YOffset + m.viewport.Height - 2
					if cursorPos >= viewportBottom {
						m.viewport.LineDown(lineHeight)
					}
				}
			}
		case "pageup":
			if m.showingConfigs {
				m.viewport.HalfViewUp()
				// Adjust cursor to match viewport position
				newCursor := m.viewport.YOffset / 2 // Account for line height
				if newCursor >= 0 {
					m.cursor = newCursor
				}
			}
		case "pagedown":
			if m.showingConfigs {
				m.viewport.HalfViewDown()
				// Adjust cursor to match viewport position
				newCursor := (m.viewport.YOffset + m.viewport.Height - 2) / 2 // Account for line height
				if newCursor < len(m.configs) {
					m.cursor = newCursor
				}
			}
		case "esc":
			if m.showingConfigs {
				m.showingConfigs = false
				m.cursor = 0
				m.selectedType = nil
				m.viewport.GotoTop()
			}
		case "enter":
			if !m.showingConfigs {
				m.selectedType = &m.gpuTypes[m.cursor]
				m.configs = getConfigsForType(m.selectedType.Configs, m.selectedType.Name)
				m.showingConfigs = true
				m.cursor = 0
				m.viewport.GotoTop()
				// Clear screen when switching to configs
				fmt.Print("\x1b[2J\x1b[H")
			} else {
				m.selectedConfig = &m.configs[m.cursor]
				return m, tea.Quit
			}
		}
	}

	// Ensure selected item is visible
	if m.showingConfigs {
		lineHeight := 2
		cursorPos := m.cursor * lineHeight
		viewportBottom := m.viewport.YOffset + m.viewport.Height - 2

		// If selected item is above viewport
		if cursorPos < m.viewport.YOffset {
			m.viewport.SetYOffset(cursorPos)
		}
		// If selected item is below viewport
		if cursorPos >= viewportBottom {
			m.viewport.SetYOffset(cursorPos - m.viewport.Height + 2)
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func formatPrice(price store.Price) string {
	// Convert price amount to float for proper decimal formatting
	var amount float64
	_, err := fmt.Sscanf(price.Amount, "%f", &amount)
	if err != nil {
		// If parsing fails, return the original amount
		return fmt.Sprintf("%s %s/hr", price.Currency, price.Amount)
	}

	// Format based on currency
	switch price.Currency {
	case "USD":
		return fmt.Sprintf("$%.2f/hr", amount)
	default:
		// For other currencies, keep a space between currency and amount
		return fmt.Sprintf("%s %.2f/hr", price.Currency, amount)
	}
}

func formatInstanceSpecs(config store.GPUConfig, instanceType store.GPUInstanceType, gpu store.SupportedGPU) string {
	// Left side: VRAM • RAM x CPU
	leftContent := fmt.Sprintf("%s VRAM • %s RAM x %d CPUs", 
		gpu.Memory,
		instanceType.Memory,
		instanceType.VCPU,
	)

	// Right side: CLOUD • Price
	rightContent := fmt.Sprintf("%s • %s", 
		strings.ToUpper(config.Provider),
		formatPrice(config.Price),
	)

	// Calculate padding needed between left and right content
	padding := configBoxStyle.GetWidth() - lipgloss.Width(leftContent) - lipgloss.Width(rightContent) - 2 // -2 for border padding

	return fmt.Sprintf("%s%s%s",
		leftContent,
		strings.Repeat(" ", padding),
		rightContent,
	)
}

func (m gpuModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.quitting {
		return "GPU selection cancelled\n"
	}
	if m.selectedConfig != nil {
		// Find the matching instance type and GPU info for the selected config
		var instanceType store.GPUInstanceType
		var gpu store.SupportedGPU
		for _, it := range m.selectedType.Configs {
			if it.Type == m.selectedConfig.Type {
				instanceType = it
				if len(it.SupportedGPUs) > 0 {
					gpu = it.SupportedGPUs[0]
				}
				break
			}
		}

		content := formatInstanceSpecs(*m.selectedConfig, instanceType, gpu)
		return lipgloss.JoinVertical(lipgloss.Left,
			gpuTitleStyle.Render("Selected Configuration:"),
			"",
			configSelectedBoxStyle.Render(content),
		)
	}

	var header string
	if !m.showingConfigs {
		// Create header with logo and title, added extra empty lines for top padding
		header = lipgloss.JoinVertical(lipgloss.Left,
			"", // Extra line for top padding
			"", // Extra line for top padding
			logoStyle.Render(NVIDIA_LOGO_2),
			"",  // Empty line for spacing
			gpuTitleStyle.Render("Select GPU Type:"),
		)
	} else {
		header = gpuTitleStyle.Render(fmt.Sprintf("Select %s Configuration:", m.selectedType.Name))
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
		configsByCount := make(map[int][]store.GPUConfig)
		for _, config := range m.configs {
			configsByCount[config.Count] = append(configsByCount[config.Count], config)
		}

		var sections []string
		counts := []int{1, 2, 4, 8}
		currentIndex := 0

		for _, count := range counts {
			if configs, exists := configsByCount[count]; exists {
				if currentIndex > 0 {
					sections = append(sections, "") // Add spacing between sections
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

					// Find the matching instance type and GPU info
					var instanceType store.GPUInstanceType
					var gpu store.SupportedGPU
					for _, it := range m.selectedType.Configs {
						if it.Type == config.Type {
							instanceType = it
							if len(it.SupportedGPUs) > 0 {
								gpu = it.SupportedGPUs[0]
							}
							break
						}
					}

					content := formatInstanceSpecs(config, instanceType, gpu)
					sections = append(sections, style.Render(content))
					currentIndex++
				}
			}
		}
		content = lipgloss.JoinVertical(lipgloss.Left, sections...)
	}

	m.viewport.SetContent(content)

	help := "\n↑/↓: Navigate • Enter: Select • ESC: Back • PgUp/PgDn: Scroll • q: Quit"

	// Join all sections vertically
	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		m.viewport.View(),
		help,
	)
}

func RunGPUPicker(types *store.InstanceTypeResponse) (*store.GPUConfig, error) {
	// Clear screen and move cursor to top before starting
	fmt.Print("\x1b[2J\x1b[H")
	
	m := initialGPUModel(types)
	p := tea.NewProgram(&m, tea.WithAltScreen())
	model, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("error running GPU picker: %v", err)
	}

	if m, ok := model.(gpuModel); ok && m.selectedConfig != nil {
		return m.selectedConfig, nil
	}
	return nil, fmt.Errorf("cancelled")
}
