package test

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/gdamore/tcell/v2"

	"github.com/spf13/cobra"
)

var (
	colorWhite    = tcell.NewRGBColor(255, 255, 255)
	colorGray     = tcell.NewRGBColor(128, 128, 128)
	colorYellow   = tcell.NewRGBColor(255, 255, 0)
	colorCyan     = tcell.NewRGBColor(0, 255, 255)
	colorCyanBase = tcell.NewRGBColor(0, 255, 255)
)

var (
	startLong    = "[internal] GPU Instance Picker"
	startExample = "[internal] GPU Instance Picker"
)

// UI package
type Element struct {
	Content string
	Style   tcell.Style
	X, Y    int
}

type Layout struct {
	Elements []Element
	Screen   tcell.Screen
}

func NewLayout(s tcell.Screen) *Layout {
	return &Layout{Screen: s}
}

func (l *Layout) AddElement(content string, style tcell.Style, x, y int) {
	l.Elements = append(l.Elements, Element{
		Content: content,
		Style:   style,
		X:       x,
		Y:       y,
	})
}
func (l *Layout) Render() {
	l.Screen.Clear()
	for _, elem := range l.Elements {
		for i, r := range elem.Content {
			l.Screen.SetContent(elem.X+i, elem.Y, r, nil, elem.Style)
		}
	}
	l.Screen.Show()
}

// LaunchableCreator struct to hold state
type LaunchableCreator struct {
	Screen                 tcell.Screen
	Chips                  []string
	Containers             []string
	SelectedChipIndex      int
	SelectedContainerIndex int
	CurrentStep            int
	URLInput               string
	ExposedPort            string
	ExposePort             bool
}

func NewLaunchableCreator(screen tcell.Screen) *LaunchableCreator {
	return &LaunchableCreator{
		Screen:     screen,
		Chips:      []string{"A100", "H100", "L40S"},
		Containers: []string{"Container 1", "Container 2", "Container 3", "Container 4"},
	}
}

func (lc *LaunchableCreator) Render() {
	layout := NewLayout(lc.Screen)

	// Add title
	layout.AddElement("Create Launchable", tcell.StyleDefault.Foreground(tcell.ColorWhite).Bold(true), 1, 1)

	// Add step indicators
	steps := []string{"Select Chip", "Select Container", "Enter URL", "Expose Port", "Review"}
	for i, step := range steps {
		style := tcell.StyleDefault.Foreground(tcell.ColorGray)
		if i == lc.CurrentStep {
			style = style.Foreground(tcell.ColorWhite).Bold(true)
		}
		layout.AddElement(step, style, i*20+1, 2)
	}

	// Render step-specific content
	switch lc.CurrentStep {
	case 0:
		lc.renderChipSelection(layout)
	case 1:
		lc.renderContainerSelection(layout)
	case 2:
		lc.renderURLInput(layout)
	case 3:
		lc.renderPortInput(layout)
	case 4:
		lc.renderReview(layout)
	}

	layout.Render()
}

func (lc *LaunchableCreator) renderChipSelection(layout *Layout) {
	for i, chip := range lc.Chips {
		style := tcell.StyleDefault.Foreground(tcell.ColorGray)
		selected := i == lc.SelectedChipIndex
		if selected {
			style = style.Foreground(colorCyanBase).Bold(true)
		}
		drawBox(layout, i*16+1, 5, 11, 5, chip, style, selected)
	}
}

func (lc *LaunchableCreator) renderContainerSelection(layout *Layout) {
	for i, container := range lc.Containers {
		style := tcell.StyleDefault.Foreground(tcell.ColorGray)
		selected := i == lc.SelectedContainerIndex
		if selected {
			style = style.Foreground(colorCyanBase).Bold(true)
		}
		drawBox(layout, (i%2)*35+1, (i/2)*4+5, 25, 3, container, style, selected)
	}
}

func (lc *LaunchableCreator) renderURLInput(layout *Layout) {
	prompt := "Enter URL (.ipynb or git repo): "
	layout.AddElement(prompt+lc.URLInput+"_", tcell.StyleDefault.Foreground(tcell.ColorWhite), 1, 5)
}

func (lc *LaunchableCreator) renderPortInput(layout *Layout) {
	prompt := "Do you want to expose a port? (y/n): "
	layout.AddElement(prompt, tcell.StyleDefault.Foreground(tcell.ColorWhite), 1, 5)
	if lc.ExposePort {
		layout.AddElement("Y", tcell.StyleDefault.Foreground(colorCyanBase), len(prompt)+1, 5)
		portPrompt := "Enter port number: "
		layout.AddElement(portPrompt+lc.ExposedPort+"_", tcell.StyleDefault.Foreground(tcell.ColorWhite), 1, 7)
	} else {
		layout.AddElement("N", tcell.StyleDefault.Foreground(colorCyanBase), len(prompt)+1, 5)
	}
}

func (lc *LaunchableCreator) renderReview(layout *Layout) {
	layout.AddElement("🤙 Chip: "+lc.Chips[lc.SelectedChipIndex], tcell.StyleDefault.Foreground(colorCyanBase).Bold(true), 1, 5)
	layout.AddElement("📦 Container: "+lc.Containers[lc.SelectedContainerIndex], tcell.StyleDefault.Foreground(colorCyanBase).Bold(true), 1, 7)
	layout.AddElement("🔗 URL: "+lc.URLInput, tcell.StyleDefault.Foreground(colorCyanBase).Bold(true), 1, 9)
	if lc.ExposePort {
		layout.AddElement("🔌 Exposed Port: "+lc.ExposedPort, tcell.StyleDefault.Foreground(colorCyanBase).Bold(true), 1, 11)
	} else {
		layout.AddElement("🔌 No Port Exposed", tcell.StyleDefault.Foreground(colorCyanBase).Bold(true), 1, 11)
	}
	layout.AddElement("----------------------------------------", tcell.StyleDefault.Foreground(colorCyanBase), 1, 13)
	layout.AddElement("Press Enter to confirm and create the launchable", tcell.StyleDefault.Foreground(tcell.ColorWhite), 1, 15)
}

func drawBox(layout *Layout, x, y, width, height int, content string, style tcell.Style, selected bool) {
	// Draw top and bottom borders
	for i := 0; i < width; i++ {
		layout.AddElement("-", style, x+i, y)
		layout.AddElement("-", style, x+i, y+height-1)
	}
	// Draw side borders
	for i := 1; i < height-1; i++ {
		layout.AddElement("|", style, x, y+i)
		layout.AddElement("|", style, x+width-1, y+i)
	}
	// Draw content
	layout.AddElement(content, style, x+1+(width-2-len(content))/2, y+height/2)

	// Draw caret if selected
	if selected {
		caretStyle := tcell.StyleDefault.Foreground(colorYellow)
		layout.AddElement(">", caretStyle, x-1, y+height/2)
	}
}

func (lc *LaunchableCreator) HandleInput(ev *tcell.EventKey) bool {
	switch lc.CurrentStep {
	case 0:
		return lc.handleChipSelection(ev)
	case 1:
		return lc.handleContainerSelection(ev)
	case 2:
		return lc.handleURLInput(ev)
	case 3:
		return lc.handlePortInput(ev)
	case 4:
		return lc.handleReview(ev)
	}
	return false
}

func (lc *LaunchableCreator) handleChipSelection(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyLeft:
		if lc.SelectedChipIndex > 0 {
			lc.SelectedChipIndex--
		}
	case tcell.KeyRight:
		if lc.SelectedChipIndex < len(lc.Chips)-1 {
			lc.SelectedChipIndex++
		}
	case tcell.KeyEnter:
		lc.CurrentStep++
		return true
	}
	return false
}

func (lc *LaunchableCreator) handleContainerSelection(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyLeft:
		if lc.SelectedContainerIndex > 0 {
			lc.SelectedContainerIndex--
		}
	case tcell.KeyRight:
		if lc.SelectedContainerIndex < len(lc.Containers)-1 {
			lc.SelectedContainerIndex++
		}
	case tcell.KeyEnter:
		lc.CurrentStep++
		return true
	}
	return false
}

func (lc *LaunchableCreator) handleURLInput(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEnter:
		if isValidURL(lc.URLInput) {
			lc.CurrentStep++
			return true
		}
	case tcell.KeyRune:
		lc.URLInput += string(ev.Rune())
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(lc.URLInput) > 0 {
			lc.URLInput = lc.URLInput[:len(lc.URLInput)-1]
		}
	}
	return false
}

func (lc *LaunchableCreator) handlePortInput(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEnter:
		if !lc.ExposePort || (lc.ExposePort && lc.ExposedPort != "") {
			lc.CurrentStep++
			return true
		}
	case tcell.KeyRune:
		if ev.Rune() == 'y' || ev.Rune() == 'Y' {
			lc.ExposePort = true
		} else if ev.Rune() == 'n' || ev.Rune() == 'N' {
			lc.ExposePort = false
			lc.ExposedPort = ""
		} else if lc.ExposePort {
			lc.ExposedPort += string(ev.Rune())
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if lc.ExposePort && len(lc.ExposedPort) > 0 {
			lc.ExposedPort = lc.ExposedPort[:len(lc.ExposedPort)-1]
		}
	}
	return false
}

func (lc *LaunchableCreator) handleReview(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyEnter {
		lc.CurrentStep++
		return true
	}
	return false
}

func isValidURL(urlStr string) bool {
	u, err := url.Parse(urlStr)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}

	if strings.HasSuffix(urlStr, ".ipynb") {
		return true
	}

	if strings.Contains(urlStr, "github.com") || strings.Contains(urlStr, "gitlab.com") || strings.Contains(urlStr, "bitbucket.org") {
		return true
	}

	return false
}

type TestStore interface {
	completions.CompletionStore
	ResetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetAllWorkspaces(options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetWorkspace(id string) (*entity.Workspace, error)
	GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error)
	CopyBin(targetBin string) error
	GetSetupScriptContentsByURL(url string) (string, error)
	UpdateUser(userID string, updatedUser *entity.UpdateUser) (*entity.User, error)
}

type ServiceMeshStore interface {
	autostartconf.AutoStartStore
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
}

type step struct {
	name      string
	draw      func(*tcell.Screen, int)
	handleKey func(*tcell.EventKey) bool
}

func NewCmdTest(t *terminal.Terminal, testStore TestStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"devonly": ""},
		Use:                   "test",
		DisableFlagsInUseLine: true,
		Short:                 "[internal] GPU Instance Picker",
		Long:                  startLong,
		Example:               startExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			gpu, err := RunGPUPicker()
			if err != nil {
				return fmt.Errorf("failed to run GPU picker: %v", err)
			}

			if gpu != nil {
				t.Vprintf("🎉 Selected GPU: %s\n", t.Green(gpu.name))
				t.Vprintf("Memory: %s\n", gpu.memory)
				t.Vprintf("Performance: %s\n", gpu.performance)
				t.Vprintf("Price: %s\n", gpu.price)
			}

			return nil
		},
	}

	return cmd
}

func runCreateLaunchable(cmd *cobra.Command, args []string) error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("failed to create screen: %v", err)
	}
	if err := screen.Init(); err != nil {
		return fmt.Errorf("failed to initialize screen: %v", err)
	}
	defer screen.Fini()

	creator := NewLaunchableCreator(screen)

	for {
		creator.Render()

		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			screen.Sync()
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
				return nil
			}
			if creator.HandleInput(ev) {
				if creator.CurrentStep >= 5 {
					// Display the final message and exit
					layout := NewLayout(screen)
					layout.AddElement("🤙 https://brev.dev", tcell.StyleDefault.Foreground(colorCyanBase).Bold(true), 1, 1)
					layout.Render()
					time.Sleep(2 * time.Second) // Display for 2 seconds before exiting
					return nil
				}
			}
		}
	}
}
