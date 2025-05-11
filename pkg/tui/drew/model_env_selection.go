package drew

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/zhengkyl/pearls/scrollbar"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
	lipgloss_table "github.com/charmbracelet/lipgloss/table"
)

const (
	envListWidthPercentage    = 40.0
	envDetailsWidthPercentage = 60.0
)

// NewEnvSelection creates a new environment pick list model.
func NewEnvSelection() *EnvSelection {
	envSelection := &EnvSelection{}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		Foreground(textColorNormalTitle).
		Padding(0, 0, 0, 2)

	delegate.Styles.NormalDesc = delegate.Styles.NormalTitle.
		Foreground(textColorNormalDescription)

	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(borderColorSelected).
		Foreground(textColorSelectedTitle).
		Padding(0, 0, 0, 1)

	delegate.Styles.SelectedDesc = delegate.Styles.SelectedTitle.
		Foreground(textColorSelectedDescription)

	delegate.Styles.DimmedTitle = lipgloss.NewStyle().
		Foreground(textColorDimmedTitle).
		Padding(0, 0, 0, 2)

	delegate.Styles.DimmedDesc = delegate.Styles.DimmedTitle.
		Foreground(textColorDimmedDescription)

	delegate.Styles.FilterMatch = lipgloss.NewStyle().Underline(true)

	list := list.New([]list.Item{}, delegate, 0, 0)
	list.SetShowStatusBar(false)
	list.SetShowTitle(false)
	list.SetStatusBarItemName("environment", "environments")
	list.SetFilteringEnabled(false)
	list.SetShowHelp(false)
	list.DisableQuitKeybindings()
	envSelection.envList = list

	envSelectedViewport := viewport.New(100, 50)
	envSelectedViewport.KeyMap = viewport.KeyMap{
		Up: key.NewBinding(
			key.WithKeys("ctrl+k"),
		),
		Down: key.NewBinding(
			key.WithKeys("ctrl+j"),
		),
		HalfPageUp: key.NewBinding(
			key.WithKeys("ctrl+u"),
		),
		HalfPageDown: key.NewBinding(
			key.WithKeys("ctrl+d"),
		),
	}
	envSelection.envSelectedViewport = envSelectedViewport

	envStatusSpinner := spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
	)
	envSelection.statusSpinner = envStatusSpinner

	envSpinner := spinner.New(
		spinner.WithSpinner(spinner.Points),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("#64748b"))),
	)
	envSelection.loadingSpinner = envSpinner

	return envSelection
}

// EnvSelection is a model that represents the environment pick list. Note that this is not a complete
// charmbracelet/bubbles/list.Model, but rather a wrapper around it that adds some additional functionality
// while allowing for simplified use of the wrapped list.
type EnvSelection struct {
	envList             list.Model
	envSelectedViewport viewport.Model

	// A spinner model to use when rendering containers or environments
	statusSpinner spinner.Model

	// A spinner model to use when fetching environments
	showLoadingSpinner bool
	loadingSpinner     spinner.Model
}

func (e *EnvSelection) HelpTextEntries() [][]string {
	return [][]string{
		{"q/esc", "exit"},
		{"o", "select org"},
		{"↑/k", "up"},
		{"↓/j", "down"},
		{"ctrl+k", "details up"},
		{"ctrl+j", "details down"},
		{"ctrl+u", "details page up"},
		{"ctrl+d", "details page down"},
	}
}

// Width returns the width of the organization pick list.
func (e *EnvSelection) Width() int {
	return e.envList.Width()
}

func (e *EnvSelection) SetWidth(width int) {
	e.envList.SetWidth(width)
	e.envSelectedViewport.Width = width
}

// Height returns the height of the organization pick list.
func (e *EnvSelection) Height() int {
	return e.envList.Height()
}

func (e *EnvSelection) SetHeight(height int) {
	e.envList.SetHeight(height + 4)
	e.envSelectedViewport.Height = height
}

type envListItem struct {
	envSelection *EnvSelection
	environment  Environment
}

func (e envListItem) Title() string {
	status := e.environment.Status
	spinner := e.envSelection.statusSpinner

	renderedName := e.environment.Name
	renderedStatus := status.StatusView(spinner)

	// right-pad the width
	width := int(float64(e.envSelection.envList.Width()) * 0.4)
	pad := width - lipgloss.Width(renderedName) - lipgloss.Width(renderedStatus) - 3 // 1 to leave us on the same line, 2 for padding
	if pad < 1 {
		pad = 1
	}

	return fmt.Sprintf("%s%s%s", renderedName, strings.Repeat(" ", pad), renderedStatus)
}

func (e envListItem) Description() string {
	return fmt.Sprintf("%dx %s (%s) • %s", e.environment.InstanceType.GPUCount, e.environment.InstanceType.GPUModel, e.environment.InstanceType.VRAM, e.environment.InstanceType.Cloud.Name())
}

func (e envListItem) FilterValue() string { return e.environment.Name }

type (
	// EnvSelectionErrorMsg is a message that indicates an error occurred while fetching environments.
	EnvSelectionErrorMsg struct{ err error }
)

func envSelectionErrorCmd(err error) tea.Cmd {
	return func() tea.Msg { return EnvSelectionErrorMsg{err} }
}

func (e *EnvSelection) View() string {
	if e.showLoadingSpinner {
		spinner := fmt.Sprintf("Loading environments %s", e.loadingSpinner.View())

		// Create a vertically centered spinner box with full height
		loadingBox := lipgloss.NewStyle().
			Height(e.envList.Height()). // Match the table height
			Width(e.envList.Width()).   // Match the table width
			Align(lipgloss.Center).
			AlignVertical(lipgloss.Center).
			Render(spinner)

		return loadingBox
	}

	var selected *Environment
	if e.envList.SelectedItem() == nil {
		selected = nil
	} else {
		if selectedItem, ok := e.envList.SelectedItem().(envListItem); ok {
			selected = &selectedItem.environment
		} else {
			selected = nil
		}
	}

	// The list view should represent 40% of the total width.
	envListViewWidth := int(float64(e.envList.Width()) * 0.4)

	// The details view should represent 60% (59% because of rounding) of the total width. 
	// Why the "-4"? The scrollbar has limited  capabilities for width rendering, so we
	// save 4 columns for it... hacky!
	envDetailsViewWidth := int(float64(e.envList.Width()-4) * 0.59)

	// Fill the details view with the selected environment details
	e.envSelectedViewport.SetContent(e.renderEnvDetails(selected, envDetailsViewWidth))

	// Render the list view
	envListView := lipgloss.NewStyle().
		Width(envListViewWidth).
		Render(e.envList.View())

	// Render the details view
	envDetailsView := lipgloss.NewStyle().
		Width(envDetailsViewWidth).
		Border(lipgloss.RoundedBorder()).
		Render(e.envSelectedViewport.View())

	// Render the scrollbar
	scrollbar := scrollbar.New()
	scrollbar.Height = e.envSelectedViewport.Height + 2 // +2 because the scrollbar is dumb and wants to preserve 2 rows for itself. Another hack
	if e.envSelectedViewport.AtTop() && e.envSelectedViewport.AtBottom() {
		scrollbar.NumPos = 0
		scrollbar.Pos = 0
	} else {
		scrollbar.NumPos = 30
		scrollbar.Pos = int(e.envSelectedViewport.ScrollPercent() * 30)
	}

	// Join the list view, details view, and scrollbar horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, envListView, lipgloss.JoinHorizontal(lipgloss.Right, envDetailsView, scrollbar.View()))
}

func (e *EnvSelection) renderEnvDetails(environment *Environment, width int) string {
	if environment == nil {
		return ""
	}

	basicInfoTable := dataTable().
		Headers(lipgloss.NewStyle().Bold(true).Foreground(textColorSelectedTitle).Render("Environment")).
		Width(width).
		Rows([][]string{
			{lipgloss.NewStyle().Foreground(textColorDimmedTitle).Render("Name"), environment.Name},
			{lipgloss.NewStyle().Foreground(textColorDimmedTitle).Render("Status"), environment.Status.StatusView(e.statusSpinner)},
		}...).
		Render()

	instanceConfigurationTable := dataTable().
		Width(width).
		Headers(lipgloss.NewStyle().Bold(true).Foreground(textColorSelectedTitle).Render("Instance Configuration")).
		Rows([][]string{
			{lipgloss.NewStyle().Foreground(textColorDimmedTitle).Render("Cloud"), environment.InstanceType.Cloud.Name()},
			{lipgloss.NewStyle().Foreground(textColorDimmedTitle).Render("GPU"), environment.InstanceType.GPUModel},
			{lipgloss.NewStyle().Foreground(textColorDimmedTitle).Render("GPU Count"), fmt.Sprintf("%d", environment.InstanceType.GPUCount)},
			{lipgloss.NewStyle().Foreground(textColorDimmedTitle).Render("VRAM"), environment.InstanceType.VRAM},
			{lipgloss.NewStyle().Foreground(textColorDimmedTitle).Render("CPU"), environment.InstanceType.CPUModel},
			{lipgloss.NewStyle().Foreground(textColorDimmedTitle).Render("CPU Count"), fmt.Sprintf("%d", environment.InstanceType.CPUCount)},
			{lipgloss.NewStyle().Foreground(textColorDimmedTitle).Render("System RAM"), environment.InstanceType.Memory},
			{lipgloss.NewStyle().Foreground(textColorDimmedTitle).Render("Storage"), environment.InstanceType.Storage},
		}...).
		Render()

	var containersTable string
	if environment.Containers == nil {
		containersTable = ""
	} else {
		table := dataTable().
			Width(width).
			Headers(lipgloss.NewStyle().Bold(true).Foreground(textColorSelectedTitle).Render("Containers"))

		rows := [][]string{
			// Single header row
			{lipgloss.NewStyle().Foreground(textColorDimmedTitle).Render("Name"), lipgloss.NewStyle().Foreground(textColorDimmedTitle).Render("Status")},
		}
		for _, container := range environment.Containers {
			// New data row
			rows = append(rows, []string{container.Name, container.Status.StatusView(e.statusSpinner)})
		}

		// Finalize the table and convert to a string
		containersTable = "\n\n\n" + table.Rows(rows...).Render()
	}

	var portsTable string
	if environment.PortMappings == nil {
		portsTable = ""
	} else {
		table := dataTable().
			Width(width).
			Headers(lipgloss.NewStyle().Bold(true).Foreground(textColorSelectedTitle).Render("Public Ports"))

		rows := [][]string{
			// Single header row
			{lipgloss.NewStyle().Foreground(textColorDimmedTitle).Render("Host Port"), lipgloss.NewStyle().Foreground(textColorDimmedTitle).Render("Public Port")},
		}
		for _, mapping := range environment.PortMappings {
			// New data row
			rows = append(rows, []string{mapping.HostPort, mapping.PublicPort})
		}

		// Finalize the table and convert to a string
		portsTable = "\n\n\n" + table.Rows(rows...).Render()
	}

	var tunnelsTable string
	if environment.Tunnels == nil {
		tunnelsTable = ""
	} else {
		table := dataTable().
			Width(width).
			Headers(lipgloss.NewStyle().Bold(true).Foreground(textColorSelectedTitle).Render("Tunnels"))

		rows := [][]string{
			// Single header row
			{lipgloss.NewStyle().Foreground(textColorDimmedTitle).Render("Host Port"), lipgloss.NewStyle().Foreground(textColorDimmedTitle).Render("Public URL")},
		}
		for _, tunnel := range environment.Tunnels {
			// New data row
			rows = append(rows, []string{tunnel.HostPort, tunnel.PublicURL})
		}

		// Finalize the table and convert to a string
		tunnelsTable = "\n\n\n" + table.Rows(rows...).Render()
	}

	return fmt.Sprintf("%s\n\n\n%s%s%s%s", basicInfoTable, instanceConfigurationTable, portsTable, tunnelsTable, containersTable)
}

func dataTable() *lipgloss_table.Table {
	return lipgloss_table.New().
		Border(lipgloss.Border{
			Top:          "─",
			Bottom:       "─",
			Left:         " ",
			Right:        " ",
			TopLeft:      " ",
			TopRight:     " ",
			BottomLeft:   " ",
			BottomRight:  " ",
			MiddleLeft:   " ",
			MiddleRight:  " ",
			Middle:       " ",
			MiddleTop:    " ",
			MiddleBottom: " ",
		}).
		BorderRow(true).
		BorderColumn(false).
		BorderTop(false).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("238")))
}

func (e *EnvSelection) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	case fetchEnvsMsg:
		// The orgs have been fetched, so we need to update the org pick list model

		// Disable the loading spinner
		e.showLoadingSpinner = false

		if msg.err != nil {
			return envSelectionErrorCmd(msg.err)
		}

		// Insert the orgs into the org pick list model
		envListItems := make([]list.Item, len(msg.environments))
		for i, env := range msg.environments {
			envListItems[i] = envListItem{envSelection: e, environment: env}
		}

		if len(envListItems) > 0 {
			e.envList.SetShowStatusBar(true)
		}
		// Update the env pick list model with the new items
		e.envList.SetItems(envListItems)

		return nil

	case tea.KeyMsg:
		// We need to know if the user has changed the selection in the env list. If they have, we then need
		// to scroll to the top of the viewport, otherwise the viewport will remember the previous scroll position.
		previousSelection := e.envList.SelectedItem().(envListItem).environment.ID

		// Pass the key event to the env pick list model to allow for environment selection
		var keyCmds []tea.Cmd
		e.envList, cmd = e.envList.Update(msg)
		keyCmds = append(keyCmds, cmd)

		// If the selection has changed, scroll to the top of the viewport
		if previousSelection != e.envList.SelectedItem().(envListItem).environment.ID {
			e.envSelectedViewport.SetYOffset(0)
		}

		// Pass the key event to the env selection viewport to allow for viewport navigation
		e.envSelectedViewport, cmd = e.envSelectedViewport.Update(msg)
		keyCmds = append(keyCmds, cmd)

		return tea.Batch(keyCmds...)

	case spinner.TickMsg:
		if msg.ID == e.statusSpinner.ID() {
			e.statusSpinner, cmd = e.statusSpinner.Update(msg)
			return cmd
		}
		if msg.ID == e.loadingSpinner.ID() && e.showLoadingSpinner {
			e.loadingSpinner, cmd = e.loadingSpinner.Update(msg)
			return cmd
		}
	}

	return cmd
}

// FetchEnvs fetches the environments and updates the env pick list model. This function automatically
// starts the spinner and returns a command that will update the env pick list model when the environments
// are fetched. The returned command should be used to render the next frame for the spinner, and should
// also be used to update the env pick list model when the environments are fetched.
func (e *EnvSelection) FetchEnvs(organizationID string) tea.Cmd {
	e.envList.SetItems([]list.Item{})

	// Fetch the organizations
	fetchEnvsCmd := cmdFetchEnvs(organizationID)

	// Start the spinner
	e.showLoadingSpinner = true
	loadingSpinnerCmd := e.loadingSpinner.Tick

	// Start the env status spinner
	statusSpinnerCmd := e.statusSpinner.Tick

	return tea.Batch(fetchEnvsCmd, loadingSpinnerCmd, statusSpinnerCmd)
}

type fetchEnvsMsg struct {
	environments []Environment
	err          error
}

func cmdFetchEnvs(organizationID string) tea.Cmd {
	return func() tea.Msg {
		environments := fetchEnvs(organizationID)

		// Sort the environments by status
		sort.Slice(environments, func(i, j int) bool {
			return environments[i].Status < environments[j].Status
		})

		return fetchEnvsMsg{environments: environments, err: nil}
	}
}

func fetchEnvs(organizationID string) []Environment {
	// simulate loading
	time.Sleep(time.Second * 1)

	return []Environment{
		{ID: "1", Name: "my-cool-env", InstanceType: Crusoe_1x_a100_40gb, Status: EnvironmentStatusRunning, PortMappings: []PortMapping{{"22", "22"}, {"8080", "80"}}, Tunnels: []Tunnel{{"443", "https://foo.bar.com"}}},
		{ID: "2", Name: "testing-crusoe", InstanceType: Crusoe_2x_a100_40gb, Status: EnvironmentStatusRunning, PortMappings: []PortMapping{{"8080", "80"}, {"9000", "8080"}}},
		{ID: "3", Name: "building-lambda", InstanceType: Lambda_1x_a100_40gb, Status: EnvironmentStatusBuilding, PortMappings: []PortMapping{{"22", "22"}}},
		{ID: "4", Name: "test-error-lambda", InstanceType: Lambda_1x_a100_40gb, Status: EnvironmentStatusError, Containers: []Container{{Name: "jupyter", Image: "jupyter:latest", Status: ContainerStatusError}}},
		{ID: "5", Name: "test-crusoe-running", InstanceType: Crusoe_1x_a100_40gb, Status: EnvironmentStatusRunning},
		{ID: "6", Name: "test-lambda-running", InstanceType: Lambda_2x_a100_40gb, Status: EnvironmentStatusRunning},
		{ID: "7", Name: "test-crusoe-starting", InstanceType: Crusoe_1x_a100_40gb, Status: EnvironmentStatusStarting, Containers: []Container{{Name: "jupyter", Image: "jupyter:latest", Status: ContainerStatusBuilding}}},
		{ID: "8", Name: "my-awesome-gpu", InstanceType: Lambda_2x_a100_40gb, Status: EnvironmentStatusStarting},
		{ID: "9", Name: "my-awesome-gpu-2", InstanceType: Crusoe_1x_a100_40gb, Status: EnvironmentStatusStopped},
		{ID: "10", Name: "my-awesome-gpu-3", InstanceType: Lambda_1x_a100_40gb, Status: EnvironmentStatusStopped},
		{ID: "11", Name: "env-12", InstanceType: Crusoe_1x_a100_40gb, Status: EnvironmentStatusDeleting},
		{ID: "12", Name: "env-13", InstanceType: Lambda_1x_a100_40gb, Status: EnvironmentStatusDeleting},
	}
}

var logoSmall = `[38;2;0;0;0;48;2;0;0;0m▞[0m[38;2;0;0;0;48;2;0;0;0m▘[0m[38;2;0;0;0;48;2;0;0;0m▗[0m[38;2;0;0;0;48;2;0;0;0m▞[0m[38;2;0;0;0;48;2;0;0;0m▗[0m[38;2;20;32;0;48;2;0;0;0m▗[0m[38;2;0;0;0;48;2;39;65;0m▀[0m[38;2;104;162;0;48;2;50;85;0m▐[0m[38;2;119;185;0;48;2;83;129;0m▀[0m[38;2;119;185;0;48;2;96;152;0m▀[0m[38;2;110;171;0;48;2;119;185;0m▖[0m[38;2;118;184;0;48;2;119;185;0m▖[0m[38;2;119;185;0;48;2;119;185;0m▗[0m[38;2;119;185;0;48;2;119;185;0m▗[0m[38;2;119;185;0;48;2;119;185;0m▘[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▗[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▝[0m[38;2;119;185;0;48;2;119;185;0m▞[0m
[38;2;0;0;0;48;2;0;0;0m▖[0m[38;2;47;75;0;48;2;4;8;0m▗[0m[38;2;0;0;0;48;2;94;150;0m▀[0m[38;2;36;60;0;48;2;103;163;0m▀[0m[38;2;89;143;0;48;2;43;71;0m▀[0m[38;2;111;173;0;48;2;13;27;0m▀[0m[38;2;85;139;0;48;2;42;65;0m▀[0m[38;2;72;120;0;48;2;84;131;0m▞[0m[38;2;65;104;0;48;2;79;121;0m▀[0m[38;2;44;73;0;48;2;99;159;0m▀[0m[38;2;18;31;0;48;2;113;177;0m▀[0m[38;2;68;108;0;48;2;36;63;0m▞[0m[38;2;101;159;0;48;2;7;13;0m▀[0m[38;2;119;185;0;48;2;56;95;0m▀[0m[38;2;113;178;0;48;2;119;185;0m▖[0m[38;2;119;185;0;48;2;119;185;0m▝[0m[38;2;119;185;0;48;2;119;185;0m▖[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▝[0m[38;2;119;185;0;48;2;119;185;0m▗[0m
[38;2;38;65;0;48;2;93;150;0m▘[0m[38;2;117;182;0;48;2;118;185;0m▘[0m[38;2;0;0;0;48;2;77;130;0m▗[0m[38;2;91;147;0;48;2;8;14;0m▗[0m[38;2;62;102;0;48;2;118;184;0m▀[0m[38;2;17;27;0;48;2;108;171;0m▗[0m[38;2;84;135;0;48;2;0;0;0m▀[0m[38;2;120;187;0;48;2;64;101;0m▗[0m[38;2;31;51;0;48;2;117;182;0m▀[0m[38;2;64;102;0;48;2;0;0;0m▖[0m[38;2;60;97;0;48;2;18;32;0m▐[0m[38;2;118;184;0;48;2;105;165;0m▐[0m[38;2;73;125;0;48;2;113;178;0m▐[0m[38;2;0;0;0;48;2;16;30;0m▐[0m[38;2;65;105;0;48;2;15;25;0m▐[0m[38;2;118;184;0;48;2;110;172;0m▐[0m[38;2;119;185;0;48;2;119;185;0m▐[0m[38;2;119;185;0;48;2;119;185;0m▗[0m[38;2;119;185;0;48;2;119;185;0m▖[0m[38;2;119;185;0;48;2;119;185;0m▞[0m
[38;2;60;101;0;48;2;0;0;0m▝[0m[38;2;30;48;0;48;2;111;174;0m▖[0m[38;2;70;115;0;48;2;118;184;0m▝[0m[38;2;97;157;0;48;2;17;31;0m▖[0m[38;2;104;165;0;48;2;21;38;0m▀[0m[38;2;81;133;0;48;2;118;185;0m▞[0m[38;2;4;9;0;48;2;89;144;0m▀[0m[38;2;116;181;0;48;2;52;83;0m▐[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;94;146;0;48;2;111;175;0m▗[0m[38;2;117;183;0;48;2;52;88;0m▀[0m[38;2;92;145;0;48;2;26;43;0m▀[0m[38;2;11;17;0;48;2;89;143;0m▀[0m[38;2;34;56;0;48;2;105;167;0m▘[0m[38;2;68;107;0;48;2;113;177;0m▗[0m[38;2;108;171;0;48;2;12;22;0m▀[0m[38;2;79;126;0;48;2;0;0;0m▀[0m[38;2;117;182;0;48;2;20;36;0m▀[0m[38;2;93;154;0;48;2;119;185;0m▖[0m[38;2;119;185;0;48;2;119;185;0m▀[0m
[38;2;0;0;0;48;2;0;0;0m▖[0m[38;2;0;0;0;48;2;0;0;0m▞[0m[38;2;70;110;0;48;2;0;0;0m▀[0m[38;2;118;184;0;48;2;39;63;0m▀[0m[38;2;48;77;0;48;2;97;155;0m▝[0m[38;2;16;30;0;48;2;108;172;0m▀[0m[38;2;76;119;0;48;2;56;93;0m▞[0m[38;2;63;102;0;48;2;96;152;0m▞[0m[38;2;46;78;0;48;2;94;146;0m▀[0m[38;2;54;87;0;48;2;83;133;0m▀[0m[38;2;96;157;0;48;2;67;109;0m▝[0m[38;2;116;182;0;48;2;31;52;0m▀[0m[38;2;102;159;0;48;2;4;9;0m▀[0m[38;2;25;40;0;48;2;52;81;0m▞[0m[38;2;0;0;0;48;2;68;110;0m▀[0m[38;2;0;0;0;48;2;104;163;0m▀[0m[38;2;36;58;0;48;2;118;185;0m▀[0m[38;2;80;127;0;48;2;116;181;0m▘[0m[38;2;118;184;0;48;2;119;185;0m▘[0m[38;2;119;185;0;48;2;119;185;0m▞[0m
[38;2;0;0;0;48;2;0;0;0m▝[0m[38;2;0;0;0;48;2;0;0;0m▘[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▞[0m[38;2;0;0;0;48;2;0;0;0m▖[0m[38;2;29;52;0;48;2;0;0;0m▀[0m[38;2;64;102;0;48;2;0;0;0m▀[0m[38;2;48;80;0;48;2;103;162;0m▞[0m[38;2;59;89;0;48;2;119;185;0m▀[0m[38;2;60;97;0;48;2;119;185;0m▀[0m[38;2;75;121;0;48;2;119;185;0m▀[0m[38;2;96;153;0;48;2;119;185;0m▀[0m[38;2;113;176;0;48;2;119;185;0m▘[0m[38;2;118;184;0;48;2;119;185;0m▘[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▝[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▝[0m[38;2;119;185;0;48;2;119;185;0m▝[0m`

var logoLarge = `[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;75;123;0;48;2;75;123;0m▀[0m[38;2;118;185;0;48;2;118;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m[38;2;119;186;0;48;2;119;185;0m▀[0m
[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;75;123;0;48;2;75;131;0m▀[0m[38;2;118;185;0;48;2;118;184;0m▀[0m[38;2;119;185;0;48;2;118;184;0m▀[0m[38;2;119;185;0;48;2;118;184;0m▀[0m[38;2;119;185;0;48;2;119;184;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m
[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;22;32;0m▀[0m[38;2;0;0;0;48;2;78;127;0m▀[0m[38;2;0;0;0;48;2;107;167;0m▀[0m[38;2;31;52;0;48;2;119;185;0m▀[0m[38;2;56;97;0;48;2;118;184;0m▀[0m[38;2;80;134;0;48;2;119;185;0m▀[0m[38;2;91;147;0;48;2;119;185;0m▀[0m[38;2;89;140;0;48;2;89;140;0m▀[0m[38;2;63;103;0;48;2;21;31;0m▀[0m[38;2;54;89;0;48;2;0;0;0m▀[0m[38;2;56;87;0;48;2;0;0;0m▀[0m[38;2;65;105;0;48;2;0;0;0m▀[0m[38;2;80;127;0;48;2;0;0;0m▀[0m[38;2;95;149;0;48;2;0;0;0m▀[0m[38;2;112;175;0;48;2;0;0;0m▀[0m[38;2;119;185;0;48;2;47;73;0m▀[0m[38;2;119;185;0;48;2;80;133;0m▀[0m[38;2;119;185;0;48;2;115;179;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m
[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;36;65;0m▀[0m[38;2;0;0;0;48;2;93;147;0m▀[0m[38;2;31;52;0;48;2;119;185;0m▀[0m[38;2;88;131;0;48;2;119;184;0m▀[0m[38;2;119;185;0;48;2;118;185;0m▀[0m[38;2;118;184;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;97;152;0m▀[0m[38;2;118;184;0;48;2;72;107;0m▀[0m[38;2;119;185;0;48;2;38;73;0m▀[0m[38;2;118;185;0;48;2;10;26;0m▀[0m[38;2;108;165;0;48;2;0;0;0m▀[0m[38;2;89;140;0;48;2;79;129;0m▀[0m[38;2;75;139;0;48;2;118;184;0m▀[0m[38;2;65;121;0;48;2;118;184;0m▀[0m[38;2;65;97;0;48;2;118;184;0m▀[0m[38;2;42;87;0;48;2;119;186;0m▀[0m[38;2;5;22;0;48;2;119;185;0m▀[0m[38;2;0;0;0;48;2;99;152;0m▀[0m[38;2;0;0;0;48;2;63;105;0m▀[0m[38;2;0;0;0;48;2;17;20;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;16;26;0;48;2;0;0;0m▀[0m[38;2;72;113;0;48;2;0;0;0m▀[0m[38;2;118;184;0;48;2;18;35;0m▀[0m[38;2;119;185;0;48;2;77;124;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m
[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;56;91;0m▀[0m[38;2;18;28;0;48;2;113;176;0m▀[0m[38;2;85;147;0;48;2;118;184;0m▀[0m[38;2;118;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;118;185;0m▀[0m[38;2;119;185;0;48;2;105;165;0m▀[0m[38;2;119;185;0;48;2;47;79;0m▀[0m[38;2;81;137;0;48;2;0;0;0m▀[0m[38;2;36;64;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;40;58;0m▀[0m[38;2;0;0;0;48;2;60;117;0m▀[0m[38;2;75;123;0;48;2;88;147;0m▀[0m[38;2;118;184;0;48;2;95;140;0m▀[0m[38;2;119;184;0;48;2;95;137;0m▀[0m[38;2;119;185;0;48;2;93;142;0m▀[0m[38;2;119;185;0;48;2;97;154;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;118;185;0m▀[0m[38;2;118;185;0;48;2;119;185;0m▀[0m[38;2;113;176;0;48;2;119;185;0m▀[0m[38;2;64;125;0;48;2;118;184;0m▀[0m[38;2;10;16;0;48;2;103;161;0m▀[0m[38;2;0;0;0;48;2;22;52;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;28;50;0;48;2;0;0;0m▀[0m[38;2;111;173;0;48;2;12;31;0m▀[0m[38;2;119;185;0;48;2;95;152;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m
[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;40;73;0m▀[0m[38;2;0;0;0;48;2;101;166;0m▀[0m[38;2;81;127;0;48;2;118;185;0m▀[0m[38;2;118;184;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;115;179;0m▀[0m[38;2;118;185;0;48;2;60;102;0m▀[0m[38;2;80;135;0;48;2;0;0;0m▀[0m[38;2;11;26;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;20;40;0m▀[0m[38;2;0;0;0;48;2;81;133;0m▀[0m[38;2;18;36;0;48;2;119;185;0m▀[0m[38;2;75;130;0;48;2;118;184;0m▀[0m[38;2;107;167;0;48;2;119;185;0m▀[0m[38;2;118;185;0;48;2;119;185;0m▀[0m[38;2;119;184;0;48;2;118;185;0m▀[0m[38;2;89;140;0;48;2;89;140;0m▀[0m[38;2;21;31;0;48;2;22;36;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;14;40;0;48;2;0;0;0m▀[0m[38;2;77;127;0;48;2;0;0;0m▀[0m[38;2;115;179;0;48;2;13;22;0m▀[0m[38;2;118;184;0;48;2;84;139;0m▀[0m[38;2;119;185;0;48;2;118;184;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;107;166;0;48;2;119;185;0m▀[0m[38;2;55;85;0;48;2;115;179;0m▀[0m[38;2;0;0;0;48;2;55;116;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;89;139;0;48;2;0;0;0m▀[0m[38;2;118;184;0;48;2;73;121;0m▀[0m[38;2;119;185;0;48;2;118;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m
[38;2;51;104;0;48;2;115;187;0m▀[0m[38;2;115;179;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;71;120;0m▀[0m[38;2;40;93;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;81;128;0m▀[0m[38;2;63;112;0;48;2;118;185;0m▀[0m[38;2;115;179;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;113;178;0m▀[0m[38;2;118;184;0;48;2;65;96;0m▀[0m[38;2;95;159;0;48;2;0;0;0m▀[0m[38;2;58;87;0;48;2;0;0;0m▀[0m[38;2;16;31;0;48;2;0;0;0m▀[0m[38;2;77;129;0;48;2;81;127;0m▀[0m[38;2;118;185;0;48;2;119;184;0m▀[0m[38;2;85;143;0;48;2;118;184;0m▀[0m[38;2;22;48;0;48;2;111;171;0m▀[0m[38;2;0;0;0;48;2;36;47;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;59;88;0;48;2;0;0;0m▀[0m[38;2;113;179;0;48;2;101;158;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;118;185;0;48;2;119;185;0m▀[0m[38;2;68;110;0;48;2;111;181;0m▀[0m[38;2;0;0;0;48;2;12;31;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;73;117;0;48;2;24;35;0m▀[0m[38;2;119;185;0;48;2;118;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m
[38;2;92;139;0;48;2;16;52;0m▀[0m[38;2;119;185;0;48;2;115;179;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;72;119;0;48;2;110;173;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;23;34;0;48;2;0;0;0m▀[0m[38;2;115;179;0;48;2;91;147;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;118;184;0m▀[0m[38;2;52;85;0;48;2;60;113;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;81;127;0;48;2;81;127;0m▀[0m[38;2;119;184;0;48;2;119;184;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;108;169;0;48;2;119;185;0m▀[0m[38;2;18;27;0;48;2;89;147;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;55;86;0m▀[0m[38;2;23;38;0;48;2;107;167;0m▀[0m[38;2;91;154;0;48;2;119;185;0m▀[0m[38;2;118;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;118;184;0m▀[0m[38;2;115;179;0;48;2;65;102;0m▀[0m[38;2;54;95;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;30;46;0m▀[0m[38;2;25;38;0;48;2;111;173;0m▀[0m[38;2;103;163;0;48;2;119;185;0m▀[0m[38;2;119;184;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m
[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;86;140;0;48;2;17;36;0m▀[0m[38;2;119;185;0;48;2;109;170;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;118;185;0;48;2;119;185;0m▀[0m[38;2;60;97;0;48;2;111;174;0m▀[0m[38;2;0;0;0;48;2;18;40;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;30;56;0;48;2;0;0;0m▀[0m[38;2;118;184;0;48;2;81;136;0m▀[0m[38;2;119;185;0;48;2;118;184;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;105;166;0;48;2;119;185;0m▀[0m[38;2;0;0;0;48;2;71;120;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;81;127;0;48;2;81;127;0m▀[0m[38;2;119;184;0;48;2;119;184;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;118;185;0;48;2;119;185;0m▀[0m[38;2;73;131;0;48;2;119;185;0m▀[0m[38;2;113;179;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;118;184;0m▀[0m[38;2;119;185;0;48;2;113;177;0m▀[0m[38;2;118;184;0;48;2;60;95;0m▀[0m[38;2;72;111;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;63;95;0m▀[0m[38;2;28;46;0;48;2;109;177;0m▀[0m[38;2;111;172;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;118;185;0m▀[0m[38;2;119;185;0;48;2;105;163;0m▀[0m[38;2;119;185;0;48;2;56;94;0m▀[0m[38;2;119;185;0;48;2;86;147;0m▀[0m[38;2;119;185;0;48;2;119;184;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m
[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;36;52;0;48;2;0;0;0m▀[0m[38;2;118;184;0;48;2;79;131;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;95;156;0;48;2;118;184;0m▀[0m[38;2;0;0;0;48;2;40;68;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;14;26;0;48;2;0;0;0m▀[0m[38;2;103;159;0;48;2;28;55;0m▀[0m[38;2;119;184;0;48;2;107;167;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;184;0;48;2;119;185;0m▀[0m[38;2;69;110;0;48;2;119;185;0m▀[0m[38;2;0;0;0;48;2;64;112;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;81;127;0;48;2;81;127;0m▀[0m[38;2;119;184;0;48;2;119;184;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;118;184;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;118;185;0;48;2;81;140;0m▀[0m[38;2;107;165;0;48;2;13;30;0m▀[0m[38;2;31;56;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;22;44;0m▀[0m[38;2;10;27;0;48;2;97;161;0m▀[0m[38;2;76;117;0;48;2;118;184;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;118;185;0m▀[0m[38;2;118;184;0;48;2;79;128;0m▀[0m[38;2;92;143;0;48;2;0;0;0m▀[0m[38;2;16;27;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;39;62;0;48;2;0;0;0m▀[0m[38;2;96;159;0;48;2;0;0;0m▀[0m[38;2;118;185;0;48;2;54;93;0m▀[0m[38;2;119;185;0;48;2;111;170;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m
[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;93;149;0;48;2;23;36;0m▀[0m[38;2;118;184;0;48;2;107;167;0m▀[0m[38;2;119;185;0;48;2;118;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;113;177;0;48;2;119;185;0m▀[0m[38;2;36;56;0;48;2;112;176;0m▀[0m[38;2;0;0;0;48;2;32;52;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;23;48;0;48;2;0;0;0m▀[0m[38;2;109;170;0;48;2;14;31;0m▀[0m[38;2;118;184;0;48;2;85;147;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;118;184;0;48;2;119;185;0m▀[0m[38;2;97;163;0;48;2;119;185;0m▀[0m[38;2;91;143;0;48;2;89;140;0m▀[0m[38;2;119;184;0;48;2;36;68;0m▀[0m[38;2;119;186;0;48;2;39;76;0m▀[0m[38;2;120;186;0;48;2;40;73;0m▀[0m[38;2;119;185;0;48;2;18;36;0m▀[0m[38;2;108;170;0;48;2;0;0;0m▀[0m[38;2;83;124;0;48;2;0;0;0m▀[0m[38;2;21;34;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;10;30;0m▀[0m[38;2;0;0;0;48;2;77;116;0m▀[0m[38;2;8;14;0;48;2;111;173;0m▀[0m[38;2;73;121;0;48;2;118;185;0m▀[0m[38;2;113;177;0;48;2;119;185;0m▀[0m[38;2;118;184;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;118;185;0m▀[0m[38;2;118;184;0;48;2;76;117;0m▀[0m[38;2;109;168;0;48;2;12;16;0m▀[0m[38;2;51;94;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;16;24;0m▀[0m[38;2;0;0;0;48;2;81;138;0m▀[0m[38;2;56;112;0;48;2;118;184;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m
[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;21;34;0;48;2;0;0;0m▀[0m[38;2;104;166;0;48;2;19;30;0m▀[0m[38;2;119;184;0;48;2;94;151;0m▀[0m[38;2;119;185;0;48;2;118;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;112;176;0;48;2;119;185;0m▀[0m[38;2;56;87;0;48;2;119;185;0m▀[0m[38;2;0;0;0;48;2;81;135;0m▀[0m[38;2;0;0;0;48;2;20;26;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;35;80;0;48;2;0;0;0m▀[0m[38;2;84;140;0;48;2;0;0;0m▀[0m[38;2;115;179;0;48;2;5;28;0m▀[0m[38;2;89;140;0;48;2;88;134;0m▀[0m[38;2;0;0;0;48;2;102;160;0m▀[0m[38;2;0;0;0;48;2;97;154;0m▀[0m[38;2;0;0;0;48;2;97;154;0m▀[0m[38;2;0;0;0;48;2;105;165;0m▀[0m[38;2;12;25;0;48;2;119;185;0m▀[0m[38;2;52;79;0;48;2;119;186;0m▀[0m[38;2;80;137;0;48;2;118;184;0m▀[0m[38;2;115;179;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;94;149;0m▀[0m[38;2;119;185;0;48;2;55;87;0m▀[0m[38;2;105;166;0;48;2;0;0;0m▀[0m[38;2;48;80;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;15;32;0m▀[0m[38;2;0;0;0;48;2;62;105;0m▀[0m[38;2;0;0;0;48;2;105;162;0m▀[0m[38;2;56;97;0;48;2;118;185;0m▀[0m[38;2;111;173;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m
[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;79;129;0;48;2;0;0;0m▀[0m[38;2;118;185;0;48;2;40;59;0m▀[0m[38;2;119;185;0;48;2;107;167;0m▀[0m[38;2;119;185;0;48;2;118;185;0m▀[0m[38;2;118;184;0;48;2;119;185;0m▀[0m[38;2;115;179;0;48;2;119;185;0m▀[0m[38;2;76;139;0;48;2;118;185;0m▀[0m[38;2;22;32;0;48;2;119;185;0m▀[0m[38;2;0;0;0;48;2;95;159;0m▀[0m[38;2;0;0;0;48;2;73;118;0m▀[0m[38;2;81;127;0;48;2;96;151;0m▀[0m[38;2;119;184;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;120;186;0m▀[0m[38;2;119;185;0;48;2;119;187;0m▀[0m[38;2;119;185;0;48;2;118;184;0m▀[0m[38;2;119;185;0;48;2;102;157;0m▀[0m[38;2;119;185;0;48;2;89;140;0m▀[0m[38;2;118;184;0;48;2;69;130;0m▀[0m[38;2;119;185;0;48;2;42;83;0m▀[0m[38;2;113;175;0;48;2;0;0;0m▀[0m[38;2;81;139;0;48;2;0;0;0m▀[0m[38;2;36;73;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;34;56;0m▀[0m[38;2;0;0;0;48;2;81;124;0m▀[0m[38;2;0;0;0;48;2;107;167;0m▀[0m[38;2;42;81;0;48;2;118;185;0m▀[0m[38;2;86;150;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;118;184;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m
[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;65;119;0;48;2;0;0;0m▀[0m[38;2;109;172;0;48;2;9;20;0m▀[0m[38;2;119;185;0;48;2;43;85;0m▀[0m[38;2;118;185;0;48;2;75;134;0m▀[0m[38;2;119;185;0;48;2;102;159;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;184;0;48;2;119;186;0m▀[0m[38;2;89;140;0;48;2;89;140;0m▀[0m[38;2;22;35;0;48;2;26;40;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;31;55;0m▀[0m[38;2;0;0;0;48;2;44;81;0m▀[0m[38;2;0;0;0;48;2;58;105;0m▀[0m[38;2;0;0;0;48;2;84;130;0m▀[0m[38;2;0;0;0;48;2;95;149;0m▀[0m[38;2;8;21;0;48;2;118;185;0m▀[0m[38;2;42;80;0;48;2;119;186;0m▀[0m[38;2;80;120;0;48;2;119;185;0m▀[0m[38;2;107;166;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;118;184;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m
[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;10;24;0;48;2;0;0;0m▀[0m[38;2;42;73;0;48;2;0;0;0m▀[0m[38;2;84;134;0;48;2;75;121;0m▀[0m[38;2;95;147;0;48;2;118;184;0m▀[0m[38;2;94;147;0;48;2;119;185;0m▀[0m[38;2;94;147;0;48;2;119;185;0m▀[0m[38;2;96;149;0;48;2;119;185;0m▀[0m[38;2;113;173;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;118;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m[38;2;119;185;0;48;2;119;185;0m▀[0m
[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;0;0;0;48;2;0;0;0m▀[0m[38;2;75;121;0;48;2;73;121;0m▀[0m[38;2;118;184;0;48;2;118;185;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m[38;2;119;185;0;48;2;119;186;0m▀[0m`
