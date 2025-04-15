package create

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/featureflag"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/charmbracelet/lipgloss"
)

var (
	createLong    = "Create a new Brev machine"
	createExample = `
  brev create <name>
	`
	// instanceTypes = []string{"p4d.24xlarge", "p3.2xlarge", "p3.8xlarge", "p3.16xlarge", "p3dn.24xlarge", "p2.xlarge", "p2.8xlarge", "p2.16xlarge", "g5.xlarge", "g5.2xlarge", "g5.4xlarge", "g5.8xlarge", "g5.16xlarge", "g5.12xlarge", "g5.24xlarge", "g5.48xlarge", "g5g.xlarge", "g5g.2xlarge", "g5g.4xlarge", "g5g.8xlarge", "g5g.16xlarge", "g5g.metal", "g4dn.xlarge", "g4dn.2xlarge", "g4dn.4xlarge", "g4dn.8xlarge", "g4dn.16xlarge", "g4dn.12xlarge", "g4dn.metal", "g4ad.xlarge", "g4ad.2xlarge", "g4ad.4xlarge", "g4ad.8xlarge", "g4ad.16xlarge", "g3s.xlarge", "g3.4xlarge", "g3.8xlarge", "g3.16xlarge"}
)

var (
	
	NVIDIA_LOGO_1 = `
	███╗   ██╗██╗   ██╗██╗██████╗ ██╗ █████╗ 
	████╗  ██║██║   ██║██║██╔══██╗██║██╔══██╗
	██╔██╗ ██║██║   ██║██║██║  ██║██║███████║
	██║╚██╗██║╚██╗ ██╔╝██║██║  ██║██║██╔══██║
	██║ ╚████║ ╚████╔╝ ██║██████╔╝██║██║  ██║
	╚═╝  ╚═══╝  ╚═══╝  ╚═╝╚═════╝ ╚═╝╚═╝  ╚═╝`

)


type CreateStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error)
	GetInstanceTypes() (*store.InstanceTypeResponse, error)
}

// buildGPUConfigFromInstanceType creates a GPU config from a full instance type
func buildGPUConfigFromInstanceType(instanceType *store.GPUInstanceType) *store.GPUConfig {
	return &store.GPUConfig{
		Type:            instanceType.Type,
		Provider:        instanceType.Provider,
		Count:          1,
		Price:          instanceType.BasePrice,
		WorkspaceGroups: instanceType.WorkspaceGroups,
	}
}

func NewCmdCreate(t *terminal.Terminal, createStore CreateStore) *cobra.Command {
	var detached bool
	var gpu *store.GPUConfig
	var cpu string
	var listTypes bool
	var instanceType string

	cmd := &cobra.Command{
		Annotations:           map[string]string{"workspace": ""},
		Use:                   "create",
		DisableFlagsInUseLine: true,
		Short:                 "Create a new instance ",
		Long:                  createLong,
		Example:               createExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			if listTypes {
				return displayInstanceTypes(t, createStore)
			}

			name := ""
			if len(args) > 0 {
				name = args[0]
			} else {
				var err error
				name, err = RunNamePicker()
				if err != nil {
					return breverrors.WrapAndTrace(err)
				}
			}

			// Get instance types first - we need this for both interactive and non-interactive modes
			types, err := createStore.GetInstanceTypes()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			// If instance type is specified, find it in the available types
			if instanceType != "" {
				var selectedInstanceType *store.GPUInstanceType
				// Find the exact instance type
				for _, it := range types.AllInstanceTypes {
					if it.Type == instanceType {
						selectedInstanceType = &it
						break
					}
				}
				if selectedInstanceType == nil {
					return breverrors.NewValidationError(fmt.Sprintf("Invalid instance type: %s. Use --list-types to see available options", instanceType))
				}
				
				// Build the GPU config the same way the picker would
				gpu = buildGPUConfigFromInstanceType(selectedInstanceType)
			} else if !cmd.Flags().Changed("cpu") {
				// Default to interactive GPU picker if no instance type or CPU specified
				selectedGPU, err := RunGPUPicker(types)
				if err != nil {
					return breverrors.WrapAndTrace(err)
				}
				gpu = selectedGPU
			}

			// Validate flags if explicitly set
			if cmd.Flags().Changed("cpu") && cpu == "" {
				return breverrors.NewValidationError("CPU instance type cannot be empty when --cpu flag is used")
			}

			err = runCreateWorkspace(t, CreateOptions{
				Name:           name,
				WorkspaceClass: cpu,
				Detached:       detached,
				Config:         gpu,
			}, createStore)
			if err != nil {
				if strings.Contains(err.Error(), "duplicate instance with name") {
					t.Vprint(t.Yellow("try running:"))
					t.Vprint(t.Yellow("\tbrev start --name [different name] [repo] # or"))
					t.Vprint(t.Yellow("\tbrev delete [name]"))
				}
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&detached, "detached", "d", false, "run the command in the background instead of blocking the shell")
	cmd.Flags().StringVarP(&cpu, "cpu", "c", "", "CPU instance type. Defaults to 2x8 [2x8, 4x16, 8x32, 16x32]. See docs.brev.dev/cpu for details")
	cmd.Flags().StringVarP(&instanceType, "instance-type", "t", "", "GPU instance type. See https://brev.dev/docs/reference/gpu for details")
	cmd.Flags().BoolVarP(&listTypes, "list-types", "l", false, "List available instance types")
	return cmd
}

type CreateOptions struct {
	Name           string
	WorkspaceClass string
	Detached       bool
	Config         *store.GPUConfig
}

func runCreateWorkspace(t *terminal.Terminal, options CreateOptions, createStore CreateStore) error {
	user, err := createStore.GetCurrentUser()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = createEmptyWorkspace(user, t, options, createStore)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func createEmptyWorkspace(user *entity.User, t *terminal.Terminal, options CreateOptions, createStore CreateStore) error {
	// ensure name
	if len(options.Name) == 0 {
		return breverrors.NewValidationError("A name field is required to create a workspace!")
	}

	// ensure org
	var orgID string
	activeorg, err := createStore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if activeorg == nil {
		return breverrors.NewValidationError("no org exist")
	}
	orgID = activeorg.ID

	clusterID := config.GlobalConfig.GetDefaultClusterID()
	cwOptions := store.NewCreateWorkspacesOptions(clusterID, options.Name, options.Config)

	if options.WorkspaceClass != "" {
		cwOptions.WithClassID(options.WorkspaceClass)
	}

	cwOptions = resolveWorkspaceUserOptions(cwOptions, user)

	if options.Config != nil {
		cwOptions.WithInstanceType(options.Config.Type)
	}

	// Format the instance specs in a box
	var specBox string
	var gpu store.SupportedGPU
	if options.Config != nil {
		// Get instance type info
		instanceTypes, err := createStore.GetInstanceTypes()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		// Find the matching instance type
		var instanceType store.GPUInstanceType
		for _, it := range instanceTypes.AllInstanceTypes {
			if it.Type == options.Config.Type {
				instanceType = it
				if len(it.SupportedGPUs) > 0 {
					gpu = it.SupportedGPUs[0]
				}
				break
			}
		}

		leftContent := fmt.Sprintf("%dx %s • %s VRAM • %s RAM x %d CPUs",
			options.Config.Count,
			gpu.Name,
			gpu.Memory,
			instanceType.Memory,
			instanceType.VCPU,
		)
		rightContent := fmt.Sprintf("%s • %s",
			strings.ToUpper(options.Config.Provider),
			formatPrice(options.Config.Price),
		)
		padding := 70 - lipgloss.Width(leftContent) - lipgloss.Width(rightContent) - 2
		content := fmt.Sprintf("%s%s%s",
			leftContent,
			strings.Repeat(" ", padding),
			rightContent,
		)
		specBox = configBoxStyle.Render(content)
		
		// Set the instance type here after we've found it
		cwOptions.WithInstanceType(instanceType.Type)
	}

	t.Vprintf("Creating instance %s in org %s\n", t.Green(cwOptions.Name), t.Green(orgID))
	if specBox != "" {
		t.Vprint(specBox)
		t.Vprintf("Instance: %s\n", t.Green(options.Config.Type))
		t.Vprint("")
	} else if cwOptions.InstanceType != "" {
		t.Vprintf("Instance type: %s\n", t.Green(cwOptions.InstanceType))
		t.Vprint("")
	}

	s := t.NewSpinner()
	s.Suffix = " Creating your instance. Hang tight 🤙"
	s.Start()
	w, err := createStore.CreateWorkspace(orgID, cwOptions)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	s.Stop()

	if options.Detached {
		return nil
	} else {
		err = pollUntil(t, w.ID, entity.Running, createStore, true)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}

		fmt.Print("\n")
		t.Vprint(t.Green("Your instance is ready!\n"))
		displayConnectBreadCrumb(t, w)

		return nil
	}
}

func resolveWorkspaceUserOptions(options *store.CreateWorkspacesOptions, user *entity.User) *store.CreateWorkspacesOptions {
	if options.WorkspaceTemplateID == "" {
		if featureflag.IsAdmin(user.GlobalUserType) {
			options.WorkspaceTemplateID = store.DevWorkspaceTemplateID
		} else {
			options.WorkspaceTemplateID = store.UserWorkspaceTemplateID
		}
	}
	if options.WorkspaceClassID == "" {
		if featureflag.IsAdmin(user.GlobalUserType) {
			options.WorkspaceClassID = store.DevWorkspaceClassID
		} else {
			options.WorkspaceClassID = store.UserWorkspaceClassID
		}
	}
	return options
}

func displayConnectBreadCrumb(t *terminal.Terminal, workspace *entity.Workspace) {
	t.Vprintf(t.Green("Connect to the instance:\n"))
	t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev open %s\t# brev open <NAME> -> open instance in VS Code\n", workspace.Name)))
	t.Vprintf(t.Yellow(fmt.Sprintf("\tbrev shell %s\t# brev shell <NAME> -> ssh into instance (shortcut)\n", workspace.Name)))
	// t.Vprintf(t.Yellow(fmt.Sprintf("\tssh %s\t# ssh <SSH-NAME> -> ssh directly to dev environment\n", workspace.GetLocalIdentifier())))
}

func pollUntil(t *terminal.Terminal, wsid string, state string, createStore CreateStore, canSafelyExit bool) error {
	s := t.NewSpinner()
	isReady := false
	if canSafelyExit {
		t.Vprintf("You can safely ctrl+c to exit\n")
	}
	s.Suffix = " hang tight 🤙"
	s.Start()
	for !isReady {
		time.Sleep(5 * time.Second)
		ws, err := createStore.GetWorkspace(wsid)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		s.Suffix = "  instance is " + strings.ToLower(ws.Status)
		if ws.Status == state {
			s.Suffix = "Instance is ready!"
			s.Stop()
			isReady = true
		}
	}
	return nil
}

func displayInstanceTypes(t *terminal.Terminal, createStore CreateStore) error {
	t.Vprint("Fetching available instance types...")
	types, err := createStore.GetInstanceTypes()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Convert to JSON and print
	jsonBytes, err := json.MarshalIndent(types, "", "  ")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	t.Vprintf("\nAvailable instance types:\n%s\n", string(jsonBytes))
	return nil
}
