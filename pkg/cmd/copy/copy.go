package copy

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"

	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/cmd/completions"
	"github.com/brevdev/brev-cli/pkg/cmd/refresh"
	"github.com/brevdev/brev-cli/pkg/cmd/util"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

var (
	copyLong    = "Copy files and directories between your local machine and remote instance"
	copyExample = "brev copy instance_name:/path/to/remote/file /path/to/local/file\nbrev copy /path/to/local/file instance_name:/path/to/remote/file\nbrev copy ./local-directory/ instance_name:/remote/path/"
)

type CopyStore interface {
	util.GetWorkspaceByNameOrIDErrStore
	refresh.RefreshStore
	GetOrganizations(options *store.GetOrganizationsOptions) ([]entity.Organization, error)
	GetWorkspaces(organizationID string, options *store.GetWorkspacesOptions) ([]entity.Workspace, error)
	GetCurrentUserKeys() (*entity.UserKeys, error)
	GetAccessToken() (string, error)
}

func NewCmdCopy(t *terminal.Terminal, store CopyStore, noLoginStartStore CopyStore) *cobra.Command {
	var host bool
	cmd := &cobra.Command{
		Annotations:           map[string]string{"access": ""},
		Use:                   "copy",
		Aliases:               []string{"cp", "scp"},
		DisableFlagsInUseLine: true,
		Short:                 "Copy files and directories between local and remote instance",
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
	if _, err := cstore.GetAccessToken(); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	workspaceNameOrID, remotePath, localPath, isUpload, err := parseCopyArguments(source, dest)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	if isUpload {
		err = validateLocalFile(localPath)
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	target, err := util.ResolveWorkspaceOrNode(cstore, workspaceNameOrID)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if target.Node != nil {
		return copyExternalNode(t, cstore, target.Node, localPath, remotePath, isUpload)
	}

	workspace := target.Workspace
	if err := util.RequireRunning(workspace); err != nil {
		return err //nolint:wrapcheck // do not present stack trace for this error
	}

	sshName, err := setupSSHConnection(t, cstore, workspace, host)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = runCopyWithFallback(t, sshName, localPath, remotePath, isUpload)
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
	err = util.WaitForSSHToBeAvailable(sshName, s, func() error { return refresh.RunRefreshAsync(cstore).Await() })
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	return sshName, nil
}

func validateLocalFile(localPath string) error {
	_, err := os.Stat(localPath)
	if err != nil {
		if os.IsNotExist(err) {
			return breverrors.NewValidationError(fmt.Sprintf("local file or directory does not exist: %s", localPath))
		}
		return breverrors.WrapAndTrace(fmt.Errorf("cannot access local file or directory %s: %w", localPath, err))
	}
	return nil
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
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

type commandRunner func(name string, args ...string) ([]byte, error)

func combinedOutputRunner(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...) //nolint:gosec // Command and args come from internal call sites using fixed binaries/flags (rsync/scp).
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("run %s command: %w", name, err)
	}
	return output, nil
}

func runCopyWithFallback(t *terminal.Terminal, sshAlias, localPath, remotePath string, isUpload bool) error {
	source, dest := transferEndpoints(sshAlias, localPath, remotePath, isUpload)

	startTime := time.Now()
	err := transferWithFallback(sshAlias, localPath, remotePath, isUpload, combinedOutputRunner, rsyncInstalledLocally, func(reason string) {
		t.Vprint(t.Yellow("%s\n", reason))
	})
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	duration := time.Since(startTime)
	fmt.Print("\n")
	t.Vprint(t.Green(fmt.Sprintf("✓ Successfully copied %s → %s (%v)\n", source, dest, duration.Round(time.Millisecond))))

	return nil
}

func rsyncInstalledLocally() bool {
	_, err := exec.LookPath("rsync")
	return err == nil
}

func transferWithFallback(sshAlias, localPath, remotePath string, isUpload bool, runner commandRunner, rsyncAvailable func() bool, onFallback func(reason string)) error {
	notifyFallback := func(reason string) {
		if onFallback != nil {
			onFallback(reason)
		}
	}

	// Directory sources are normalized to contents-copy form so the
	// destination always mirrors the source, whether or not it already
	// exists, and rsync and scp produce identical layouts. Uploads can
	// stat the local source; downloads probe the remote path type.
	sourceIsDir := isUpload && isDirectory(localPath)
	if !isUpload {
		sourceIsDir = remotePathIsDir(sshAlias, remotePath, runner)
	}

	if !rsyncAvailable() {
		notifyFallback("rsync not found on this machine, using scp. Install rsync for faster transfers.")
		return runSCPCommand(sshAlias, localPath, remotePath, isUpload, sourceIsDir, runner)
	}

	err := runRsyncCommand(sshAlias, localPath, remotePath, isUpload, sourceIsDir, runner)
	if err == nil {
		return nil
	}

	if strings.Contains(err.Error(), "command not found") {
		notifyFallback("rsync is not installed on the instance, falling back to scp. Install rsync on the instance for faster transfers.")
	} else {
		notifyFallback("rsync failed, falling back to scp...")
	}

	scpErr := runSCPCommand(sshAlias, localPath, remotePath, isUpload, sourceIsDir, runner)
	if scpErr != nil {
		return fmt.Errorf("%v\nscp fallback failed: %w", err, scpErr)
	}

	return nil
}

func remotePathIsDir(sshAlias, remotePath string, runner commandRunner) bool {
	quoted := "'" + strings.ReplaceAll(remotePath, "'", `'\''`) + "'"
	_, err := runner("ssh", sshAlias, "test", "-d", quoted)
	return err == nil
}

func runRsyncCommand(sshAlias, localPath, remotePath string, isUpload, sourceIsDir bool, runner commandRunner) error {
	rsyncArgs := buildRsyncArgs(sshAlias, localPath, remotePath, isUpload, sourceIsDir)
	output, err := runner("rsync", rsyncArgs...)
	if err != nil {
		return fmt.Errorf("rsync failed: %s\nOutput: %s", err.Error(), string(output))
	}
	return nil
}

func runSCPCommand(sshAlias, localPath, remotePath string, isUpload, sourceIsDir bool, runner commandRunner) error {
	scpArgs := buildSCPArgs(sshAlias, localPath, remotePath, isUpload, sourceIsDir)
	output, err := runner("scp", scpArgs...)
	if err != nil {
		return fmt.Errorf("scp failed: %s\nOutput: %s", err.Error(), string(output))
	}
	return nil
}

func buildRsyncArgs(sshAlias, localPath, remotePath string, isUpload, sourceIsDir bool) []string {
	source, dest := transferEndpoints(sshAlias, localPath, remotePath, isUpload)

	rsyncArgs := []string{"-z", "-e", "ssh"}
	if !isUpload || sourceIsDir {
		rsyncArgs = append(rsyncArgs, "-r")
	}
	// A trailing slash makes rsync copy the directory's contents, so the
	// destination becomes the copy instead of having the source directory
	// nested inside it. scp gets the same treatment via "/." below.
	if sourceIsDir && !strings.HasSuffix(source, "/") {
		source += "/"
	}
	rsyncArgs = append(rsyncArgs, source, dest)

	return rsyncArgs
}

func buildSCPArgs(sshAlias, localPath, remotePath string, isUpload, sourceIsDir bool) []string {
	source, dest := transferEndpoints(sshAlias, localPath, remotePath, isUpload)

	scpArgs := []string{}
	if !isUpload || sourceIsDir {
		scpArgs = append(scpArgs, "-r")
	}
	// "dir/." makes scp copy the directory's contents, mirroring rsync's
	// trailing-slash behavior regardless of whether the destination exists.
	if sourceIsDir {
		if !strings.HasSuffix(source, "/") {
			source += "/"
		}
		source += "."
	}
	scpArgs = append(scpArgs, source, dest)

	return scpArgs
}

func transferEndpoints(sshAlias, localPath, remotePath string, isUpload bool) (source, dest string) {
	remoteTarget := fmt.Sprintf("%s:%s", sshAlias, remotePath)
	if isUpload {
		return localPath, remoteTarget
	}
	return remoteTarget, localPath
}

func copyExternalNode(t *terminal.Terminal, cstore CopyStore, node *nodev1.ExternalNode, localPath, remotePath string, isUpload bool) error {
	info, err := util.ResolveExternalNodeSSH(cstore, node)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	alias := info.SSHAlias()

	// Ensure SSH config is up to date so the alias resolves.
	refreshRes := refresh.RunRefreshAsync(cstore)
	if err := refreshRes.Await(); err != nil {
		return breverrors.WrapAndTrace(err)
	}

	s := t.NewSpinner()
	err = util.WaitForSSHToBeAvailable(alias, s, func() error { return refresh.RunRefreshAsync(cstore).Await() })
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return runCopyWithFallback(t, alias, localPath, remotePath, isUpload)
}
