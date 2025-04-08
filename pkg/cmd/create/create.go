package create

import (
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
)

var (
	createLong    = "Create a new Brev machine"
	createExample = `
  brev create <name>
	`
	// instanceTypes = []string{"p4d.24xlarge", "p3.2xlarge", "p3.8xlarge", "p3.16xlarge", "p3dn.24xlarge", "p2.xlarge", "p2.8xlarge", "p2.16xlarge", "g5.xlarge", "g5.2xlarge", "g5.4xlarge", "g5.8xlarge", "g5.16xlarge", "g5.12xlarge", "g5.24xlarge", "g5.48xlarge", "g5g.xlarge", "g5g.2xlarge", "g5g.4xlarge", "g5g.8xlarge", "g5g.16xlarge", "g5g.metal", "g4dn.xlarge", "g4dn.2xlarge", "g4dn.4xlarge", "g4dn.8xlarge", "g4dn.16xlarge", "g4dn.12xlarge", "g4dn.metal", "g4ad.xlarge", "g4ad.2xlarge", "g4ad.4xlarge", "g4ad.8xlarge", "g4ad.16xlarge", "g3s.xlarge", "g3.4xlarge", "g3.8xlarge", "g3.16xlarge"}
)

type CreateStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	GetActiveOrganizationOrDefault() (*entity.Organization, error)
	GetCurrentUser() (*entity.User, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	CreateWorkspace(organizationID string, options *store.CreateWorkspacesOptions) (*entity.Workspace, error)
	GetInstanceTypes() (*store.InstanceTypeResponse, error)
}

func NewCmdCreate(t *terminal.Terminal, createStore CreateStore) *cobra.Command {
	var detached bool
	var gpu string
	var cpu string
	var listTypes bool

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

			// Validate flags if explicitly set
			if cmd.Flags().Changed("gpu") && gpu == "" {
				return breverrors.NewValidationError("GPU instance type cannot be empty when --gpu flag is used")
			}
			if cmd.Flags().Changed("cpu") && cpu == "" {
				return breverrors.NewValidationError("CPU instance type cannot be empty when --cpu flag is used")
			}

			// Set default T4 instance if no flags are provided
			if !cmd.Flags().Changed("gpu") && !cmd.Flags().Changed("cpu") {
				gpu = "n1-highmem-4:nvidia-tesla-t4:1"
			}

			err := runCreateWorkspace(t, CreateOptions{
				Name:           name,
				WorkspaceClass: cpu,
				Detached:       detached,
				InstanceType:   gpu,
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
	cmd.Flags().StringVarP(&gpu, "gpu", "g", "", "GPU instance type. See https://brev.dev/docs/reference/gpu for details")
	cmd.Flags().BoolVarP(&listTypes, "list-types", "l", false, "List available instance types")
	return cmd
}

type CreateOptions struct {
	Name           string
	WorkspaceClass string
	Detached       bool
	InstanceType   string
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
	cwOptions := store.NewCreateWorkspacesOptions(clusterID, options.Name)

	if options.WorkspaceClass != "" {
		cwOptions.WithClassID(options.WorkspaceClass)
	}

	cwOptions = resolveWorkspaceUserOptions(cwOptions, user)

	if options.InstanceType != "" {
		cwOptions.WithInstanceType(options.InstanceType)
	}

	t.Vprintf("Creating instane %s in org %s\n", t.Green(cwOptions.Name), t.Green(orgID))
	t.Vprintf("\tname %s\n", t.Green(cwOptions.Name))
	if options.InstanceType != "" {
		t.Vprintf("\tGPU instance %s\n", t.Green(options.InstanceType))
	} else {
		t.Vprintf("\tCPU instance %s\n", t.Green(cwOptions.WorkspaceClassID))
	}
	t.Vprintf("\tCloud %s\n\n", t.Green(cwOptions.WorkspaceGroupID))

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
	t.Vprint("\nAvailable CPU instance types:")
	t.Vprint("  2x8  - 2 CPU cores, 8GB RAM")
	t.Vprint("  4x16 - 4 CPU cores, 16GB RAM")
	t.Vprint("  8x32 - 8 CPU cores, 32GB RAM")
	t.Vprint("  16x32 - 16 CPU cores, 32GB RAM")

	t.Vprint("\nAvailable GPU instance types:")
	t.Vprint("  n1-highmem-4:nvidia-tesla-t4:1 - NVIDIA T4 (x1)")
	return nil
}
