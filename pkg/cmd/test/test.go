package test

import (
	"fmt"

	"github.com/brevdev/brev-cli/pkg/autostartconf"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	// "github.com/brevdev/brev-cli/pkg/util"
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
		// Args:                  cmderrors.TransformToValidationError(cobra.MinimumNArgs(1)),
		RunE: runChipSelector,
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
			x, y := (i%2)*15, (i/2)*6
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
			case tcell.KeyUp:
				if selectedIndex > 1 {
					selectedIndex -= 2
				}
			case tcell.KeyDown:
				if selectedIndex < len(chips)-2 {
					selectedIndex += 2
				}
			case tcell.KeyLeft:
				if selectedIndex%2 == 1 {
					selectedIndex--
				}
			case tcell.KeyRight:
				if selectedIndex%2 == 0 && selectedIndex < len(chips)-1 {
					selectedIndex++
				}
			}
		}
	}
}

func drawChip(s tcell.Screen, x, y int, name string, selected bool) {
	width, height := 10, 5
	style := tcell.StyleDefault

	
	if selected {
		style = style.Foreground(tcell.NewRGBColor(0, 255, 255)) // Cyan color in RGB
	}


	// Draw top and bottom borders
	for i := 0; i < width; i++ {
		s.SetContent(x+i, y, '-', nil, style)
		s.SetContent(x+i, y+height-1, '-', nil, style)
	}

	// Draw side borders and fill
	for i := 1; i < height-1; i++ {
		s.SetContent(x, y+i, '|', nil, style)
		s.SetContent(x+width-1, y+i, '|', nil, style)
		for j := 1; j < width-1; j++ {
			s.SetContent(x+j, y+i, ' ', nil, style)
		}
	}

	// Draw chip name
	for i, r := range name {
		s.SetContent(x+1+(width-2-len(name))/2+i, y+height/2, r, nil, style)
	}
}