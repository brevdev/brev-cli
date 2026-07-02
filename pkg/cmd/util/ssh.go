package util

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/briandowns/spinner"
)

var (
	sshAvailabilityConnectTimeoutSeconds = 3
	sshAvailabilityAttemptTimeout        = 5 * time.Second
	sshAvailabilityWaitDelay             = time.Second
	sshAvailabilityRetrySleep            = time.Second
	sshAvailabilityMaxAttempts           = 20
)

// WorkspacePollingStore is the minimal interface needed for polling workspace state
type WorkspacePollingStore interface {
	GetWorkspace(workspaceID string) (*entity.Workspace, error)
}

// WorkspaceStartStore is the interface needed for starting stopped workspaces
type WorkspaceStartStore interface {
	GetWorkspaceByNameOrIDErrStore
	WorkspacePollingStore
	StartWorkspace(workspaceID string) (*entity.Workspace, error)
}

// PollUntil polls the workspace status until it matches the desired state or times out
func PollUntil(s *spinner.Spinner, wsid string, state string, pollingStore WorkspacePollingStore, waitMsg string, timeout time.Duration) error {
	s.Suffix = waitMsg
	s.Start()
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			s.Stop()
			return breverrors.WrapAndTrace(fmt.Errorf("timed out waiting for instance to reach %s state after %v", state, timeout))
		}
		time.Sleep(5 * time.Second)
		ws, err := pollingStore.GetWorkspace(wsid)
		if err != nil {
			s.Stop()
			return breverrors.WrapAndTrace(err)
		}
		s.Suffix = waitMsg
		if ws.Status == state {
			s.Stop()
			return nil
		}
	}
}

// WaitForSSHToBeAvailable polls until an SSH connection can be established
func WaitForSSHToBeAvailable(sshAlias string, s *spinner.Spinner) error {
	counter := 0
	s.Suffix = " waiting for SSH connection to be available"
	s.Start()
	for {
		attempt := counter + 1
		ctx, cancel := context.WithTimeout(context.Background(), sshAvailabilityAttemptTimeout)
		cmd := exec.CommandContext(ctx, "ssh",
			"-T",
			"-o", fmt.Sprintf("ConnectTimeout=%d", sshAvailabilityConnectTimeoutSeconds),
			"-o", "ConnectionAttempts=1",
			"-o", "BatchMode=yes",
			"-o", "NumberOfPasswordPrompts=0",
			"-o", "RequestTTY=no",
			"-o", "LogLevel=ERROR",
			sshAlias,
			"true",
		)
		cmd.WaitDelay = sshAvailabilityWaitDelay
		out, err := cmd.CombinedOutput()
		timedOut := ctx.Err() == context.DeadlineExceeded
		cancel()
		if err == nil {
			s.Stop()
			return nil
		}

		stdErr := strings.TrimSpace(string(out))
		if timedOut {
			stdErr = fmt.Sprintf("SSH attempt %d timed out after %s", attempt, sshAvailabilityAttemptTimeout)
		} else if stdErr == "" {
			stdErr = err.Error()
		}

		if counter == sshAvailabilityMaxAttempts || (!timedOut && !store.SatisfactorySSHErrMessage(stdErr)) {
			s.Stop()
			return breverrors.WrapAndTrace(errors.New("\n" + stdErr))
		}

		s.Stop()
		_, _ = fmt.Fprintf(s.Writer, "still waiting for SSH connection (attempt %d failed; retrying)\n", attempt)
		counter++
		time.Sleep(sshAvailabilityRetrySleep)
		s.Start()
	}
}

// StartWorkspaceIfStopped starts a workspace and waits for it to be running
func StartWorkspaceIfStopped(t *terminal.Terminal, s *spinner.Spinner, tstore WorkspaceStartStore, wsIDOrName string, workspace *entity.Workspace, timeout time.Duration) error {
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
	t.Vprintf("%s", t.Yellow("Instance %s is starting. \n\n", startedWorkspace.Name))
	err = PollUntil(s, workspace.ID, entity.Running, tstore, " hang tight 🤙", timeout)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	_, err = GetUserWorkspaceByNameOrIDErr(tstore, wsIDOrName)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}
