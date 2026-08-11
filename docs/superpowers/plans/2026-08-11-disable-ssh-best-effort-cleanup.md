# Disable SSH Best-Effort Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `brev disable-ssh` attempt local Brev-key cleanup directly at the caller's privilege level before backend work, continue through independent failures, and support an explicit, idempotent `sudo brev disable-ssh` retry without a privileged helper mode.

**Architecture:** Keep the descriptor-safe Linux account sweep unchanged, but invoke it directly from the command with the current effective UID. Use a command-local Cobra persistent hook to defer the root command's normal pre-run until after the local sweep, then split the confirmed operation into two independently attempted obligations—local tagged-key cleanup and remote SSH-record removal—and join their classified errors at the end. Remove the same-binary sudo re-execution path and restore `main.go` to generic CLI startup only.

**Tech Stack:** Go 1.25, Cobra, ConnectRPC/protobuf, the repository's `terminal` and `errors` packages, `os.Geteuid`, and existing Linux `golang.org/x/sys/unix` key-rewrite code.

## Global Constraints

- Implement only in `/Users/pratpatel/code/brev-cli-byon-network-join` on branch `codex/byon-network-join`.
- Treat `docs/superpowers/specs/2026-08-07-byon-network-ssh-separation-design.md` at commit `d1591559` or later as the approved behavioral contract.
- This follow-up supersedes only the privileged-helper and backend-first `disable-ssh` portions of `docs/superpowers/plans/2026-08-07-byon-network-ssh-separation.md`; do not change the already-implemented `join`, `enable-ssh`, `leave`, alias, or SSH-grant boundaries.
- `disable-ssh` remains Linux-only and still requires the global `/etc/brev/device_registration.json` registration before confirmation or mutation.
- Do not automatically invoke `sudo`, re-execute the Brev binary, add a hidden helper argument, or add command-specific dispatch to `main.go`.
- When `os.Geteuid() != 0`, stderr must include: `Warning: not running as root; public key cleanup may be incomplete. Re-run "sudo brev disable-ssh" to allow cleanup across all local accounts.`
- `--approve` skips only confirmation; it does not suppress the non-root, node-wide, or active-session warnings.
- After confirmation, local key cleanup runs before `GetCurrentUser`, `GetNode`, NetBird reconnection, or `RevokeNodeSSHAccess`.
- The root Cobra `PersistentPreRunE` must not execute automatically before `disable-ssh`. Invoke it explicitly as the first remote-phase operation after local cleanup so version, feature-flag, user-home, or `--user` failures are classified as remote cleanup errors.
- Local cleanup and remote-record cleanup are independent obligations. Attempt remote cleanup after a local error, retain both causes, and return nonzero if either obligation is incomplete.
- Classify local failures with `failed to clean up public keys` and auth, node lookup, tunnel, or revocation failures with `failed to remove remote SSH access records`.
- Never print the overall `SSH access disabled` success line when either obligation fails. Partial key counts may be printed as progress, not success.
- Preserve idempotency: marker-free files are not rewritten; each run fetches a fresh `SSHAccess` snapshot; absent grants are not re-revoked; registration and membership are never removed.
- Do not change the secure Linux file traversal/rewrite implementation in `localkeys_linux.go` or its race, symlink, FIFO, ownership, and mode guarantees.
- Keep the generic `pkg/sudo` non-interactive fix. Other commands still depend on it.
- Use TDD for every behavior change: add the focused failing test, observe the expected failure, implement the smallest change, rerun, and commit.
- Run `gofmt` on touched Go files. Keep errors wrapped with `%w` and preserve causes for `errors.Is`/`require.ErrorIs`.

## File Map

### Modify

- `pkg/cmd/disablessh/disablessh.go`: current-EUID warning, direct cleaner dependency, cleanup-first state machine, remote cleanup helper, joined error classification, and registration-based confirmation output.
- `pkg/cmd/disablessh/disablessh_test.go`: warning/root behavior, cleanup-first ordering, independent failure continuation, exact error labels, cancellation, retry idempotency, and removal of sudo-gater expectations.
- `pkg/cmd/disablessh/localkeys.go`: retain only account parsing, Brev-line filtering, account sweeping, and `newSystemLocalKeyCleaner`; delete privileged process/helper protocol.
- `pkg/cmd/disablessh/localkeys_test.go`: retain parser/filter/sweep tests; delete privileged runner and helper-mode tests.
- `main.go`: remove the `disablessh` early-dispatch branch and its command-specific imports.
- `pkg/integration/cli_output_compatibility_test.go`: prove the retired helper token reaches normal Cobra handling rather than a special main entrypoint.
- `pkg/sudo/sudo_test.go`: replace the stale disable-specific fixture reason with generic sudo-test wording; keep the generic non-interactive behavior unchanged.
- `docs/BYON.md`: document current-EUID cleanup, non-root failure behavior, error aggregation, and sudo retry.
- `.agents/skills/brev-cli/SKILL.md`: keep the bundled skill's BYON guidance aligned.
- `.agents/skills/brev-cli/reference/commands.md`: replace the privileged-root-sweep description with the explicit retry contract.
- `CHANGELOG.md`: call out the changed `disable-ssh` elevation and retry behavior.

### Deliberately Unchanged

- `pkg/cmd/disablessh/localkeys_linux.go` and `localkeys_linux_test.go`: the secure rewrite and no-marker no-rewrite behavior already satisfy the new contract.
- `pkg/cmd/disablessh/localkeys_unsupported.go`: the direct cleaner already has a non-Linux companion and the command rejects non-Linux platforms first.
- `pkg/sudo`: remains available to `join` and `leave`; `disable-ssh` simply stops depending on it.
- `pkg/cmd/cmd.go`: continues to register the ordinary top-level `disable-ssh` Cobra command exactly once.

---

### Task 1: Stop Automatically Elevating `disable-ssh`

**Files:**

- Modify: `pkg/cmd/disablessh/disablessh.go:4-50,119-133`
- Modify: `pkg/cmd/disablessh/disablessh_test.go:92-118,232-278,313-350,515-533`

**Interfaces:**

- Consumes: existing `localKeyCleaner.RemoveBrevKeys(context.Context) (KeyCleanupResult, error)` and `newSystemLocalKeyCleaner()`.
- Produces: `disableSSHDeps.geteuid func() int`; production defaults `geteuid` to `os.Geteuid` and `keyCleaner` to `newSystemLocalKeyCleaner()`.

- [ ] **Step 1: Add failing direct-cleaner and warning tests**

Delete `disableSSHTestGater`, remove `gater` from `disableSSHTestHarness`, add an effective UID to the harness, and make the injected getter mutable per test. Add a confirmation hook so warning order is observable:

```go
type disableSSHTestConfirmer struct {
	events        *[]string
	answer        bool
	calls         int
	labels        []string
	beforeConfirm func()
}

func (c *disableSSHTestConfirmer) ConfirmYesNo(label string) bool {
	if c.beforeConfirm != nil {
		c.beforeConfirm()
	}
	c.calls++
	c.labels = append(c.labels, label)
	recordDisableSSHEvent(c.events, "confirm")
	return c.answer
}

type disableSSHTestHarness struct {
	events        []string
	store         *disableSSHTestStore
	registrations *disableSSHTestRegistrationStore
	confirmer     *disableSSHTestConfirmer
	tunnel        *disableSSHTestTunnel
	cleaner       *disableSSHTestKeyCleaner
	client        *disableSSHRecordingClient
	deps          disableSSHDeps
	euid          int
}

func newDisableSSHTestHarness(accesses ...*nodev1.SSHAccess) *disableSSHTestHarness {
	h := &disableSSHTestHarness{euid: 0}
	h.store = &disableSSHTestStore{events: &h.events}
	h.registrations = &disableSSHTestRegistrationStore{
		events: &h.events,
		exists: true,
		reg: &register.DeviceRegistration{
			ExternalNodeID: "node_123",
			DisplayName:    "owned-node",
			OrgID:          "org_123",
			OrgName:        "owned-org",
		},
	}
	h.confirmer = &disableSSHTestConfirmer{events: &h.events, answer: true}
	h.tunnel = &disableSSHTestTunnel{events: &h.events}
	h.cleaner = &disableSSHTestKeyCleaner{
		events: &h.events,
		result: KeyCleanupResult{AccountsScanned: 4, AccountsChanged: 2, KeysRemoved: 3},
	}
	h.client = &disableSSHRecordingClient{
		events:       &h.events,
		node:         &nodev1.ExternalNode{ExternalNodeId: "node_123", Name: "owned-node", SshAccess: accesses},
		revokeErrors: make(map[int]error),
	}
	h.deps = disableSSHDeps{
		platform:          &disableSSHTestPlatform{compatible: true, events: &h.events},
		confirmer:         h.confirmer,
		geteuid:           func() int { return h.euid },
		tunnel:            h.tunnel,
		nodeClients:       disableSSHTestNodeClientFactory{client: h.client},
		registrationStore: h.registrations,
		keyCleaner:        h.cleaner,
	}
	return h
}
```

Add these tests:

```go
func TestDefaultDisableSSHDeps_UsesDirectSystemCleaner(t *testing.T) {
	deps := defaultDisableSSHDeps()
	require.NotNil(t, deps.geteuid)
	_, ok := deps.keyCleaner.(systemLocalKeyCleaner)
	require.True(t, ok)
}

func TestRunDisableSSH_NonRootWarnsAndApproveDoesNotSuppressWarnings(t *testing.T) {
	h := newDisableSSHTestHarness()
	h.euid = 1000

	_, stderr, err := h.run(t, true)
	require.NoError(t, err)
	require.Zero(t, h.confirmer.calls)
	require.Contains(t, stderr, "Warning: not running as root; public key cleanup may be incomplete.")
	require.Contains(t, stderr, `Re-run "sudo brev disable-ssh" to allow cleanup across all local accounts.`)
	require.Contains(t, stderr, "node-wide")
	require.Contains(t, stderr, "active SSH sessions are not forcibly terminated")
}

func TestRunDisableSSH_RootDoesNotPrintNonRootWarning(t *testing.T) {
	h := newDisableSSHTestHarness()
	h.euid = 0

	_, stderr, err := h.run(t, true)
	require.NoError(t, err)
	require.NotContains(t, stderr, "not running as root")
}

func TestRunDisableSSH_NonRootWarningIsWrittenBeforeConfirmation(t *testing.T) {
	h := newDisableSSHTestHarness()
	h.euid = 1000
	var warnings bytes.Buffer
	h.confirmer.beforeConfirm = func() {
		require.Contains(t, warnings.String(), "not running as root")
	}

	_, err := captureDisableSSHStdout(t, func(term *terminal.Terminal) error {
		return runDisableSSH(context.Background(), term, &warnings, h.store, h.deps, false)
	})
	require.NoError(t, err)
}
```

Apply these exact legacy-test changes so the package compiles after deleting the fake gater:

- Delete every `require.Zero(t, h.gater.calls)` and `h.gater.reasons` assertion.
- Rename `TestRunDisableSSH_CancelStopsBeforeSudoTunnelRevocationAndCleanup` to `TestRunDisableSSH_CancelStopsBeforeTunnelRevocationAndCleanup`.
- In `TestRunDisableSSH_ConnectsBeforeFirstRevocation`, temporarily require `tunnel`, `revoke:user_1`, then `cleanup`; Task 2 will move cleanup first.
- In `TestRunDisableSSH_NoGrantsSkipsTunnelAndStillCleansOrphanedKeys`, require only `cleanup` for the mutation subsequence.
- Rename `TestRunDisableSSH_StateMachineOrdersPreflightConfirmationAndSudo` to `TestRunDisableSSH_StateMachineHasNoAutomaticElevation` and require this exact Task 1 sequence:

  ```go
  require.Equal(t, []string{
	  "platform",
	  "registration-exists",
	  "registration-load",
	  "auth",
	  "get-node",
	  "confirm",
	  "tunnel",
	  "revoke:user_1",
	  "cleanup",
  }, h.events)
  ```

At this task boundary, backend-first ordering otherwise remains unchanged.

- [ ] **Step 2: Run the focused tests and observe the expected failure**

Run:

```bash
go test ./pkg/cmd/disablessh -run 'Test(DefaultDisableSSHDeps_UsesDirectSystemCleaner|RunDisableSSH_(NonRootWarnsAndApproveDoesNotSuppressWarnings|RootDoesNotPrintNonRootWarning|NonRootWarningIsWrittenBeforeConfirmation))' -count=1
```

Expected: FAIL because `disableSSHDeps` has no `geteuid`, defaults to `newPrivilegedLocalKeyCleaner`, and does not print the non-root warning.

- [ ] **Step 3: Replace the sudo gate with a direct current-EUID dependency**

In `disablessh.go`, remove the `pkg/sudo` import, add `os`, and make the dependency shape exactly:

```go
type disableSSHDeps struct {
	platform          externalnode.PlatformChecker
	confirmer         terminal.Confirmer
	geteuid           func() int
	tunnel            register.NetBirdConnector
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
	keyCleaner        localKeyCleaner
}

func defaultDisableSSHDeps() disableSSHDeps {
	return disableSSHDeps{
		platform:          register.LinuxPlatform{},
		confirmer:         register.TerminalPrompter{},
		geteuid:           os.Geteuid,
		tunnel:            register.Netbird{},
		nodeClients:       register.DefaultNodeClientFactory{},
		registrationStore: register.NewFileRegistrationStore(),
		keyCleaner:        newSystemLocalKeyCleaner(),
	}
}
```

After normalizing a nil warning writer and before confirmation, add:

```go
if deps.geteuid() != 0 {
	_, _ = fmt.Fprintln(warnings, `Warning: not running as root; public key cleanup may be incomplete. Re-run "sudo brev disable-ssh" to allow cleanup across all local accounts.`)
}
```

Delete the entire `deps.gater.Gate(...)` block. Do not add any replacement sudo prompt or subprocess.

- [ ] **Step 4: Format and run the full command package**

Run:

```bash
gofmt -w pkg/cmd/disablessh/disablessh.go pkg/cmd/disablessh/disablessh_test.go
go test ./pkg/cmd/disablessh -count=1
```

Expected: PASS. Existing backend-first behavior remains green while automatic elevation and its test double are gone.

- [ ] **Step 5: Commit the privilege-boundary change**

```bash
git add pkg/cmd/disablessh/disablessh.go pkg/cmd/disablessh/disablessh_test.go
git commit -m "refactor: stop auto-elevating disable-ssh"
```

---

### Task 2: Attempt Local and Remote Cleanup Independently

**Files:**

- Modify: `pkg/cmd/disablessh/disablessh.go:75-198`
- Modify: `pkg/cmd/disablessh/disablessh_test.go:313-551`

**Interfaces:**

- Consumes: `disableSSHDeps` from Task 1, `cmdcontext.InvokeParentPersistentPreRun`, `register.FetchRegisteredNode`, `revokeSSHAccesses`, and `breverrors.Join`.
- Produces: `disableSSHDeps.prepareRemote func() error` and `removeRemoteSSHAccessRecords(context.Context, DisableSSHStore, disableSSHDeps, *register.DeviceRegistration) error`; `runDisableSSH` always invokes the local cleaner before this helper after confirmation.

- [ ] **Step 1: Replace backend-first tests with cleanup-first failure tests**

Add or replace tests with the following behavior:

```go
func TestRunDisableSSH_CleanupRunsBeforeAuthenticationAndAuthFailureIsRemoteError(t *testing.T) {
	authErr := errors.New("authentication failed")
	h := newDisableSSHTestHarness()
	h.store.currentUserErr = authErr

	stdout, _, err := h.run(t, true)
	require.ErrorIs(t, err, authErr)
	require.Contains(t, err.Error(), "failed to remove remote SSH access records")
	require.NotContains(t, err.Error(), "failed to clean up public keys")
	requireOrderedSubsequence(t, h.events, "registration-load", "cleanup", "auth")
	require.NotContains(t, stdout, "SSH access disabled")
}

func TestNewCmdDisableSSH_ParentPreRunFailureOccursAfterLocalCleanup(t *testing.T) {
	parentErr := errors.New("root pre-run failed")
	h := newDisableSSHTestHarness()
	root := &cobra.Command{
		Use: "brev",
		PersistentPreRunE: func(*cobra.Command, []string) error {
			recordDisableSSHEvent(&h.events, "parent-pre-run")
			return parentErr
		},
	}

	_, err := captureDisableSSHStdout(t, func(term *terminal.Terminal) error {
		root.AddCommand(newCmdDisableSSH(term, h.store, h.deps))
		root.SetArgs([]string{"disable-ssh", "--approve"})
		root.SetErr(io.Discard)
		return root.Execute()
	})
	require.ErrorIs(t, err, parentErr)
	require.Contains(t, err.Error(), "failed to remove remote SSH access records")
	requireOrderedSubsequence(t, h.events, "cleanup", "parent-pre-run")
	require.Equal(t, []string{"parent-pre-run"}, filterDisableSSHEvents(h.events, "parent-pre-run"))
	require.Equal(t, 1, h.cleaner.calls)
	require.Zero(t, h.store.currentUserCalls)
}

func TestRunDisableSSH_LocalFailureStillRemovesRemoteRecords(t *testing.T) {
	cleanupErr := errors.New("alice authorized_keys is not writable")
	h := newDisableSSHTestHarness(testSSHAccess("user_1", "alice", "port_1"))
	h.cleaner.result = KeyCleanupResult{AccountsScanned: 2, AccountsChanged: 1, KeysRemoved: 1}
	h.cleaner.err = cleanupErr

	stdout, _, err := h.run(t, true)
	require.ErrorIs(t, err, cleanupErr)
	require.Contains(t, err.Error(), "failed to clean up public keys")
	require.NotContains(t, err.Error(), "failed to remove remote SSH access records")
	require.Len(t, h.client.revokeRequests, 1)
	requireOrderedSubsequence(t, h.events, "cleanup", "auth", "get-node", "tunnel", "revoke:user_1")
	require.Contains(t, stdout, "1 keys removed")
	require.NotContains(t, stdout, "SSH access disabled")
	require.Zero(t, h.client.removeNodeCalls)
	require.Zero(t, h.client.closePortCalls)
	require.Zero(t, h.tunnel.uninstallCalls)
	require.Zero(t, h.registrations.deleteCalls)
}

func TestRunDisableSSH_LocalAndRemoteFailuresAreJoined(t *testing.T) {
	cleanupErr := errors.New("local cleanup failed")
	revokeErr := errors.New("remote revoke failed")
	h := newDisableSSHTestHarness(testSSHAccess("user_1", "ubuntu", "port_1"))
	h.cleaner.err = cleanupErr
	h.client.revokeErrors[0] = revokeErr

	stdout, _, err := h.run(t, true)
	require.ErrorIs(t, err, cleanupErr)
	require.ErrorIs(t, err, revokeErr)
	require.Contains(t, err.Error(), "failed to clean up public keys")
	require.Contains(t, err.Error(), "failed to remove remote SSH access records")
	require.Equal(t, 1, h.cleaner.calls)
	require.Len(t, h.client.revokeRequests, 1)
	require.NotContains(t, stdout, "SSH access disabled")
}

func TestRunDisableSSH_NonRootFailureThenRootRetrySkipsRemovedRecords(t *testing.T) {
	cleanupErr := errors.New("another account is not writable")
	h := newDisableSSHTestHarness(testSSHAccess("user_1", "ubuntu", "port_1"))
	h.euid = 1000
	h.cleaner.err = cleanupErr

	_, _, err := h.run(t, true)
	require.ErrorIs(t, err, cleanupErr)
	require.Len(t, h.client.revokeRequests, 1)
	require.Equal(t, 1, h.tunnel.ensureCalls)

	h.euid = 0
	h.cleaner.err = nil
	h.client.node.SshAccess = nil
	h.cleaner.result = KeyCleanupResult{}
	_, _, err = h.run(t, true)
	require.NoError(t, err)
	require.Len(t, h.client.revokeRequests, 1, "second run must not re-revoke an absent record")
	require.Equal(t, 1, h.tunnel.ensureCalls, "second run with no records must skip the tunnel")
	require.Equal(t, 2, h.cleaner.calls, "each run rechecks local state idempotently")
}
```

Update the cancellation test to require exactly:

```go
require.Equal(t, []string{"platform", "registration-exists", "registration-load", "confirm"}, h.events)
require.Zero(t, h.cleaner.calls)
require.Zero(t, h.store.currentUserCalls)
```

Replace `TestRunDisableSSH_ShowsGrantAndDistinctLinuxAccountCounts` with a registration-only preflight assertion:

```go
func TestRunDisableSSH_ConfirmationOutputUsesLocalRegistration(t *testing.T) {
	h := newDisableSSHTestHarness()
	h.store.currentUserErr = errors.New("backend unavailable after confirmation")

	stdout, _, err := h.run(t, true)
	require.Error(t, err)
	require.Contains(t, stdout, "owned-node")
	require.Contains(t, stdout, "node_123")
	require.NotContains(t, stdout, "SSH grants:")
	require.NotContains(t, stdout, "Linux accounts:")
}
```

Use these replacements rather than leaving contradictory backend-first tests in the file:

- Replace `TestRunDisableSSH_ShowsGrantAndDistinctLinuxAccountCounts` with `TestRunDisableSSH_ConfirmationOutputUsesLocalRegistration`.
- Replace `TestRunDisableSSH_AnyRevocationFailureBlocksLocalCleanup` with `TestRunDisableSSH_RevocationFailureOccursAfterLocalCleanup`.
- Rename `TestRunDisableSSH_NotFoundRevocationBlocksLocalCleanup` to `TestRunDisableSSH_NotFoundRevocationPreservesCauseAfterLocalCleanup`.
- Rename `TestRunDisableSSH_TunnelFailureStopsBeforeRevocationAndCleanup` to `TestRunDisableSSH_TunnelFailureOccursAfterLocalCleanup`.
- Replace `TestRunDisableSSH_LocalCleanupFailureReturnsErrorAndPreservesMembership` with `TestRunDisableSSH_LocalFailureStillRemovesRemoteRecords`; the existing no-membership-mutation test continues to protect node, registration, NetBird, and port boundaries.
- Rename `TestRunDisableSSH_BackendNodeFailureStopsBeforeConfirmationAndMutation` to `TestRunDisableSSH_BackendNodeFailureOccursAfterConfirmationAndCleanup`.

- [ ] **Step 2: Run the focused state-machine tests and observe RED**

Run:

```bash
go test ./pkg/cmd/disablessh -run 'Test(NewCmdDisableSSH_ParentPreRunFailureOccursAfterLocalCleanup|RunDisableSSH_(CleanupRunsBeforeAuthenticationAndAuthFailureIsRemoteError|LocalFailureStillRemovesRemoteRecords|LocalAndRemoteFailuresAreJoined|NonRootFailureThenRootRetrySkipsRemovedRecords|ConfirmationOutputUsesLocalRegistration))' -count=1
```

Expected: FAIL because current code authenticates before confirmation, stops on the first obligation error, prints backend-derived counts, and runs cleanup last.

- [ ] **Step 3: Extract remote cleanup and implement the two-obligation state machine**

Add `github.com/brevdev/brev-cli/pkg/cmdcontext` to `disablessh.go`. Extend the dependency struct and defaults with:

```go
type disableSSHDeps struct {
	platform          externalnode.PlatformChecker
	confirmer         terminal.Confirmer
	geteuid           func() int
	prepareRemote     func() error
	tunnel            register.NetBirdConnector
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
	keyCleaner        localKeyCleaner
}

func defaultDisableSSHDeps() disableSSHDeps {
	return disableSSHDeps{
		platform:          register.LinuxPlatform{},
		confirmer:         register.TerminalPrompter{},
		geteuid:           os.Geteuid,
		prepareRemote:     func() error { return nil },
		tunnel:            register.Netbird{},
		nodeClients:       register.DefaultNodeClientFactory{},
		registrationStore: register.NewFileRegistrationStore(),
		keyCleaner:        newSystemLocalKeyCleaner(),
	}
}
```

Also set `prepareRemote: func() error { return nil }` in `newDisableSSHTestHarness`. Replace `newCmdDisableSSH` with this command-level hook structure; Cobra v1.8.1 executes only the closest persistent pre-run by default, so the child hook prevents the root hook from running before `RunE`:

```go
func newCmdDisableSSH(t *terminal.Terminal, store DisableSSHStore, deps disableSSHDeps) *cobra.Command {
	var approveFlag bool
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "disable-ssh",
		DisableFlagsInUseLine: true,
		Short:                 "Disable all Brev-managed SSH access on this node",
		Long:                  "Disable every Brev-managed SSH credential on this joined node without changing Brev network membership or the SSH daemon.",
		Example:               "  brev disable-ssh\n  brev disable-ssh --approve",
		Args:                  cobra.NoArgs,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			// Defer the parent's fallible setup until after local key cleanup.
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			runDeps := deps
			runDeps.prepareRemote = func() error {
				return cmdcontext.InvokeParentPersistentPreRun(cmd, args)
			}
			return runDisableSSH(cmd.Context(), t, cmd.ErrOrStderr(), store, runDeps, approveFlag)
		},
	}
	cmd.Flags().BoolVar(&approveFlag, "approve", false, "skip confirmation prompt (assume yes)")
	return cmd
}
```

Replace `runDisableSSH` with this complete state machine:

```go
func runDisableSSH(
	ctx context.Context,
	t *terminal.Terminal,
	warnings io.Writer,
	store DisableSSHStore,
	deps disableSSHDeps,
	skipConfirm bool,
) error { //nolint:funlen // Ordered teardown state machine is intentionally explicit.
	if !deps.platform.IsCompatible() {
		return fmt.Errorf("brev disable-ssh is only supported on Linux")
	}

	exists, err := deps.registrationStore.Exists()
	if err != nil {
		return fmt.Errorf("check joined-device registration: %w", err)
	}
	if !exists {
		return breverrors.New(`This machine has not joined a Brev network; run "brev join" first.`)
	}

	reg, err := deps.registrationStore.Load()
	if err != nil {
		return fmt.Errorf("read joined-device registration: %w", err)
	}

if warnings == nil {
	warnings = io.Discard
}

t.Vprint("")
t.Vprint(t.White("══════════════════════════════════════════════════"))
t.Vprint(t.White("  Disabling Brev-managed SSH access"))
t.Vprint(t.White("══════════════════════════════════════════════════"))
t.Vprint("")
t.Vprintf("  Node:           %s (%s)\n", reg.DisplayName, reg.ExternalNodeID)
t.Vprint("")

if deps.geteuid() != 0 {
	_, _ = fmt.Fprintln(warnings, `Warning: not running as root; public key cleanup may be incomplete. Re-run "sudo brev disable-ssh" to allow cleanup across all local accounts.`)
}
_, _ = fmt.Fprintln(warnings, "Warning: this is a node-wide operation that removes all Brev-managed SSH credentials on this node.")
_, _ = fmt.Fprintln(warnings, "Warning: active SSH sessions are not forcibly terminated.")

if !skipConfirm && !deps.confirmer.ConfirmYesNo("Disable all Brev-managed SSH access on this node?") {
	t.Vprint("Disable SSH canceled.")
	return nil
}

result, localCleanupErr := deps.keyCleaner.RemoveBrevKeys(ctx)
if localCleanupErr != nil {
	localCleanupErr = fmt.Errorf("failed to clean up public keys: %w", localCleanupErr)
}

remoteCleanupErr := removeRemoteSSHAccessRecords(ctx, store, deps, reg)
if remoteCleanupErr != nil {
	remoteCleanupErr = fmt.Errorf("failed to remove remote SSH access records: %w", remoteCleanupErr)
}

if err := breverrors.Join(localCleanupErr, remoteCleanupErr); err != nil {
	t.Vprintf("  Public key cleanup: %d keys removed; %d accounts changed.\n", result.KeysRemoved, result.AccountsChanged)
	return fmt.Errorf("disable SSH incomplete: %w", err)
}

t.Vprintf("%s  SSH access disabled: %d keys removed; %d accounts changed.\n", t.Green("  ✓"), result.KeysRemoved, result.AccountsChanged)
return nil
}
```

Add the remote helper immediately below `runDisableSSH`:

```go
func removeRemoteSSHAccessRecords(
	ctx context.Context,
	store DisableSSHStore,
	deps disableSSHDeps,
	reg *register.DeviceRegistration,
) error {
	if err := deps.prepareRemote(); err != nil {
		return fmt.Errorf("prepare Brev command: %w", err)
	}
	if _, err := store.GetCurrentUser(); err != nil {
		return fmt.Errorf("authenticate Brev user: %w", err)
	}

	node, err := register.FetchRegisteredNode(ctx, deps.nodeClients, store, reg)
	if err != nil {
		return fmt.Errorf("fetch registered node: %w", err)
	}
	accesses := snapshotSSHAccess(node.GetSshAccess())
	if len(accesses) == 0 {
		return nil
	}

	if err := deps.tunnel.EnsureConnected(ctx); err != nil {
		return fmt.Errorf("connect Brev tunnel: %w", err)
	}
	client := deps.nodeClients.NewNodeClient(store, config.GlobalConfig.GetBrevPublicAPIURL())
	if err := revokeSSHAccesses(ctx, client, reg.ExternalNodeID, accesses); err != nil {
		return err
	}
	return nil
}
```

Delete `distinctLinuxAccountCount`; the command deliberately no longer authenticates before confirmation merely to render counts.

- [ ] **Step 4: Update every affected legacy assertion explicitly**

Use these exact replacements while retaining the existing tuple detail, sequential-call, and no-membership-mutation assertions:

```go
// TestRunDisableSSH_ConnectsBeforeFirstRevocation
requireOrderedSubsequence(t, h.events, "cleanup", "auth", "get-node", "tunnel", "revoke:user_1")

// TestRunDisableSSH_StateMachineHasNoAutomaticElevation
require.Equal(t, []string{
	"platform",
	"registration-exists",
	"registration-load",
	"confirm",
	"cleanup",
	"auth",
	"get-node",
	"tunnel",
	"revoke:user_1",
}, h.events)

// TestRunDisableSSH_ContinuesAfterMiddleRevocationFailureAndJoinsErrors
require.ErrorIs(t, err, firstErr)
require.ErrorIs(t, err, middleErr)
require.Contains(t, err.Error(), "failed to remove remote SSH access records")
require.Equal(t, 1, h.cleaner.calls)
require.Len(t, h.client.revokeRequests, 3)

// Rename to TestRunDisableSSH_NotFoundRevocationPreservesCauseAfterLocalCleanup
require.Error(t, err)
require.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
require.Contains(t, err.Error(), "failed to remove remote SSH access records")
require.Equal(t, 1, h.cleaner.calls)

// Rename to TestRunDisableSSH_RevocationFailureOccursAfterLocalCleanup
require.Error(t, err)
require.Contains(t, err.Error(), "failed to remove remote SSH access records")
require.Equal(t, 1, h.cleaner.calls)
require.Len(t, h.client.revokeRequests, 2)

// TestRunDisableSSH_NoGrantsSkipsTunnelAndStillCleansOrphanedKeys
requireOrderedSubsequence(t, h.events, "cleanup", "auth", "get-node")
require.Zero(t, h.tunnel.ensureCalls)
require.Empty(t, h.client.revokeRequests)

// TestRunDisableSSH_IgnoresNilAccessEntries
require.NotContains(t, stdout, "SSH grants:")
require.Len(t, h.client.revokeRequests, 2)

// Rename to TestRunDisableSSH_TunnelFailureOccursAfterLocalCleanup
require.ErrorIs(t, err, tunnelErr)
require.Contains(t, err.Error(), "failed to remove remote SSH access records")
require.Equal(t, 1, h.cleaner.calls)
require.Empty(t, h.client.revokeRequests)

// Rename to TestRunDisableSSH_BackendNodeFailureOccursAfterConfirmationAndCleanup
requireOrderedSubsequence(t, h.events, "confirm", "cleanup", "auth", "get-node")
require.Equal(t, 1, h.cleaner.calls)
require.NotContains(t, stdout, "SSH access disabled")

// TestRunDisableSSH_SuccessIncludesCleanupCounts
require.NoError(t, err)
require.Contains(t, stdout, "SSH access disabled")
require.Contains(t, stdout, "3 keys removed")
require.Contains(t, stdout, "2 accounts changed")
```

- [ ] **Step 5: Run formatting, the full package, and the local idempotency regression**

Run:

```bash
gofmt -w pkg/cmd/disablessh/disablessh.go pkg/cmd/disablessh/disablessh_test.go
go test ./pkg/cmd/disablessh -count=1
go test ./pkg/cmd/disablessh -run '^TestSystemLocalKeyCleaner_AttemptsEveryAccountAndJoinsErrors$|^TestStripBrevManagedAuthorizedKeyLines_NoMarkersReturnsOriginalBytes$' -count=1
```

Expected: PASS. The first command suite proves orchestration idempotency; the second preserves per-account continuation and marker-free byte idempotency.

- [ ] **Step 6: Commit cleanup-first orchestration**

```bash
git add pkg/cmd/disablessh/disablessh.go pkg/cmd/disablessh/disablessh_test.go
git commit -m "fix: make disable-ssh cleanup retryable"
```

---

### Task 3: Remove the Privileged Helper Entrypoint

**Files:**

- Modify: `pkg/integration/cli_output_compatibility_test.go`
- Modify: `main.go:3-23`
- Modify: `pkg/cmd/disablessh/localkeys.go:3-20,117-208`
- Modify: `pkg/cmd/disablessh/localkeys_test.go:120-278`
- Modify: `pkg/sudo/sudo_test.go:34`

**Interfaces:**

- Consumes: Task 1's direct `newSystemLocalKeyCleaner()` dependency.
- Produces: ordinary `main()` startup with no command-specific pre-dispatch; `localkeys.go` exposes no helper token, privileged runner, or same-binary re-execution path.

- [ ] **Step 1: Add a failing process-boundary regression test**

Add this test beside the existing CLI compatibility tests:

```go
func Test_DisableSSHCleanupHelperIsNotAnEntrypoint(t *testing.T) {
	cmd := exec.Command("go", "run", brevCLIPath, "__brev-disable-ssh-cleanup")
	output, err := cmd.CombinedOutput()
	require.Error(t, err)
	assert.Contains(t, string(output), "unknown command")
	assert.NotContains(t, string(output), "privileged Brev key cleanup")
	assert.NotContains(t, string(output), "local cleanup is only supported on Linux")
}
```

- [ ] **Step 2: Run the process test and observe the special-entrypoint failure**

Run:

```bash
go test ./pkg/integration -run '^Test_DisableSSHCleanupHelperIsNotAnEntrypoint$' -count=1
```

Expected: FAIL because current `main.go` intercepts the token before Cobra and reports a privileged-helper error instead of `unknown command`.

- [ ] **Step 3: Restore generic `main.go` startup**

Delete the `context`, `fmt`, and `pkg/cmd/disablessh` imports and the complete `RunLocalKeyCleanupHelper` branch. The resulting file starts as:

```go
package main

import (
	"os"

	"github.com/brevdev/brev-cli/pkg/analytics"
	"github.com/brevdev/brev-cli/pkg/cmd"
	"github.com/brevdev/brev-cli/pkg/cmd/cmderrors"
	"github.com/brevdev/brev-cli/pkg/errors"
)

func main() {
	done := errors.GetDefaultErrorReporter().Setup()
	defer done()
	defer analytics.Close()
	command := cmd.NewDefaultBrevCommand()

	if err := command.Execute(); err != nil {
		analytics.CaptureCommandError()
		cmderrors.DisplayAndHandleError(err)
		done()
		os.Exit(1) //nolint:gocritic // manually call done
	}
}
```

- [ ] **Step 4: Delete the helper protocol and its unit tests**

In `localkeys.go`, delete:

- `cleanupHelperArg`.
- `privilegedCommandRunner`, `execPrivilegedCommandRunner`, and its `Output` method.
- `privilegedLocalKeyCleaner` and `newPrivilegedLocalKeyCleaner`.
- `RunLocalKeyCleanupHelper` and `runLocalKeyCleanupHelper`.

The file must end immediately after:

```go
func newSystemLocalKeyCleaner() localKeyCleaner {
	return systemLocalKeyCleaner{
		listAccounts: listLocalAccounts,
		cleanAccount: cleanLocalAccount,
	}
}
```

Reduce its imports to `bytes`, `context`, `fmt`, `path`, `register`, and `breverrors`.

In `localkeys_test.go`, delete `fakeLocalKeyCleaner`, both privileged-runner fake types, all `TestPrivilegedLocalKeyCleaner_*` tests, `TestExecPrivilegedCommandRunner_IncludesStderrOnFailure`, and all `TestRunLocalKeyCleanupHelper_*` tests. Keep the parser, marker-filter, and `TestSystemLocalKeyCleaner_AttemptsEveryAccountAndJoinsErrors` coverage unchanged.

In `pkg/sudo/sudo_test.go`, change only the fixture reason passed to `gater.Gate`:

```go
err = gater.Gate(terminal.New(), sudoTestConfirmer{}, "Privileged test operation", true)
```

Do not revert or otherwise change the generic non-interactive sudo failure assertion.

- [ ] **Step 5: Format and verify the helper is absent at source and process boundaries**

Run:

```bash
gofmt -w main.go pkg/cmd/disablessh/localkeys.go pkg/cmd/disablessh/localkeys_test.go pkg/integration/cli_output_compatibility_test.go pkg/sudo/sudo_test.go
go test ./pkg/cmd/disablessh -count=1
go test ./pkg/integration -run '^Test_DisableSSHCleanupHelperIsNotAnEntrypoint$' -count=1
go test ./pkg/sudo -count=1
rg -n 'RunLocalKeyCleanupHelper|newPrivilegedLocalKeyCleaner|privilegedLocalKeyCleaner|__brev-disable-ssh-cleanup' main.go pkg/cmd/disablessh
```

Expected: all three test commands PASS. The final `rg` prints no matches and exits 1; that no-match result is success for this verification step.

- [ ] **Step 6: Commit the entrypoint cleanup**

```bash
git add main.go pkg/cmd/disablessh/localkeys.go pkg/cmd/disablessh/localkeys_test.go pkg/integration/cli_output_compatibility_test.go pkg/sudo/sudo_test.go
git commit -m "refactor: remove disable-ssh helper mode"
```

---

### Task 4: Align Documentation and Run Final Verification

**Files:**

- Modify: `docs/BYON.md:46-65`
- Modify: `.agents/skills/brev-cli/SKILL.md:205-230`
- Modify: `.agents/skills/brev-cli/reference/commands.md:555-566`
- Modify: `CHANGELOG.md:16-19`

**Interfaces:**

- Consumes: the completed command behavior from Tasks 1-3.
- Produces: one consistent user contract across BYON docs, bundled agent guidance, command reference, and release notes.

- [ ] **Step 1: Prove the checked-in docs still describe the retired helper**

Run:

```bash
rg -n 'privileged root sweep|privileged.*sweep|revokes each exact active.*then' docs/BYON.md .agents/skills/brev-cli/SKILL.md .agents/skills/brev-cli/reference/commands.md CHANGELOG.md
```

Expected: matches in `docs/BYON.md` and `.agents/skills/brev-cli/reference/commands.md` demonstrate stale backend-first/automatic-elevation copy.

- [ ] **Step 2: Replace the user-facing disable description**

Use this substance everywhere, shortening only to fit the surrounding document:

```text
`disable-ssh` is node-wide. After confirmation it first attempts to remove
Brev-tagged public keys from every local account accessible at the current
privilege level, then removes every remaining backend SSH access record. A
non-root run warns that public-key cleanup may be incomplete. The command still
attempts remote cleanup and exits nonzero if either obligation is incomplete;
rerun `sudo brev disable-ssh` to retry the local sweep with root access.

Retries are safe: files without Brev markers are not rewritten and backend
records already removed are not revoked again. The command leaves ports,
`sshd`, active sessions, network membership, the backend node, and local
registration unchanged.
```

In `CHANGELOG.md` under `### Changed`, add:

```markdown
- `brev disable-ssh` now performs best-effort key cleanup at the caller's privilege level, reports incomplete local or remote cleanup as an error, and recommends an explicit `sudo brev disable-ssh` retry instead of automatically elevating.
```

Do not claim that root guarantees success; immutable files, read-only filesystems, malformed account data, and races still surface as nonzero errors.

- [ ] **Step 3: Verify documentation terminology and formatting**

Run:

```bash
rg -n 'privileged root sweep|__brev-disable-ssh-cleanup|automatically elevat' docs/BYON.md .agents/skills/brev-cli/SKILL.md .agents/skills/brev-cli/reference/commands.md
git diff --check
```

Expected: the first command prints no matches and exits 1. `git diff --check` exits 0.

- [ ] **Step 4: Run focused behavior and race verification**

Run:

```bash
go test ./pkg/cmd/disablessh -count=1
go test -race ./pkg/cmd/disablessh -count=1
go test ./pkg/integration -run '^Test_DisableSSHCleanupHelperIsNotAnEntrypoint$' -count=1
go test ./pkg/cmd ./pkg/cmd/register ./pkg/cmd/enablessh ./pkg/cmd/deregister ./pkg/sudo -count=1
```

Expected: all commands PASS. The race run preserves the descriptor-safe account-sweep coverage while exercising the new orchestration.

- [ ] **Step 5: Verify root and platform builds plus scoped lint**

Run on the macOS development host:

```bash
go build .
go test -c -o /tmp/brev-disablessh-darwin.test ./pkg/cmd/disablessh
golangci-lint run . ./pkg/cmd/... ./pkg/sudo/...
```

Expected: all commands exit 0 and lint reports `0 issues`.

When a Linux amd64 runner or cached Go 1.25 container is available, also run:

```bash
docker run --rm --platform linux/amd64 -v "$PWD":/src -w /src golang:1.25 sh -lc 'go test -c -o /tmp/brev-disablessh-linux.test ./pkg/cmd/disablessh && go build -o /tmp/brev-cli-linux . && go test -race ./pkg/cmd/disablessh -run "^Test(SystemAuthorizedKeysCleaner|ReplaceAuthorizedKeys)" -count=1'
```

Expected: Linux test binary and CLI build succeed; the focused secure-rewrite race suite passes.

- [ ] **Step 6: Attempt the repository-wide suite and classify only known baselines**

Run:

```bash
go test ./...
```

Expected: attempt the full suite. If the known untouched macOS baselines recur, record them separately:

- `e2etest/setup`: hard-coded `/home/ubuntu/brev-cli`.
- `pkg/ssh`: unavailable JetBrains Gateway path followed by the existing nil panic.
- `pkg/store`: Windows/WSL expectations on Darwin.

Any new failure in `main`, `pkg/integration`, `pkg/cmd/disablessh`, `pkg/cmd`, `pkg/cmd/register`, `pkg/cmd/enablessh`, `pkg/cmd/deregister`, or `pkg/sudo` blocks completion.

- [ ] **Step 7: Review the final diff and commit docs**

Run:

```bash
git diff --check
git status --short
git diff --stat
git diff
```

Confirm the final diff contains no `main.go` feature dispatch, helper token, sudo gate, automatic elevation, node/port removal, NetBird uninstall, registration deletion, or unrelated edits.

Then commit:

```bash
git add docs/BYON.md .agents/skills/brev-cli/SKILL.md .agents/skills/brev-cli/reference/commands.md CHANGELOG.md
git commit -m "docs: explain disable-ssh sudo retry"
```
