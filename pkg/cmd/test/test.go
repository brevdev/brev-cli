package test

import (
	"fmt"
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
    colorCyanBase = tcell.NewRGBColor(0, 255, 255)
)

var (
	startLong    = "[internal] test"
	startExample = "[internal] test"
)

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
	name     string
	draw     func(*tcell.Screen, int)
	handleKey func(*tcell.EventKey) bool
}

func NewCmdTest(_ *terminal.Terminal, _ TestStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"devonly": ""},
		Use:                   "test",
		DisableFlagsInUseLine: true,
		Short:                 "[internal] Test random stuff.",
		Long:                  startLong,
		Example:               startExample,
		RunE:                  runCreateLaunchable,
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

	chips := []string{"A100", "H100", "L40S"}
	containers := []string{"Container 1", "Container 2", "Container 3", "Container 4"}
	selectedChipIndex := 0
	selectedContainerIndex := 0
	currentStep := 0
	// Enable mouse tracking
	screen.EnableMouse()

	steps := []step{
        {
            name: "Select Chip",
            draw: func(s *tcell.Screen, t int) {
                (*s).Clear()
                drawTitle(*s)
                drawStepIndicator(*s, currentStep)
                drawChips(*s, chips, selectedChipIndex, t)
            },
            handleKey: func(ev *tcell.EventKey) bool {
                switch ev.Key() {
                case tcell.KeyLeft:
                    if selectedChipIndex > 0 {
                        selectedChipIndex--
                    }
                case tcell.KeyRight:
                    if selectedChipIndex < len(chips)-1 {
                        selectedChipIndex++
                    }
                case tcell.KeyEnter:
                    currentStep++
                    return true
                }
                return false
            },
        },
        {
            name: "Select Container",
            draw: func(s *tcell.Screen, t int) {
                (*s).Clear()
                drawTitle(*s)
                drawStepIndicator(*s, currentStep)
                drawChipSelectionSummary(*s, chips[selectedChipIndex])
                drawContainers(*s, containers, selectedContainerIndex)
            },
            handleKey: func(ev *tcell.EventKey) bool {
                switch ev.Key() {
                case tcell.KeyLeft:
                    if selectedContainerIndex > 0 {
                        selectedContainerIndex--
                    }
                case tcell.KeyRight:
                    if selectedContainerIndex < len(containers)-1 {
                        selectedContainerIndex++
                    }
                case tcell.KeyEnter:
                    currentStep++
                    return true
                }
                return false
            },
        },
		{
            name: "Review Selections",
            draw: func(s *tcell.Screen, t int) {
                (*s).Clear()
                drawTitle(*s)
                drawStepIndicator(*s, currentStep)
                drawChipSelectionSummary(*s, chips[selectedChipIndex])
                drawContainerSelectionSummary(*s, containers[selectedContainerIndex])
                // You can add more UI elements for the next steps here
            },
            handleKey: func(ev *tcell.EventKey) bool {
                switch ev.Key() {
                case tcell.KeyEnter:
                    currentStep++
                    return true
                }
                return false
            },
        },
		{name: "Step 3", draw: func(s *tcell.Screen, t int) {}, handleKey: func(ev *tcell.EventKey) bool { return true }},
		{name: "Step 4", draw: func(s *tcell.Screen, t int) {}, handleKey: func(ev *tcell.EventKey) bool { return true }},
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
    
	// Initial draw
    drawCurrentStep(screen, steps, currentStep, selectedChipIndex, selectedContainerIndex)

    for {
        ev := screen.PollEvent()
        switch ev := ev.(type) {
        case *tcell.EventResize:
            screen.Sync()
        case *tcell.EventKey:
            if ev.Key() == tcell.KeyEscape || ev.Key() == tcell.KeyCtrlC {
                return nil
            }
            if steps[currentStep].handleKey(ev) {
                if currentStep >= len(steps) {
                    return fmt.Errorf("launchable created")
                }
            }
        }

        // Redraw after each event
        drawCurrentStep(screen, steps, currentStep, selectedChipIndex, selectedContainerIndex)
    }
}

func drawCurrentStep(screen tcell.Screen, steps []step, currentStep, selectedChipIndex, selectedContainerIndex int) {
    t := int(time.Now().UnixNano() / 1e7 % 20)
    steps[currentStep].draw(&screen, t)
    screen.Show()
}

func drawTitle(s tcell.Screen) {
	title := "Create Launchable"
	style := tcell.StyleDefault.Foreground(tcell.ColorWhite).Bold(true)
	for i, r := range title {
		s.SetContent(i+1, 1, r, nil, style)
	}
}

func drawStepIndicator(s tcell.Screen, currentStep int) {
	steps := []string{"Select Chip", "Select Container", "Step 3", "Step 4"}
	for i, step := range steps {
		style := tcell.StyleDefault.Foreground(tcell.ColorGray)
		if i == currentStep {
			style = style.Foreground(tcell.ColorWhite).Bold(true)
		}
		for j, r := range step {
			s.SetContent(i*20+j+1, 2, r, nil, style)
		}
	}
}

func drawChips(s tcell.Screen, chips []string, selectedIndex, t int) {
	for i, chip := range chips {
		x := i*16 + 1
		y := 5
		drawChip(s, x, y, chip, i == selectedIndex, t)
	}
}

func drawChip(s tcell.Screen, x, y int, name string, selected bool, t int) {
	width, height := 11, 5
	style := tcell.StyleDefault

	if selected {
		// Pulsating effect
		// colorValue := 128 + int32(127*float64(t)/20.0)
		// style = style.Foreground(tcell.NewRGBColor(0, colorValue, colorValue))

		pulseFactor := float64(t) / 20.0
		r := uint8(0)
		g := uint8(128 + int(127*pulseFactor))
		b := uint8(128 + int(127*pulseFactor))
		style = style.Foreground(tcell.NewRGBColor(int32(r), int32(g), int32(b)))

		arrowStyle := tcell.StyleDefault.Foreground(tcell.NewRGBColor(255, 255, 0))
		s.SetContent(x-1, y+height/2, '>', nil, arrowStyle)
	}

	// Draw chip outline and content
	for i := 0; i < width; i++ {
		s.SetContent(x+i, y, '-', nil, style)
		s.SetContent(x+i, y+height-1, '-', nil, style)
	}
	for i := 1; i < height-1; i++ {
		s.SetContent(x, y+i, '|', nil, style)
		s.SetContent(x+width-1, y+i, '|', nil, style)
	}
	for i, r := range name {
		s.SetContent(x+1+(width-2-len(name))/2+i, y+height/2, r, nil, style)
	}
}

func drawChipSelectionSummary(s tcell.Screen, chipName string) {
    summary := fmt.Sprintf("🤙 Chip: %s", chipName)
    style := tcell.StyleDefault.Foreground(colorCyanBase).Bold(true)
    for i, r := range summary {
        s.SetContent(1+i, 5, r, nil, style)
    }
}

func drawContainerSelectionSummary(s tcell.Screen, containerName string) {
    summary := fmt.Sprintf("📦 Container: %s", containerName)
    style := tcell.StyleDefault.Foreground(colorCyanBase).Bold(true)
    for i, r := range summary {
        s.SetContent(1+i, 7, r, nil, style)
    }
    // Add a separator line
    for i := 0; i < 70; i++ {
        s.SetContent(1+i, 8, '-', nil, style)
    }
}

func drawContainers(s tcell.Screen, containers []string, selectedIndex int) {
    for i, container := range containers {
        x := (i%2)*35 + 1
        y := (i/2)*4 + 8  // Adjusted vertical spacing
        drawContainer(s, x, y, container, i == selectedIndex)
    }
}


func drawContainer(s tcell.Screen, x, y int, name string, selected bool) {
    width, height := 25, 3  // Adjusted size
    style := tcell.StyleDefault
    if selected {
        style = style.Foreground(colorCyanBase)
        arrowStyle := tcell.StyleDefault.Foreground(colorYellow)
        s.SetContent(x-1, y+height/2, '>', nil, arrowStyle)
    }

    // Draw outline
    for i := 0; i < width; i++ {
        s.SetContent(x+i, y, '-', nil, style)
        s.SetContent(x+i, y+height-1, '-', nil, style)
    }
    for i := 1; i < height-1; i++ {
        s.SetContent(x, y+i, '|', nil, style)
        s.SetContent(x+width-1, y+i, '|', nil, style)
    }

    // Draw container name
    for i, r := range name {
        s.SetContent(x+1+(width-2-len(name))/2+i, y+height/2, r, nil, style)
    }
}
