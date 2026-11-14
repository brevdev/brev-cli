package util

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/briandowns/spinner"
)

var (
	sshAvailabilityConnectTimeoutSeconds = 3
	sshAvailabilityAttemptTimeout        = 5 * time.Second
	sshAvailabilityWaitDelay             = time.Second
	sshAvailabilityRetrySleep            = time.Second
	sshAvailabilityMaxAttempts           = 3
)

// RequireRunning returns an actionable error when the workspace cannot be
// reached over SSH
func RequireRunning(workspace *entity.Workspace) error {
	switch workspace.Status {
	case entity.Running:
		return nil
	case entity.Stopped:
		return breverrors.NewValidationError(fmt.Sprintf(
			"instance %s is not running, please start it with: brev start %s",
			workspace.Name, workspace.Name))
	default:
		return breverrors.NewValidationError(fmt.Sprintf(
			"instance %s is not running (status: %s); run 'brev ls' to check on it",
			workspace.Name, workspace.Status))
	}
}

// WaitForSSHToBeAvailable polls until an SSH connection can be established.
// passed function regenerates the SSH config between attempts. A
// freshly created workspace is RUNNING before its SSH endpoint is published, so
// the generated config has no entry for the alias yet and ssh reports "Could not
// resolve hostname"; retrying the same invocation cannot help, regenerating the
// config can. Both classes of failure share the same attempt budget.
func WaitForSSHToBeAvailable(sshAlias string, s *spinner.Spinner, refreshConfig func() error) error {
	s.Suffix = " waiting for SSH connection to be available"
	s.Start()
	defer s.Stop()
	for attempt := 1; ; attempt++ {
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
			return nil
		}

		stdErr := strings.TrimSpace(string(out))
		if timedOut {
			stdErr = fmt.Sprintf("SSH attempt %d timed out after %s", attempt, sshAvailabilityAttemptTimeout)
		} else if stdErr == "" {
			stdErr = err.Error()
		}

		aliasUnresolved := sshAliasUnresolved(stdErr)
		// A failure that will not resolve by waiting (bad credentials, host key
		// mismatch) fails immediately. It is expected, so print it cleanly
		// rather than dumping a stack trace.
		if !aliasUnresolved && !timedOut && !store.SatisfactorySSHErrMessage(stdErr) {
			return breverrors.NewValidationError("\n" + stdErr)
		}

		if attempt >= sshAvailabilityMaxAttempts {
			if aliasUnresolved {
				return breverrors.NewValidationError(fmt.Sprintf(
					"no SSH address for %s yet; it is probably still provisioning — try again shortly", sshAlias))
			}
			return breverrors.NewValidationError(fmt.Sprintf(
				"SSH to %s was not available after %d attempts: %s\ncheck the instance with: brev ls",
				sshAlias, attempt, stdErr))
		}

		// The endpoint has not been published yet, so the config needs
		// regenerating before the next attempt can possibly succeed.
		if aliasUnresolved && refreshConfig != nil {
			if refreshErr := refreshConfig(); refreshErr != nil {
				// Best effort: the next attempt reports the real problem.
				log.Printf("ssh: could not refresh config for %s: %v", sshAlias, refreshErr)
			}
		}

		s.Stop()
		_, _ = fmt.Fprintf(s.Writer, "still waiting for SSH connection (attempt %d failed; retrying)\n", attempt)
		time.Sleep(sshAvailabilityRetrySleep)
		s.Start()
	}
}

func sshAliasUnresolved(stdErr string) bool {
	return strings.Contains(stdErr, "Could not resolve hostname")
}
