package copy

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/refresh"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/brevdev/brev-cli/pkg/writeconnectionevent"
	"github.com/briandowns/spinner"

	"github.com/spf13/cobra"
)

var (
	copyLong    = "Copy files between your local machine and remote instance"
	copyExample = "brev copy instance_name:/path/to/remote/file /path/to/local/file\nbrev copy /path/to/local/file instance_name:/path/to/remote/file"
)

type CopyStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	refresh.RefreshStore
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	StartWorkspace(workspaceID string) (*entity.Workspace, error)
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
	GetCurrentUserKeys() (*entity.UserKeys, error)
}

func NewCmdCopy(t *terminal.Terminal, store CopyStore, noLoginStartStore CopyStore) *cobra.Command {
	var host bool
	cmd := &cobra.Command{
		Annotations:           map[string]string{"ssh": ""},
		Use:                   "copy",
		Aliases:               []string{"cp", "scp"},
		DisableFlagsInUseLine: true,
		Short:                 "copy files between local and remote instance",
		Long:                  copyLong,
		Example:               copyExample,
		Args:                  cmderrors.TransformToValidationError(cobra.ExactArgs(2)),
		ValidArgsFunction:     completions.GetAllWorkspaceNameCompletionHandler(noLoginStartStore, t),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := runCopyCommand(t, store, args[0], args[1], host)
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&host, "host", "", false, "copy to/from the host machine instead of the container")

	return cmd
}

func runCopyCommand(t *terminal.Terminal, cstore CopyStore, source, dest string, host bool) error {
	workspaceNameOrID, remotePath, localPath, isUpload, err := parseCopyArguments(source, dest)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	workspace, err := prepareWorkspace(t, cstore, workspaceNameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	sshName, err := setupSSHConnection(t, cstore, workspace, host)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	_ = writeconnectionevent.WriteWCEOnEnv(cstore, workspace.DNS)

	err = runSCP(sshName, localPath, remotePath, isUpload)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

func parseCopyArguments(source, dest string) (workspaceNameOrID, remotePath, localPath string, isUpload bool, err error) {
	sourceWorkspace, sourcePath, err := parseWorkspacePath(source)
	if err != nil {
		return "", "", "", false, err
	}

	destWorkspace, destPath, err := parseWorkspacePath(dest)
	if err != nil {
		return "", "", "", false, err
	}

	if (sourceWorkspace == "" && destWorkspace == "") || (sourceWorkspace != "" && destWorkspace != "") {
		return "", "", "", false, breverrors.NewValidationError("exactly one of source or destination must be an instance path (format: instance_name:/path)")
	}

	if sourceWorkspace != "" {
		return sourceWorkspace, sourcePath, dest, false, nil
	}
	return destWorkspace, destPath, source, true, nil
}

func prepareWorkspace(t *terminal.Terminal, cstore CopyStore, workspaceNameOrID string) (*entity.Workspace, error) {
	s := t.NewSpinner()
	workspace, err := util.GetUserWorkspaceByNameOrIDErr(cstore, workspaceNameOrID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	if workspace.Status == "STOPPED" {
		err = startWorkspaceIfStopped(t, s, cstore, workspaceNameOrID, workspace)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
	}

	err = pollUntil(s, workspace.ID, "RUNNING", cstore, " waiting for instance to be ready...")
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	workspace, err = util.GetUserWorkspaceByNameOrIDErr(cstore, workspaceNameOrID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if workspace.Status != "RUNNING" {
		return nil, breverrors.New("Workspace is not running")
	}

	return workspace, nil
}

func setupSSHConnection(t *terminal.Terminal, cstore CopyStore, workspace *entity.Workspace, host bool) (string, error) {
	refreshRes := refresh.RunRefreshAsync(cstore)

	localIdentifier := workspace.GetLocalIdentifier()
	if host {
		localIdentifier = workspace.GetHostIdentifier()
	}

	sshName := string(localIdentifier)

	err := refreshRes.Await()
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	s := t.NewSpinner()
	err = waitForSSHToBeAvailable(sshName, s)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	return sshName, nil
}

func parseWorkspacePath(path string) (workspace, filePath string, err error) {
	if !strings.Contains(path, ":") {
		return "", path, nil
	}

	parts := strings.Split(path, ":")
	if len(parts) != 2 {
		return "", "", breverrors.NewValidationError("invalid instance path format, use instance_name:/path")
	}

	return parts[0], parts[1], nil
}

func runSCP(sshAlias, localPath, remotePath string, isUpload bool) error {
	var scpCmd *exec.Cmd

	if isUpload {
		scpCmd = exec.Command("scp", localPath, fmt.Sprintf("%s:%s", sshAlias, remotePath)) //nolint:gosec //sshAlias is validated workspace identifier
	} else {
		scpCmd = exec.Command("scp", fmt.Sprintf("%s:%s", sshAlias, remotePath), localPath) //nolint:gosec //sshAlias is validated workspace identifier
	}

	output, err := scpCmd.CombinedOutput()
	if err != nil {
		return breverrors.WrapAndTrace(fmt.Errorf("scp failed: %s\nOutput: %s", err.Error(), string(output)))
	}

	return nil
}

func waitForSSHToBeAvailable(sshAlias string, s *spinner.Spinner) error {
	counter := 0
	s.Suffix = " waiting for SSH connection to be available"
	s.Start()
	for {
		cmd := exec.Command("ssh", "-o", "ConnectTimeout=10", sshAlias, "echo", " ")
		out, err := cmd.CombinedOutput()
		if err == nil {
			s.Stop()
			return nil
		}

		outputStr := string(out)
		stdErr := strings.Split(outputStr, "\n")[1]

		if counter == 40 || !store.SatisfactorySSHErrMessage(stdErr) {
			return breverrors.WrapAndTrace(errors.New("\n" + stdErr))
		}

		counter++
		time.Sleep(1 * time.Second)
	}
}

func startWorkspaceIfStopped(t *terminal.Terminal, s *spinner.Spinner, tstore CopyStore, wsIDOrName string, workspace *entity.Workspace) error {
	activeOrg, err := tstore.GetActiveOrganizationOrDefault()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	workspaces, err := tstore.GetWorkspaceByNameOrID(activeOrg.ID, wsIDOrName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	startedWorkspace, err := tstore.StartWorkspace(workspaces[0].ID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	t.Vprintf(t.Yellow("Instance %s is starting. \n\n", startedWorkspace.Name))
	err = pollUntil(s, workspace.ID, entity.Running, tstore, " hang tight 🤙")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	workspace, err = util.GetUserWorkspaceByNameOrIDErr(tstore, wsIDOrName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func pollUntil(s *spinner.Spinner, wsid string, state string, copyStore CopyStore, waitMsg string) error {
	isReady := false
	s.Suffix = waitMsg
	s.Start()
	for !isReady {
		time.Sleep(5 * time.Second)
		ws, err := copyStore.GetWorkspace(wsid)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		s.Suffix = waitMsg
		if ws.Status == state {
			isReady = true
		}
	}
	return nil
}
