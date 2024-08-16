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
	urlInput := ""
	exposedPort := ""
	exposePort := false

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
			name: "Enter URL",
			draw: func(s *tcell.Screen, t int) {
				(*s).Clear()
				drawTitle(*s)
				drawStepIndicator(*s, currentStep)
				drawChipSelectionSummary(*s, chips[selectedChipIndex])
				drawContainerSelectionSummary(*s, containers[selectedContainerIndex])
				drawURLInput(*s, urlInput)
			},
			handleKey: func(ev *tcell.EventKey) bool {
				switch ev.Key() {
				case tcell.KeyEnter:
					if isValidURL(urlInput) {
						currentStep++
						return true
					}
				case tcell.KeyRune:
					urlInput += string(ev.Rune())
				case tcell.KeyBackspace, tcell.KeyBackspace2:
					if len(urlInput) > 0 {
						urlInput = urlInput[:len(urlInput)-1]
					}
				}
				return false
			},
		},
		{
			name: "Expose Port",
			draw: func(s *tcell.Screen, t int) {
				(*s).Clear()
				drawTitle(*s)
				drawStepIndicator(*s, currentStep)
				drawChipSelectionSummary(*s, chips[selectedChipIndex])
				drawContainerSelectionSummary(*s, containers[selectedContainerIndex])
				drawURLSummary(*s, urlInput)
				drawExposePortOption(*s, exposePort, exposedPort)
			},
			handleKey: func(ev *tcell.EventKey) bool {
				switch ev.Key() {
				case tcell.KeyEnter:
					if !exposePort || (exposePort && exposedPort != "") {
						currentStep++
						return true
					}
				case tcell.KeyRune:
					if ev.Rune() == 'y' || ev.Rune() == 'Y' {
						exposePort = true
					} else if ev.Rune() == 'n' || ev.Rune() == 'N' {
						exposePort = false
						exposedPort = ""
					} else if exposePort {
						exposedPort += string(ev.Rune())
					}
				case tcell.KeyBackspace, tcell.KeyBackspace2:
					if exposePort && len(exposedPort) > 0 {
						exposedPort = exposedPort[:len(exposedPort)-1]
					}
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
                drawURLSummary(*s, urlInput)
                drawPortSummary(*s, exposePort, exposedPort)
                drawSeparatorLine(*s)
                drawConfirmationPrompt(*s)
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

func drawURLInput(s tcell.Screen, urlInput string) {
	prompt := "Enter URL (.ipynb or git repo): "
	style := tcell.StyleDefault.Foreground(tcell.ColorWhite)
	for i, r := range prompt {
		s.SetContent(1+i, 10, r, nil, style)
	}
	for i, r := range urlInput {
		s.SetContent(1+len(prompt)+i, 10, r, nil, style)
	}
	s.SetContent(1+len(prompt)+len(urlInput), 10, '_', nil, style)
}

func drawExposePortOption(s tcell.Screen, exposePort bool, exposedPort string) {
	prompt := "Do you want to expose a port? (y/n): "
	style := tcell.StyleDefault.Foreground(tcell.ColorWhite)
	for i, r := range prompt {
		s.SetContent(1+i, 11, r, nil, style)
	}
	if exposePort {
		s.SetContent(1+len(prompt), 11, 'Y', nil, style)
		portPrompt := "Enter port number: "
		for i, r := range portPrompt {
			s.SetContent(1+i, 12, r, nil, style)
		}
		for i, r := range exposedPort {
			s.SetContent(1+len(portPrompt)+i, 12, r, nil, style)
		}
		s.SetContent(1+len(portPrompt)+len(exposedPort), 12, '_', nil, style)
	} else {
		s.SetContent(1+len(prompt), 11, 'N', nil, style)
	}
}

func isValidURL(urlStr string) bool {
	u, err := url.Parse(urlStr)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	
	// Check if it's a .ipynb file
	if strings.HasSuffix(urlStr, ".ipynb") {
		return true
	}
	
	// Check if it's a git repo (this is a simple check, you might want to expand it)
	if strings.Contains(urlStr, "github.com") || strings.Contains(urlStr, "gitlab.com") || strings.Contains(urlStr, "bitbucket.org") {
		return true
	}
	
	return false
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
	steps := []string{"Select Chip", "Select Container", "Enter URL", "Expose Port"}
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
}

func drawURLSummary(s tcell.Screen, url string) {
    summary := fmt.Sprintf("🔗 URL: %s", url)
    style := tcell.StyleDefault.Foreground(colorCyanBase).Bold(true)
    for i, r := range summary {
        s.SetContent(1+i, 9, r, nil, style)
    }
}

func drawPortSummary(s tcell.Screen, exposePort bool, exposedPort string) {
    var summary string
    if exposePort {
        summary = fmt.Sprintf("🔌 Exposed Port: %s", exposedPort)
    } else {
        summary = "🔌 No Port Exposed"
    }
    style := tcell.StyleDefault.Foreground(colorCyanBase).Bold(true)
    for i, r := range summary {
        s.SetContent(1+i, 11, r, nil, style)
    }
}

func drawSeparatorLine(s tcell.Screen) {
    style := tcell.StyleDefault.Foreground(colorCyanBase)
    for i := 0; i < 70; i++ {
        s.SetContent(1+i, 13, '-', nil, style)
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



func drawConfirmationPrompt(s tcell.Screen) {
    prompt := "Press Enter to confirm and create the launchable"
    style := tcell.StyleDefault.Foreground(tcell.ColorWhite)
    for i, r := range prompt {
        s.SetContent(1+i, 15, r, nil, style)
    }
}