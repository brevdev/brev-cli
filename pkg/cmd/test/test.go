package test

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/gdamore/tcell/v2"

	"github.com/spf13/cobra"
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

func NewCmdTest(_ *terminal.Terminal, _ TestStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"devonly": ""},
		Use:                   "test",
		DisableFlagsInUseLine: true,
		Short:                 "[internal] Test random stuff.",
		Long:                  startLong,
		Example:               startExample,
		RunE:                  runChipSelector,
	}

	return cmd
}

func runChipSelector(cmd *cobra.Command, args []string) error {
	screen, err := tcell.NewScreen()
	if err != nil {
		return fmt.Errorf("failed to create screen: %v", err)
	}
	if err := screen.Init(); err != nil {
		return fmt.Errorf("failed to initialize screen: %v", err)
	}
	defer screen.Fini()

	chips := []string{"A100", "H100", "L40S"}
	selectedIndex := 0

	drawChips := func() {
		screen.Clear()
		for i, chip := range chips {
			x := i * 16 // Increased spacing between chips
			y := 3      // Fixed y position for single row
			drawChip(screen, x, y, chip, i == selectedIndex)
		}
		screen.Show()
	}

	for {
		drawChips()

		switch ev := screen.PollEvent().(type) {
		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyEscape, tcell.KeyCtrlC:
				return nil
			case tcell.KeyEnter:
				return fmt.Errorf("selected: %s", chips[selectedIndex])
			case tcell.KeyLeft:
				if selectedIndex > 0 {
					selectedIndex--
				}
			case tcell.KeyRight:
				if selectedIndex < len(chips)-1 {
					selectedIndex++
				}
			}
		}
	}
}

func drawChip(s tcell.Screen, x, y int, name string, selected bool) {
	width, height := 11, 5 // Increased width by 1 to accommodate the space
	style := tcell.StyleDefault
	
	if selected {
		style = style.Foreground(tcell.NewRGBColor(0, 255, 255)) // Brighter cyan color
		// Draw ASCII arrow to the left of the selected chip
		arrowStyle := tcell.StyleDefault.Foreground(tcell.NewRGBColor(255, 255, 0)) // Yellow arrow
		s.SetContent(x, y+height/2, '>', nil, arrowStyle)
	}

	// Draw space before the chip
	s.SetContent(x+1, y+height/2, ' ', nil, style)

	// Draw top and bottom borders
	for i := 2; i < width; i++ {
		s.SetContent(x+i, y, '-', nil, style)
		s.SetContent(x+i, y+height-1, '-', nil, style)
	}

	// Draw side borders and fill
	for i := 1; i < height-1; i++ {
		s.SetContent(x+2, y+i, '|', nil, style)
		s.SetContent(x+width-1, y+i, '|', nil, style)
		for j := 3; j < width-1; j++ {
			s.SetContent(x+j, y+i, ' ', nil, style)
		}
	}

	// Draw chip name
	for i, r := range name {
		s.SetContent(x+3+(width-4-len(name))/2+i, y+height/2, r, nil, style)
	}
}