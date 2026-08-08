# BYON Network and SSH Separation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `join`/`leave` own BYON NetBird membership, make `enable-ssh`/`disable-ssh` own Brev-managed SSH credentials, and retain `register`/`deregister` only as deprecated aliases.

**Architecture:** Keep persisted registration and shared external-node helpers in `pkg/cmd/register`, add one strict reconnecting NetBird primitive, and put node-wide SSH revocation plus privileged local-key cleanup in a new `pkg/cmd/disablessh` package. The SSH commands validate existing membership before mutation; teardown commands preserve retry state and never cross the membership/credential boundary.

**Tech Stack:** Go 1.25, Cobra, ConnectRPC/protobuf, `golang.org/x/sys/unix` for Linux descriptor-safe file operations, standard-library JSON/process APIs, and the repository's existing terminal, sudo, store, and error packages.

## Global Constraints

- Implement only in `/Users/pratpatel/code/brev-cli-byon-network-join` on branch `codex/byon-network-join`.
- Treat `docs/superpowers/specs/2026-08-07-byon-network-ssh-separation-design.md` as the approved behavioral contract.
- Keep the internal `register` and `deregister` package names, `DeviceRegistration`, the registration file, backend RPC/proto shapes, organization selection, and the default Brev network unchanged.
- `join` must not prompt for SSH, resolve a Linux user, open a port, write `authorized_keys`, or grant SSH access.
- `enable-ssh` may reconnect an existing tunnel, but must never call `AddNode`, choose an organization, or create local registration.
- `disable-ssh` must not remove the node, close ports, stop sshd, uninstall NetBird, or delete registration. It removes all backend SSH-access tuples first and only then sweeps every local account for Brev-tagged keys.
- `leave` must not revoke SSH tuples or edit keys. It removes the node, uninstalls NetBird using the existing semantics, and deletes registration last.
- Keep `grant-ssh` and `revoke-ssh` behavior unchanged.
- All alias and safety warnings go to Cobra stderr; ordinary progress and success output continue through `terminal.Terminal`.
- Do not report success after partial teardown. Wrap errors with operation and tuple/account context and preserve their causes.
- Use TDD for every task: add the focused failing test, run it to observe the expected failure, implement the smallest behavior, rerun, then commit.
- Run `gofmt` on every touched Go file. Do not attribute the known macOS baseline failures in Linux e2e setup, JetBrains Gateway detection, or WSL store tests to this work.

## File Map

### Modify

- `pkg/cmd/register/providers.go`: strict NetBird connection contract and injectable command runner.
- `pkg/cmd/register/register.go`: canonical `join`, `register` alias, hidden legacy flag, and membership-only orchestration.
- `pkg/cmd/register/register_test.go`: join surface, compatibility, and no-SSH regression coverage.
- `pkg/cmd/register/device_registration_store.go`: `brev join` recovery guidance.
- `pkg/cmd/register/device_registration_store_test.go`: guidance assertion.
- `pkg/cmd/register/sshkeys.go`: export the exact Brev-marker predicate for the privileged cleanup package.
- `pkg/cmd/register/sshkeys_test.go`: marker predicate coverage.
- `pkg/cmd/enablessh/enablessh.go`: hard joined-node and strict-tunnel preconditions.
- `pkg/cmd/enablessh/enablessh_test.go`: orchestration ordering and no-mutation tests.
- `pkg/cmd/deregister/deregister.go`: canonical `leave`, alias warning, warnings, and retry-safe membership-only teardown.
- `pkg/cmd/deregister/deregister_test.go`: alias, warning, ordering, retry, and failure tests.
- `pkg/cmd/cmd.go`: canonical root wiring and `disable-ssh` registration.
- `pkg/cmd/cmd_test.go`: root command/alias surface.
- `main.go`: dispatch the fixed privileged cleanup helper before normal CLI initialization.
- `README.md`, `CHANGELOG.md`, `.agents/skills/brev-cli/SKILL.md`, and `.agents/skills/brev-cli/reference/commands.md`: user guidance and release notes.

### Create

- `pkg/cmd/register/providers_test.go`: strict NetBird connection tests.
- `pkg/cmd/register/node.go` and `pkg/cmd/register/node_test.go`: shared registered-node lookup.
- `pkg/cmd/disablessh/disablessh.go` and `pkg/cmd/disablessh/disablessh_test.go`: public node-wide disable command.
- `pkg/cmd/disablessh/localkeys.go` and `pkg/cmd/disablessh/localkeys_test.go`: OS-neutral account parsing, byte filtering, helper protocol, and sudo runner.
- `pkg/cmd/disablessh/localkeys_linux.go` and `pkg/cmd/disablessh/localkeys_linux_test.go`: Linux NSS enumeration and secure `authorized_keys` rewrite.
- `pkg/cmd/disablessh/localkeys_unsupported.go`: unsupported-platform implementation for non-Linux builds.
- `pkg/cmd/disablessh/testdata/passwd.txt`, `authorized_keys.before`, and `authorized_keys.after`: deterministic cleanup fixtures.
- `docs/BYON.md`: explicit onboarding and retirement workflow.

---

## Task 1: Add a Strict, Reconnecting NetBird Connection Primitive

**Files:**

- Modify: `pkg/cmd/register/providers.go`
- Modify: `pkg/cmd/register/register.go`
- Modify: `pkg/cmd/register/register_test.go`
- Modify: `pkg/cmd/deregister/deregister_test.go`
- Create: `pkg/cmd/register/providers_test.go`

- [ ] **Step 1: Write the command-runner and connection tests**

Add a scripted runner to `providers_test.go` that records exact commands and supplies queued output/errors:

```go
type netBirdCall struct {
	name string
	args []string
}

type netBirdResult struct {
	output []byte
	err    error
}

type fakeNetBirdCommandRunner struct {
	results  []netBirdResult
	fallback netBirdResult
	calls    []netBirdCall
}

func (f *fakeNetBirdCommandRunner) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, netBirdCall{name: name, args: append([]string(nil), args...)})
	if len(f.results) == 0 {
		return append([]byte(nil), f.fallback.output...), f.fallback.err
	}
	result := f.results[0]
	f.results = f.results[1:]
	return append([]byte(nil), result.output...), result.err
}

func (f *fakeNetBirdCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	_, err := f.Output(ctx, name, args...)
	return err
}
```

Cover these behaviors with `connectTimeout: 10 * time.Millisecond` and `pollInterval: time.Millisecond`. Set a sticky disconnected or error fallback in timeout tests so polling cannot exhaust a scripted slice:

- `TestNetbirdEnsureConnected_AlreadyConnectedDoesNotReconnect`: active service and `Management: Connected`; no `sudo` call of any kind.
- `TestNetbirdEnsureConnected_StartsInactiveService`: inactive service calls `sudo systemctl start netbird` before status.
- `TestNetbirdEnsureConnected_ReconnectsAndWaitsForConfirmation`: disconnected status calls `sudo netbird up`, observes another disconnected status, then succeeds only on connected status.
- `TestNetbirdEnsureConnected_ReconnectFailure`: `netbird up` failure is returned with `failed to reconnect Brev tunnel` context.
- `TestNetbirdEnsureConnected_StatusNeverConfirmsConnection`: timeout returns `Brev tunnel connection was not confirmed`.
- `TestNetbirdEnsureConnected_StatusErrorsAreNotSuccess`: repeated status errors time out and preserve the last status failure.

- [ ] **Step 2: Run the focused tests and observe the compile failure**

Run:

```bash
go test ./pkg/cmd/register -run '^TestNetbirdEnsureConnected_' -count=1
```

Expected: FAIL because `Netbird` has no injected runner or `EnsureConnected` method.

- [ ] **Step 3: Introduce the strict connector contract and runner**

Replace the old permissive `EnsureRunning` contract with:

```go
type NetBirdConnector interface {
	EnsureConnected(context.Context) error
}

type NetBirdManager interface {
	NetBirdConnector
	Install() error
	Uninstall() error
}
```

In `providers.go`, make the zero value production-safe:

```go
const (
	defaultNetBirdConnectTimeout = 30 * time.Second
	defaultNetBirdPollInterval   = 500 * time.Millisecond
)

type netBirdCommandRunner interface {
	Output(context.Context, string, ...string) ([]byte, error)
	Run(context.Context, string, ...string) error
}

type execNetBirdCommandRunner struct{}

func (execNetBirdCommandRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func (execNetBirdCommandRunner) Run(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type Netbird struct {
	runner         netBirdCommandRunner
	connectTimeout time.Duration
	pollInterval   time.Duration
}
```

Add private default accessors so existing `Netbird{}` construction still works. Implement `EnsureConnected(ctx)` in this order:

1. Check `systemctl is-active netbird`; when inactive or errored, run `sudo systemctl start netbird`.
2. Run `netbird status`; return immediately only when `netbirdManagementConnected` is true.
3. Otherwise run `sudo netbird up`.
4. Poll `netbird status` until it positively reports `Management: Connected`, the caller cancels, or the bounded timeout expires.
5. Treat status-command errors as unconfirmed status, remember the latest error, and include it in the timeout error.

Move `netbirdManagementConnected` from `register.go` beside this implementation. Start the bounded `context.WithTimeout` only after any interactive `sudo` start/up command returns, so the 30-second positive-confirmation window does not cut off a password prompt. Use a timer/ticker for polling and do not sleep unconditionally in tests.

- [ ] **Step 4: Make existing-registration reconciliation use the strict primitive**

Update `checkExistingRegistration` so backend `CONNECTED` status no longer returns before checking the local tunnel. Always call:

```go
if err := deps.netbird.EnsureConnected(ctx); err != nil {
	t.Vprintf("  %s\n", t.Yellow(fmt.Sprintf("Warning: %v", err)))
} else {
	t.Vprint(t.Green("  Brev tunnel is connected."))
}
```

Retain warning-only behavior for this already-joined reconciliation path. Add `TestCheckExistingRegistration_ReconcilesLocalTunnel` by calling the existing helper directly, and update every `mockNetBirdManager` in both `register_test.go` and `deregister_test.go` with `EnsureConnected(context.Context) error` so the repository compiles between tasks. Task 2 may rename the test alongside the user-facing join orchestration.

- [ ] **Step 5: Run tests and format**

Run:

```bash
gofmt -w pkg/cmd/register/providers.go pkg/cmd/register/providers_test.go pkg/cmd/register/register.go pkg/cmd/register/register_test.go pkg/cmd/deregister/deregister_test.go
go test ./pkg/cmd/register ./pkg/cmd/deregister -run 'Test(NetbirdEnsureConnected|CheckExistingRegistration)' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/cmd/register/providers.go pkg/cmd/register/providers_test.go pkg/cmd/register/register.go pkg/cmd/register/register_test.go pkg/cmd/deregister/deregister_test.go
git commit -m "feat: require confirmed Brev tunnel connectivity"
```


---

## Task 2: Make `join` Canonical and Remove SSH from Membership Setup

**Files:**

- Modify: `pkg/cmd/register/register.go`
- Modify: `pkg/cmd/register/register_test.go`
- Modify: `pkg/cmd/register/device_registration_store.go`
- Modify: `pkg/cmd/register/device_registration_store_test.go`
- Modify: `pkg/cmd/cmd.go`
- Modify: `pkg/cmd/cmd_test.go`

- [ ] **Step 1: Add command-surface and compatibility tests**

Add tests with a parent Cobra command so both canonical and alias lookup execute the same command:

```go
func TestNewCmdJoin_CommandSurface(t *testing.T) {
	cmd := NewCmdJoin(testTerminal(t), panicRegisterStore{})
	root := &cobra.Command{Use: "brev"}
	root.AddCommand(cmd)
	resolved, _, err := root.Find([]string{"register"})
	require.NoError(t, err)
	require.Equal(t, "join", cmd.Name())
	require.Equal(t, []string{"register"}, cmd.Aliases)
	require.Same(t, cmd, resolved)
	require.Error(t, cmd.Args(cmd, []string{"unexpected"}))
	require.True(t, cmd.Flags().Lookup("ssh-port").Hidden)
}
```

Also add:

- `TestNewCmdJoin_RegisterAliasWarnsOnExecution`: `register` writes the approved two-line warning to `cmd.SetErr(&stderr)` and then invokes the join handler.
- `TestNewCmdJoin_HelpDoesNotWarn`: `register --help` writes no deprecation warning.
- Table-driven `TestNewCmdJoin_LegacySSHPortFailsBeforeSideEffects` for `join --ssh-port 22`, `join -p 22`, `register --ssh-port 22`, and `register -p 22`; assert the exact migration error and zero platform, sudo, auth, NetBird, RPC, and persistence calls.
- `TestRunJoin_InteractivePromptsOnlyForMembership`: record prompts and assert there is no SSH or port prompt.
- `TestRunJoin_DoesNotOpenPortOrGrantSSH`: a successful join performs AddNode/save/setup but zero OpenPort and GrantNodeSSHAccess calls, and output contains `brev enable-ssh`.
- `Test_LoadRegistration_FailsWhenMissing`: assert the error contains `brev join` and not `brev register`.

- [ ] **Step 2: Run the new tests and observe the expected failure**

```bash
go test ./pkg/cmd/register ./pkg/cmd -run 'Test(NewCmdJoin|RunJoin|LoadRegistration|NewBrevCommand_BYON)' -count=1
```

Expected: FAIL because `NewCmdJoin` and the canonical root command do not exist and registration still owns SSH.

- [ ] **Step 3: Rename the user-facing registration orchestration**

Keep package and storage terminology intact, but rename these symbols:

```go
type joinOpts struct {
	interactive bool
	name        string
	orgName     string
	skipConfirm bool
}

type joinPrompter interface {
	terminal.Confirmer
	terminal.Selector
	Input(terminal.PromptContent) string
}

type joinDeps struct {
	platform          externalnode.PlatformChecker
	prompter          joinPrompter
	gater             sudo.Gater
	netbird           NetBirdManager
	setupRunner       SetupRunner
	nodeClients       externalnode.NodeClientFactory
	hardwareProfiler  HardwareProfiler
	registrationStore RegistrationStore
}

func NewCmdJoin(t *terminal.Terminal, store RegisterStore) *cobra.Command
func runJoin(ctx context.Context, t *terminal.Terminal, store RegisterStore, opts joinOpts, deps joinDeps) error
func runJoinSteps(ctx context.Context, t *terminal.Terminal, store RegisterStore, name string, org *entity.Organization, deps joinDeps) error
```

Add `TerminalPrompter.Input` as a thin wrapper over `terminal.PromptGetInput`, consolidate the confirmer/selector/input dependency behind `joinPrompter`, and update existing tests/mocks to the new names. `defaultJoinDeps` carries forward the current real platform, sudo gate, NetBird, setup runner, node client, hardware profiler, and registration store.

Use canonical Cobra metadata:

```go
Use:     "join",
Aliases: []string{"register"},
Short:   "Join this device to a Brev network",
Args:    cobra.NoArgs,
```

Keep `--name/-n`, `--org/-o`, and `--approve`. Bind `--ssh-port/-p` only as a hidden integer compatibility flag. In `RunE`, before constructing dependencies or calling `runJoin`, use `cmd.Flags().Changed("ssh-port")` so explicit zero also fails:

```go
if cmd.CalledAs() == "register" {
	fmt.Fprintln(cmd.ErrOrStderr(), `Warning: "brev register" is deprecated; use "brev join" instead.`)
	fmt.Fprintln(cmd.ErrOrStderr(), `This command no longer enables SSH; run "brev enable-ssh" separately.`)
}
if cmd.Flags().Changed("ssh-port") {
	return fmt.Errorf("--ssh-port is no longer supported by brev join or brev register; run brev join, then run brev enable-ssh on the joined machine")
}
```

Compute interactive mode only from `nameFlag == "" && orgFlag == ""`. Update long help, examples, confirmation text, progress, success text, Linux-platform error, and rejoin guidance to `join`/`leave` terminology.

- [ ] **Step 4: Delete the SSH tail from join**

Delete `sshPort` from options and remove:

- the SSH enable confirmation;
- `user.Current()` from the join path;
- `grantSSHAccessWithPort` and `grantSSHAccess` from `register.go`;
- the registration-only SSH retry/OpenPort test cases now duplicated by `sshkeys` and `sshkeys_port_resolve` coverage.

Change the successful tail to:

```go
if err := runJoinSteps(ctx, t, s, name, org, deps); err != nil {
	return err
}
t.Vprint("")
t.Vprint("SSH access was not enabled. To enable it for your user, run: brev enable-ssh")
return nil
```

Call `s.GetCurrentUser()` for authentication without retaining a `brevUser`, because membership setup no longer grants SSH.

- [ ] **Step 5: Update recovery guidance and root wiring**

Change the missing-registration error to:

```go
return nil, breverrors.New("device registration not found, run 'brev join' first")
```

In `pkg/cmd/cmd.go`, register only `register.NewCmdJoin(t, externalNodeCmdStore)`. Add `TestNewBrevCommand_BYONCommandSurface` to prove `join` exists once and `register` resolves to the same pointer rather than a separately registered command.

- [ ] **Step 6: Run focused tests and format**

```bash
gofmt -w pkg/cmd/register/register.go pkg/cmd/register/register_test.go pkg/cmd/register/device_registration_store.go pkg/cmd/register/device_registration_store_test.go pkg/cmd/cmd.go pkg/cmd/cmd_test.go
go test ./pkg/cmd/register ./pkg/cmd -run 'Test(NewCmdJoin|RunJoin|LoadRegistration|NewBrevCommand_BYON)' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add pkg/cmd/register/register.go pkg/cmd/register/register_test.go pkg/cmd/register/device_registration_store.go pkg/cmd/register/device_registration_store_test.go pkg/cmd/cmd.go pkg/cmd/cmd_test.go
git commit -m "feat: separate network join from SSH enablement"
```


---

## Task 3: Require Joined, Connected Membership Before `enable-ssh`

**Files:**

- Create: `pkg/cmd/register/node.go`
- Create: `pkg/cmd/register/node_test.go`
- Modify: `pkg/cmd/enablessh/enablessh.go`
- Modify: `pkg/cmd/enablessh/enablessh_test.go`

- [ ] **Step 1: Write shared-node lookup tests**

Add `TestFetchRegisteredNode_Success`, `TestFetchRegisteredNode_RPCError`, and `TestFetchRegisteredNode_NilNodeIsError`. The helper contract is:

```go
func FetchRegisteredNode(
	ctx context.Context,
	nodeClients externalnode.NodeClientFactory,
	tokenProvider externalnode.TokenProvider,
	reg *DeviceRegistration,
) (*nodev1.ExternalNode, error)
```

Assert the request contains both `ExternalNodeId` and `OrganizationId`, RPC errors retain `error retrieving joined node` context, and a response with no node returns a nonnil error.

- [ ] **Step 2: Write full enable orchestration tests**

Introduce fakes for platform, registration store, connector, provisioner, and store. Record operation order and add:

- `TestNewCmdEnableSSH_RejectsPositionalArguments`.
- `TestRunEnableSSH_MissingRegistrationDirectsUserToJoin`: exact guidance, no auth, node lookup, tunnel, or provisioner call.
- `TestRunEnableSSH_MissingBackendNodeDoesNotConnectOrProvision`.
- `TestRunEnableSSH_ConnectedTunnelProvisionsSSH`: order is platform, registration, auth, node, tunnel, provision.
- `TestRunEnableSSH_ReconnectsBeforeProvisioning`: the fake connector changes disconnected to connected and provision runs after it.
- `TestRunEnableSSH_TunnelFailureDoesNotProvision`.
- `TestRunEnableSSH_UnconfirmedTunnelDoesNotProvision`.
- `TestRunEnableSSH_NeverAddsNode`: the fake node service fails the test if `AddNode` is called.

The injected mutation boundary should be:

```go
type sshAccessProvisioner interface {
	Provision(
		context.Context,
		*terminal.Terminal,
		externalnode.TokenProvider,
		*register.DeviceRegistration,
		*entity.User,
		*nodev1.ExternalNode,
	) error
}
```

- [ ] **Step 3: Run the tests and observe failure**

```bash
go test ./pkg/cmd/register ./pkg/cmd/enablessh -run 'Test(FetchRegisteredNode|NewCmdEnableSSH|RunEnableSSH)' -count=1
```

Expected: FAIL because lookup is local to `enablessh` and SSH provisioning precedes a strict tunnel check.

- [ ] **Step 4: Add the shared registered-node helper**

Create `node.go` with the signature above. Build the existing `GetNodeRequest`, wrap RPC failure, and reject `resp == nil`, `resp.Msg == nil`, or `resp.Msg.GetExternalNode() == nil` with:

```text
registered node was not returned by Brev; run "brev leave" and "brev join" to repair membership
```

Delete the private `fetchRegisteredNode` from `enablessh.go` and use the shared helper in both SSH commands added by this plan.

- [ ] **Step 5: Refactor enablement behind a post-connect provisioner**

Use these dependencies:

```go
type enableSSHDeps struct {
	platform          externalnode.PlatformChecker
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
	tunnel            register.NetBirdConnector
	provisioner       sshAccessProvisioner
}

type defaultSSHAccessProvisioner struct {
	prompter    terminal.Selector
	nodeClients externalnode.NodeClientFactory
}
```

`defaultEnableSSHDeps` must use `register.LinuxPlatform{}`, `register.NewFileRegistrationStore()`, `register.Netbird{}`, and a `defaultSSHAccessProvisioner` built with the same real node-client factory and terminal selector.

Move current Linux-user lookup, `checkSSHDaemon`, `ResolveSSHAccessPort`, and `SetupAndRegisterNodeSSHAccess` into `defaultSSHAccessProvisioner.Provision`. Keep the success output in `runEnableSSH` after the provisioner returns.

Implement this exact mutation boundary in `runEnableSSH`:

```go
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
brevUser, err := s.GetCurrentUser()
if err != nil {
	return breverrors.WrapAndTrace(err)
}
node, err := register.FetchRegisteredNode(ctx, deps.nodeClients, s, reg)
if err != nil {
	return fmt.Errorf("enable SSH failed: %w", err)
}
if err := deps.tunnel.EnsureConnected(ctx); err != nil {
	return fmt.Errorf("enable SSH requires a connected Brev tunnel: %w", err)
}
if err := deps.provisioner.Provision(ctx, t, s, reg, brevUser, node); err != nil {
	return fmt.Errorf("enable SSH failed: %w", err)
}
```

Add `Args: cobra.NoArgs` and update help to say “joined node.” A healthy tunnel performs no privileged command; only `Netbird.EnsureConnected` invokes interactive `sudo` when it must start the service or run `netbird up`. Do not add an unconditional sudo gate, AddNode, organization, or registration-save behavior.

- [ ] **Step 6: Run tests and format**

```bash
gofmt -w pkg/cmd/register/node.go pkg/cmd/register/node_test.go pkg/cmd/enablessh/enablessh.go pkg/cmd/enablessh/enablessh_test.go
go test ./pkg/cmd/register ./pkg/cmd/enablessh -run 'Test(FetchRegisteredNode|NewCmdEnableSSH|RunEnableSSH)' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add pkg/cmd/register/node.go pkg/cmd/register/node_test.go pkg/cmd/enablessh/enablessh.go pkg/cmd/enablessh/enablessh_test.go
git commit -m "feat: require joined tunnel before enabling SSH"
```


---

## Task 4: Build a Privileged, Node-Wide Brev Key Cleanup Boundary

**Files:**

- Modify: `pkg/cmd/register/sshkeys.go`
- Modify: `pkg/cmd/register/sshkeys_test.go`
- Modify: `main.go`
- Create: `pkg/cmd/disablessh/localkeys.go`
- Create: `pkg/cmd/disablessh/localkeys_test.go`
- Create: `pkg/cmd/disablessh/localkeys_linux.go`
- Create: `pkg/cmd/disablessh/localkeys_linux_test.go`
- Create: `pkg/cmd/disablessh/localkeys_unsupported.go`
- Create: `pkg/cmd/disablessh/testdata/passwd.txt`
- Create: `pkg/cmd/disablessh/testdata/authorized_keys.before`
- Create: `pkg/cmd/disablessh/testdata/authorized_keys.after`

The existing sudo gate only validates or refreshes credentials; it does not elevate the Go process. A node-wide sweep therefore needs a narrow privileged subprocess rather than calling `RemoveBrevAuthorizedKeys` for other users from the unprivileged CLI.

- [ ] **Step 1: Expose and lock down the marker predicate**

Rename the existing private helper without changing its semantics:

```go
// IsBrevManagedAuthorizedKeysLine reports whether a line was managed by a
// current or legacy Brev CLI SSH flow.
func IsBrevManagedAuthorizedKeysLine(line string) bool {
	return strings.Contains(line, BrevKeyPrefixLegacy) || strings.Contains(line, "#brev-portID:")
}
```

Update internal callers and add table-driven coverage for current marker, legacy marker, unrelated key, blank line, and a comment that contains neither exact marker.

- [ ] **Step 2: Add pure account-parser and byte-filter tests**

Create fixtures with root, two normal users, a service account, and two users sharing one home. `authorized_keys.before` must include an unrelated options-prefixed key, one current Brev marker, one legacy marker, a blank line, CRLF content, and a final newline. `authorized_keys.after` must contain every unrelated byte in the original order.

Define the OS-neutral shapes:

```go
const cleanupHelperArg = "__brev-disable-ssh-cleanup"

type KeyCleanupResult struct {
	AccountsScanned int `json:"accounts_scanned"`
	AccountsChanged int `json:"accounts_changed"`
	KeysRemoved     int `json:"keys_removed"`
}

type localAccount struct {
	Username string
	HomeDir  string
}

type localKeyCleaner interface {
	RemoveBrevKeys(context.Context) (KeyCleanupResult, error)
}

func parsePasswd(data []byte) ([]localAccount, error)
func stripBrevManagedAuthorizedKeyLines(data []byte) (cleaned []byte, removed int)
```

Add:

- `TestParsePasswd_EnumeratesAndDeduplicatesHomes`: all account types remain; duplicate home appears once.
- `TestParsePasswd_RejectsMalformedRecord`: fewer than seven fields is an error with line context.
- `TestParsePasswd_RejectsRelativeHome`: only absolute home paths are accepted.
- `TestStripBrevManagedAuthorizedKeyLines_PreservesUnrelatedBytes`: compare exact bytes with `authorized_keys.after` and assert two removals.
- `TestStripBrevManagedAuthorizedKeyLines_NoMarkersReturnsOriginalBytes`: zero removals and byte equality.

Implement filtering with `bytes.SplitAfter(data, []byte("\n"))`; remove the trailing `\n` and optional `\r` only for marker classification, and append every unremoved segment unchanged. This preserves blank lines, CRLF, ordering, and final-newline state.

- [ ] **Step 3: Add aggregate-cleaner tests**

Use injected account listing and per-account cleaning:

```go
type systemLocalKeyCleaner struct {
	listAccounts func(context.Context) ([]localAccount, error)
	cleanAccount func(localAccount) (int, error)
}

func (c systemLocalKeyCleaner) RemoveBrevKeys(context.Context) (KeyCleanupResult, error)
func newSystemLocalKeyCleaner() localKeyCleaner
func newPrivilegedLocalKeyCleaner() localKeyCleaner
```

Add `TestSystemLocalKeyCleaner_AttemptsEveryAccountAndJoinsErrors`. Return failures for the first and third account, verify the second still runs, verify the result counts successful removals, and assert the combined error contains both usernames and home paths. Use `breverrors.Join` after wrapping each account failure.

- [ ] **Step 4: Run the pure tests and observe failure**

```bash
go test ./pkg/cmd/register ./pkg/cmd/disablessh -run 'Test(IsBrevManaged|ParsePasswd|StripBrev|SystemLocalKeyCleaner)' -count=1
```

Expected: FAIL because the package and exported predicate do not exist.

- [ ] **Step 5: Implement the OS-neutral cleanup and sudo protocol**

Add a privileged runner with injectable seams:

```go
type privilegedCommandRunner interface {
	Output(context.Context, string, ...string) ([]byte, error)
}

type privilegedLocalKeyCleaner struct {
	geteuid    func() int
	executable func() (string, error)
	runner     privilegedCommandRunner
	direct     localKeyCleaner
}

func (c privilegedLocalKeyCleaner) RemoveBrevKeys(ctx context.Context) (KeyCleanupResult, error) {
	if c.geteuid() == 0 {
		return c.direct.RemoveBrevKeys(ctx)
	}
	executable, err := c.executable()
	if err != nil {
		return KeyCleanupResult{}, fmt.Errorf("locate Brev executable: %w", err)
	}
	output, err := c.runner.Output(ctx, "sudo", "-n", executable, cleanupHelperArg)
	if err != nil {
		return KeyCleanupResult{}, fmt.Errorf("run privileged Brev key cleanup: %w", err)
	}
	var result KeyCleanupResult
	if err := json.Unmarshal(output, &result); err != nil {
		return KeyCleanupResult{}, fmt.Errorf("decode privileged Brev key cleanup result: %w", err)
	}
	return result, nil
}
```

The real runner must use `exec.CommandContext(...).Output()` and include `*exec.ExitError.Stderr` in its returned error, but never mix stderr into the JSON stdout stream.

Add this exported dispatcher for `main.go`:

```go
func RunLocalKeyCleanupHelper(ctx context.Context, args []string, stdout io.Writer) (bool, error)
```

It returns `(false, nil)` unless the first argument exactly equals `cleanupHelperArg`. Once selected, it accepts exactly one argument, requires Linux, requires `os.Geteuid() == 0`, invokes `newSystemLocalKeyCleaner`, and JSON-encodes only `KeyCleanupResult` to stdout. It accepts no usernames, home directories, or file paths.

Use an unexported injected variant in tests and add:

- `TestPrivilegedLocalKeyCleaner_RootRunsDirectly`.
- `TestPrivilegedLocalKeyCleaner_UsesFixedSudoCommandWhenNotRoot`.
- `TestPrivilegedLocalKeyCleaner_RejectsInvalidJSON`.
- `TestRunLocalKeyCleanupHelper_IgnoresNormalCLIArguments`.
- `TestRunLocalKeyCleanupHelper_RejectsExtraArguments`.
- `TestRunLocalKeyCleanupHelper_RejectsNonRoot`.
- `TestRunLocalKeyCleanupHelper_EmitsJSON`.

- [ ] **Step 6: Implement secure Linux account enumeration and file replacement**

Put Linux implementation behind `//go:build linux`. Resolve `getent` only from fixed candidates `/usr/bin/getent` and `/bin/getent`, run `getent passwd`, and parse its stdout. A missing command, nonzero exit, or malformed record fails the sweep instead of falsely reporting completeness.

For each deduplicated absolute home:

1. Start from an open descriptor for `/` and walk every cleaned absolute-home component with `unix.Openat(..., unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW)`. Reject `..`, symlinks in any intermediate or final component, and non-directories; `O_NOFOLLOW` on one absolute-path open is insufficient because it protects only the final component.
2. Open literal `.ssh` from the verified home descriptor with the same directory/no-follow flags.
3. Use `unix.Fstatat` with `AT_SYMLINK_NOFOLLOW` on literal `authorized_keys` and require a regular file before opening it. Then open it with `unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK`, immediately `Fstat` the descriptor again to close the race, and reject a FIFO, device, directory, socket, or changed/non-regular target before reading.
4. Treat `ENOENT` for a home component, `.ssh`, or `authorized_keys` as zero removals. Return every other unsafe-type or path error with account and path context.
5. Read the file from its descriptor, filter it, and skip all writes when no markers match.
6. Record the original `Stat_t.Uid`, `Stat_t.Gid`, and permission bits.
7. Create a random `authorized_keys.brev-cleanup-*` file in the already-open `.ssh` directory with `O_CREAT|O_EXCL|O_WRONLY|O_NOFOLLOW`.
8. Write all cleaned bytes, `Fchown` to the original UID/GID, then `Fchmod` to the original permission bits (chown can clear mode bits), `Fsync`, atomically `Renameat` over literal `authorized_keys`, and `Fsync` the directory.
9. Close descriptors on every path and unlink an unrenamed temporary file on failure.

Do not recurse, follow symlinks, evaluate shell text, or accept a path from the caller. The non-Linux file must return `brev disable-ssh local cleanup is only supported on Linux` while preserving compilation on Darwin.

- [ ] **Step 7: Add Linux filesystem tests**

Behind `//go:build linux`, add:

- `TestSystemAuthorizedKeysCleaner_RemovesBothMarkersAndPreservesModeAndOwnership`: use a mode with the setgid bit and verify the full promised mode after the required `Fchown`-then-`Fchmod` order.
- `TestSystemAuthorizedKeysCleaner_NoMarkersDoesNotRewrite`: compare inode before/after to prove no replacement.
- `TestSystemAuthorizedKeysCleaner_MissingSSHDirectoryIsSuccess`.
- `TestSystemAuthorizedKeysCleaner_MissingAuthorizedKeysIsSuccess`.
- `TestSystemAuthorizedKeysCleaner_RejectsSSHDirectorySymlink`.
- `TestSystemAuthorizedKeysCleaner_RejectsAuthorizedKeysSymlink`.
- `TestSystemAuthorizedKeysCleaner_RejectsIntermediateHomeSymlink`.
- `TestSystemAuthorizedKeysCleaner_RejectsFIFOWithoutBlocking`.
- `TestSystemAuthorizedKeysCleaner_RejectsNonRegularAuthorizedKeys`.

Use `t.TempDir()` only; never point tests at a real account home. Ownership assertions may compare unchanged UID/GID without changing them.

- [ ] **Step 8: Dispatch the helper before normal CLI initialization**

At the top of `main`, before Sentry, analytics, stores, version checks, or Cobra setup:

```go
handled, err := disablessh.RunLocalKeyCleanupHelper(context.Background(), os.Args[1:], os.Stdout)
if handled {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return
}
```

This prevents the root subprocess from logging in, creating analytics events, or running a second backend operation.

- [ ] **Step 9: Run tests, format, and verify both build targets**

```bash
gofmt -w main.go pkg/cmd/register/sshkeys.go pkg/cmd/register/sshkeys_test.go pkg/cmd/disablessh/localkeys.go pkg/cmd/disablessh/localkeys_test.go pkg/cmd/disablessh/localkeys_linux.go pkg/cmd/disablessh/localkeys_linux_test.go pkg/cmd/disablessh/localkeys_unsupported.go
go test ./pkg/cmd/register ./pkg/cmd/disablessh -run 'Test(IsBrevManaged|ParsePasswd|StripBrev|SystemLocalKeyCleaner|PrivilegedLocalKeyCleaner|RunLocalKeyCleanupHelper)' -count=1
go test . -run '^$'
GOOS=linux GOARCH=amd64 go test -c -o /tmp/brev-disablessh-linux.test ./pkg/cmd/disablessh
GOOS=linux GOARCH=amd64 go build -o /tmp/brev-cli-linux .
```

Expected: OS-neutral tests PASS locally, the Darwin root executable compiles with the early dispatcher, and both the Linux package tests and Linux root executable cross-compile. Run the tagged filesystem tests on a Linux runner during final verification.

- [ ] **Step 10: Commit**

```bash
git add main.go pkg/cmd/register/sshkeys.go pkg/cmd/register/sshkeys_test.go pkg/cmd/disablessh/localkeys.go pkg/cmd/disablessh/localkeys_test.go pkg/cmd/disablessh/localkeys_linux.go pkg/cmd/disablessh/localkeys_linux_test.go pkg/cmd/disablessh/localkeys_unsupported.go pkg/cmd/disablessh/testdata
git commit -m "feat: add privileged node-wide Brev key cleanup"
```


---

## Task 5: Add Backend-First, Node-Wide `disable-ssh`

**Files:**

- Create: `pkg/cmd/disablessh/disablessh.go`
- Create: `pkg/cmd/disablessh/disablessh_test.go`
- Modify: `pkg/cmd/cmd.go`
- Modify: `pkg/cmd/cmd_test.go`

- [ ] **Step 1: Define orchestration seams and write command tests**

Use these narrow command dependencies:

```go
type DisableSSHStore interface {
	GetCurrentUser() (*entity.User, error)
	GetAccessToken() (string, error)
}

type disableSSHDeps struct {
	platform          externalnode.PlatformChecker
	confirmer         terminal.Confirmer
	gater             sudo.Gater
	tunnel            register.NetBirdConnector
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
	keyCleaner        localKeyCleaner
}

func NewCmdDisableSSH(t *terminal.Terminal, store DisableSSHStore) *cobra.Command
func newCmdDisableSSH(t *terminal.Terminal, store DisableSSHStore, deps disableSSHDeps) *cobra.Command
func runDisableSSH(
	ctx context.Context,
	t *terminal.Terminal,
	warnings io.Writer,
	store DisableSSHStore,
	deps disableSSHDeps,
	skipConfirm bool,
) error
```

`defaultDisableSSHDeps` must use `register.LinuxPlatform{}`, `register.TerminalPrompter{}`, `sudo.Default`, `register.Netbird{}`, `register.DefaultNodeClientFactory{}`, `register.NewFileRegistrationStore()`, and `newPrivilegedLocalKeyCleaner()`. The public constructor must close over these defaults; tests call the injected constructor.

Add `TestNewCmdDisableSSH_CommandSurface` and `TestNewCmdDisableSSH_RejectsArguments`. Assert `Use: "disable-ssh"`, `Args: cobra.NoArgs`, configuration annotation, and `--approve`.

- [ ] **Step 2: Write state-machine tests before implementation**

Use a fake ConnectRPC service that records GetNode, RevokeNodeSSHAccess, RemoveNode, ClosePort, and AddNode calls. Add:

- `TestRunDisableSSH_MissingRegistrationDoesNotAuthenticateOrCallRPC`.
- `TestRunDisableSSH_CancelStopsBeforeSudoTunnelRevocationAndCleanup`.
- `TestRunDisableSSH_ApproveSkipsConfirmationButPrintsSafetyWarning`.
- `TestRunDisableSSH_ShowsGrantAndDistinctLinuxAccountCounts`.
- `TestRunDisableSSH_IgnoresNilAccessEntries`.
- `TestRunDisableSSH_ConnectsBeforeFirstRevocation`.
- `TestRunDisableSSH_RevokesEveryExactTupleSequentiallyOnce`.
- `TestRunDisableSSH_ContinuesAfterMiddleRevocationFailureAndJoinsErrors`.
- `TestRunDisableSSH_AnyRevocationFailureBlocksLocalCleanup`.
- `TestRunDisableSSH_NoGrantsSkipsTunnelAndStillCleansOrphanedKeys`.
- `TestRunDisableSSH_LocalCleanupFailureReturnsErrorAndPreservesMembership`.
- `TestRunDisableSSH_DoesNotRemoveNodeClosePortUninstallNetBirdOrDeleteRegistration`.

Use access records with repeated Linux accounts so the warning asserts both total-grant and distinct-account counts. For exact tuple assertions, compare:

```go
&nodev1.RevokeNodeSSHAccessRequest{
	ExternalNodeId: reg.ExternalNodeID,
	PortId:         access.GetPortId(),
	UserId:         access.GetUserId(),
	LinuxUser:      access.GetLinuxUser(),
}
```

- [ ] **Step 3: Run the new tests and observe failure**

```bash
go test ./pkg/cmd/disablessh ./pkg/cmd -run 'Test(NewCmdDisableSSH|RunDisableSSH|NewBrevCommand_BYON)' -count=1
```

Expected: FAIL because the public command does not exist.

- [ ] **Step 4: Implement preflight and confirmation**

Configure the command as:

```go
Use:                   "disable-ssh",
Short:                 "Disable all Brev-managed SSH access on this node",
Args:                  cobra.NoArgs,
DisableFlagsInUseLine: true,
Annotations:           map[string]string{"configuration": ""},
```

Implement preflight in this order:

1. Linux compatibility.
2. `registrationStore.Exists`; if false, return `This machine has not joined a Brev network; run "brev join" first.`
3. Load registration.
4. Authenticate with `GetCurrentUser`.
5. Fetch the registered node through `register.FetchRegisteredNode`.
6. Copy every non-nil entry from `node.GetSshAccess()` into a new slice before mutation; nil protobuf entries are not active grants.
7. Print node, total grants, distinct Linux accounts, and the warning that existing sessions are not forcibly terminated.
8. Confirm unless `--approve`; a cancellation returns nil.
9. Only after confirmation, call the sudo gate with reason `Node-wide Brev SSH cleanup`.

The active-session and node-wide-scope warnings must use the supplied stderr writer even with `--approve`.

- [ ] **Step 5: Implement backend-first revocation and conditional cleanup**

If the access snapshot is nonempty, call `deps.tunnel.EnsureConnected(ctx)` before creating any revoke request. If it fails, return without local cleanup. When the snapshot is empty, skip the tunnel entirely.

Revoke sequentially in slice order. Continue after errors and collect each with exact context:

```go
revokeErrs = append(revokeErrs, fmt.Errorf(
	"revoke SSH access for user %q, Linux account %q, port %q: %w",
	access.GetUserId(),
	access.GetLinuxUser(),
	access.GetPortId(),
	err,
))
```

After the loop:

```go
if err := breverrors.Join(revokeErrs...); err != nil {
	return fmt.Errorf("disable SSH backend cleanup incomplete: %w", err)
}
result, err := deps.keyCleaner.RemoveBrevKeys(ctx)
if err != nil {
	return fmt.Errorf("disable SSH local key cleanup incomplete: %w", err)
}
```

Do not suppress arbitrary RPC NotFound errors: the backend revoke operation itself is already idempotent for an absent exact tuple, while a transport NotFound can mean a missing node or port and must not authorize a broad local sweep.

Print success only after the cleaner succeeds, including `result.KeysRemoved` and `result.AccountsChanged`. Keep membership and registration intact on every outcome.

- [ ] **Step 6: Wire the command exactly once at the root**

Import `pkg/cmd/disablessh` in `pkg/cmd/cmd.go` and add:

```go
cmd.AddCommand(disablessh.NewCmdDisableSSH(t, externalNodeCmdStore))
```

Extend `TestNewBrevCommand_BYONCommandSurface` to assert one canonical `disable-ssh` command and no alias.

- [ ] **Step 7: Run tests and format**

```bash
gofmt -w pkg/cmd/disablessh/disablessh.go pkg/cmd/disablessh/disablessh_test.go pkg/cmd/cmd.go pkg/cmd/cmd_test.go
go test ./pkg/cmd/disablessh ./pkg/cmd -run 'Test(NewCmdDisableSSH|RunDisableSSH|NewBrevCommand_BYON)' -count=1
go test -race ./pkg/cmd/disablessh -count=1
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add pkg/cmd/disablessh/disablessh.go pkg/cmd/disablessh/disablessh_test.go pkg/cmd/cmd.go pkg/cmd/cmd_test.go
git commit -m "feat: add node-wide disable-ssh command"
```

---

## Task 6: Make `leave` Canonical and Membership-Only

**Files:**

- Modify: `pkg/cmd/deregister/deregister.go`
- Modify: `pkg/cmd/deregister/deregister_test.go`
- Modify: `pkg/cmd/cmd.go`
- Modify: `pkg/cmd/cmd_test.go`

The current backend masks a missing `GetNode` as Connect `PermissionDenied`, while `RemoveNode` itself is idempotent for a missing node. Do not downgrade `PermissionDenied`: preserve the approved stop-on-lookup-error contract by performing leave preflight through organization-scoped `ListNodes` and matching the persisted external-node ID. An absent ID is a retryable missing node only when the response is complete; if the response has a next-page token, stop because the current backend ignores requested page parameters and the CLI cannot safely prove absence.

- [ ] **Step 1: Write canonical command and alias tests**

Add:

- `TestNewCmdLeave_CommandSurface`: `Use: "leave"`, alias `deregister`, `Args: cobra.NoArgs`, configuration annotation, and `--approve`.
- `TestNewCmdLeave_DeregisterAliasWarnsOnExecution`: assert the approved two-line warning on Cobra stderr.
- `TestNewCmdLeave_HelpDoesNotWarn`.
- `TestNewCmdLeave_RejectsArguments` for both canonical and alias invocations.

The execution-only alias warning is:

```go
if cmd.CalledAs() == "deregister" {
	fmt.Fprintln(cmd.ErrOrStderr(), `Warning: "brev deregister" is deprecated; use "brev leave" instead.`)
	fmt.Fprintln(cmd.ErrOrStderr(), `This command no longer removes SSH keys; run "brev disable-ssh" before leaving if you want to remove Brev-managed SSH access.`)
}
```

- [ ] **Step 2: Write leave state-machine tests**

Use a shared event recorder across registration, auth, node RPC, confirmation, sudo, NetBird, and registration deletion. Add:

- `TestRunLeave_RemainingGrantsWarnButDoNotBlock`: known access records print the retained-host-key warning and cancellation guidance.
- `TestRunLeave_ApproveSkipsConfirmationButNotWarnings`.
- `TestRunLeave_CancelStopsBeforeSudoAndMutation`.
- `TestRunLeave_OrderIsRemoveNodeUninstallDeleteRegistration`.
- `TestRunLeave_RemoveNodeFailureStopsLocalTeardown`.
- `TestRunLeave_CompleteNodeListWithoutRegisteredIDAllowsAuthoritativeRemoveRetry`.
- `TestRunLeave_ListPermissionDeniedStopsBeforeConfirmationAndMutation`.
- `TestRunLeave_RegisteredIDAbsentFromIncompleteListStopsBeforeMutation`.
- `TestRunLeave_OtherLookupFailureStopsBeforeConfirmationAndMutation`.
- `TestRunLeave_RemoveNodeNotFoundIsAccepted`.
- `TestRunLeave_NetBirdFailureReturnsErrorAndRetainsRegistration`.
- `TestRunLeave_RegistrationDeleteFailureReturnsErrorAndNoSuccess`.
- `TestRunLeave_NeverRevokesSSHOrEditsAuthorizedKeys`.

Assert warning text is written through the injected stderr writer, including with `--approve`. Assert no success string is present for every failure.

- [ ] **Step 3: Run the new tests and observe failure**

```bash
go test ./pkg/cmd/deregister ./pkg/cmd -run 'Test(NewCmdLeave|RunLeave|NewBrevCommand_BYON)' -count=1
```

Expected: FAIL because `leave` does not exist and deregistration still edits the invoking user's key file.

- [ ] **Step 4: Rename the public orchestration and remove SSH dependencies**

Keep the package name, but use:

```go
type LeaveStore interface {
	GetCurrentUser() (*entity.User, error)
	GetAccessToken() (string, error)
}

type netBirdUninstaller interface {
	Uninstall() error
}

type leaveDeps struct {
	platform          externalnode.PlatformChecker
	confirmer         terminal.Confirmer
	gater             sudo.Gater
	netbird           netBirdUninstaller
	nodeClients       externalnode.NodeClientFactory
	registrationStore register.RegistrationStore
}

func NewCmdLeave(t *terminal.Terminal, store LeaveStore) *cobra.Command
func runLeave(
	ctx context.Context,
	t *terminal.Terminal,
	warnings io.Writer,
	store LeaveStore,
	deps leaveDeps,
	skipConfirm bool,
) error
```

`defaultLeaveDeps` must use `register.LinuxPlatform{}`, `register.TerminalPrompter{}`, `sudo.Default`, `register.Netbird{}`, `register.DefaultNodeClientFactory{}`, and `register.NewFileRegistrationStore()`.

Delete `SSHKeyRemover`, `brevSSHKeyRemover`, `os/user`, and all direct key-removal output. Use canonical Cobra metadata and retain only `--approve`.

- [ ] **Step 5: Implement read-only preflight and warnings**

After Linux check, registration load, and authentication, call a private organization-scoped helper:

```go
func lookupJoinedNodeForLeave(
	ctx context.Context,
	client nodev1connect.ExternalNodeServiceClient,
	reg *register.DeviceRegistration,
) (node *nodev1.ExternalNode, missing bool, err error) {
	resp, err := client.ListNodes(ctx, connect.NewRequest(&nodev1.ListNodesRequest{
		OrganizationId: reg.OrgID,
	}))
	if err != nil {
		return nil, false, fmt.Errorf("list organization nodes: %w", err)
	}
	if resp == nil || resp.Msg == nil {
		return nil, false, fmt.Errorf("list organization nodes: empty response")
	}
	for _, candidate := range resp.Msg.GetItems() {
		if candidate != nil && candidate.GetExternalNodeId() == reg.ExternalNodeID {
			return candidate, false, nil
		}
	}
	if resp.Msg.GetNextPageToken() != "" {
		return nil, false, fmt.Errorf("registered node was not in the returned page and node listing is incomplete")
	}
	return nil, true, nil
}
```

Any ListNodes error, including `PermissionDenied`, or an incomplete response without the registered ID returns `inspect joined node before leaving` before confirmation, sudo, or mutation. When the complete list proves the node is absent, continue with no access snapshot and write that the backend node is already absent but tagged host keys may remain. This is the idempotent retry path; authoritative `RemoveNode` is still called defensively.

Always write this safety warning before confirmation:

```text
Leaving removes the Brev tunnel and may interrupt commands using Brev SSH. Run this locally or through out-of-band access.
```

When known non-nil grants remain, include total grant and distinct Linux-account counts and this action:

```text
Leaving stops Brev-routed SSH but does not remove keys from authorized_keys. Cancel and run "brev disable-ssh" first if you want Brev-managed SSH credentials removed.
```

Do not block on grants. Confirm through `terminal.Confirmer` unless `--approve` was supplied, then call the sudo gate only after confirmation so cancellation has no elevation side effect.

- [ ] **Step 6: Implement authoritative, retry-safe teardown**

Call operations strictly in this order:

```go
_, err := client.RemoveNode(ctx, connect.NewRequest(&nodev1.RemoveNodeRequest{
	ExternalNodeId: reg.ExternalNodeID,
}))
if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
	return fmt.Errorf("leave Brev network: remove node: %w", err)
}
if err := deps.netbird.Uninstall(); err != nil {
	return fmt.Errorf("leave Brev network: uninstall tunnel: %w", err)
}
if err := deps.registrationStore.Delete(); err != nil {
	return fmt.Errorf("leave Brev network: delete local registration: %w", err)
}
```

Do not delete registration when RemoveNode or Uninstall fails. Return the Delete error rather than printing a warning and false completion. Only after all three operations succeed, print `Left the Brev network.`

- [ ] **Step 7: Update root wiring and command-surface tests**

Replace `deregister.NewCmdDeregister` with `deregister.NewCmdLeave`. Extend the root test to prove `leave` exists once and `deregister` resolves to the exact same command pointer, not an independently registered command.

- [ ] **Step 8: Run tests and format**

```bash
gofmt -w pkg/cmd/deregister/deregister.go pkg/cmd/deregister/deregister_test.go pkg/cmd/cmd.go pkg/cmd/cmd_test.go
go test ./pkg/cmd/deregister ./pkg/cmd -run 'Test(NewCmdLeave|RunLeave|NewBrevCommand_BYON)' -count=1
```

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add pkg/cmd/deregister/deregister.go pkg/cmd/deregister/deregister_test.go pkg/cmd/cmd.go pkg/cmd/cmd_test.go
git commit -m "feat: separate network leave from SSH cleanup"
```

---

## Task 7: Document the Explicit Workflows and Verify the Complete Change

**Files:**

- Create: `docs/BYON.md`
- Modify: `README.md`
- Modify: `CHANGELOG.md`
- Modify: `.agents/skills/brev-cli/SKILL.md`
- Modify: `.agents/skills/brev-cli/reference/commands.md`
- Modify: all Go files touched in Tasks 1–6 if verification exposes formatting or lint defects

- [ ] **Step 1: Write the BYON guide**

Create `docs/BYON.md` with these explicit workflows:

```text
Join networking only:
  brev join

Optionally enable SSH for yourself, then grant collaborators individually:
  brev enable-ssh
  brev grant-ssh

Remove all Brev-managed SSH credentials, then retire membership:
  brev disable-ssh
  brev leave
```

Document that:

- `register` and `deregister` are deprecated aliases.
- `enable-ssh` requires prior join and reconnects an existing disconnected tunnel.
- `disable-ssh` is node-wide, leaves ports allocated, does not stop sshd, and does not terminate active SSH sessions.
- `leave` removes the VPN route/backend node but does not remove physical host keys.
- `leave` currently preserves the old behavior of uninstalling NetBird even if the user installed it before Brev; tracking install ownership is a follow-up.

Link this guide from `README.md` directly below the existing NVIDIA/Brev documentation link.

- [ ] **Step 2: Update shipped CLI-skill guidance**

Add a “BYON Network and SSH Commands” section to `.agents/skills/brev-cli/reference/commands.md` before configuration commands. Document `join`, `register`, `enable-ssh`, `grant-ssh`, `revoke-ssh`, `disable-ssh`, `leave`, and `deregister`, including the two canonical multi-command workflows.

Update `.agents/skills/brev-cli/SKILL.md` so no instruction implies join automatically enables SSH. Keep cloud-instance commands outside this change untouched.

- [ ] **Step 3: Add release notes**

Under `CHANGELOG.md` Unreleased, record:

- Added: `join`, `leave`, and node-wide `disable-ssh`.
- Changed: `join` no longer enables SSH; `enable-ssh` requires and reconnects existing membership.
- Deprecated: `register` and `deregister` remain aliases and warn on stderr.
- Migration: scripts using `--ssh-port` must run `brev join` followed by `brev enable-ssh`.

- [ ] **Step 4: Search for stale user-facing terminology**

Run:

```bash
rg -n "brev (register|deregister)|--ssh-port|Registering your device|Deregistering your device" README.md CHANGELOG.md docs .agents/skills pkg/cmd
```

Expected: remaining `register`/`deregister` references are alias documentation, deprecation tests, internal persistence/backend terminology, or deliberate compatibility errors. No public example presents either alias as canonical, and no join help presents `--ssh-port` as supported.

- [ ] **Step 5: Verify focused behavior**

Run:

```bash
go test ./pkg/cmd/register ./pkg/cmd/enablessh ./pkg/cmd/disablessh ./pkg/cmd/deregister ./pkg/cmd -count=1
go test -race ./pkg/cmd/disablessh -count=1
```

Expected: PASS. If the restricted sandbox denies an existing `httptest` listener, rerun outside the sandbox and record that environment distinction.

- [ ] **Step 6: Format and lint the touched command packages**

Run:

```bash
gofmt -w main.go pkg/cmd/register/providers.go pkg/cmd/register/providers_test.go pkg/cmd/register/register.go pkg/cmd/register/register_test.go pkg/cmd/register/device_registration_store.go pkg/cmd/register/device_registration_store_test.go pkg/cmd/register/node.go pkg/cmd/register/node_test.go pkg/cmd/register/sshkeys.go pkg/cmd/register/sshkeys_test.go pkg/cmd/enablessh/enablessh.go pkg/cmd/enablessh/enablessh_test.go pkg/cmd/disablessh/disablessh.go pkg/cmd/disablessh/disablessh_test.go pkg/cmd/disablessh/localkeys.go pkg/cmd/disablessh/localkeys_test.go pkg/cmd/disablessh/localkeys_linux.go pkg/cmd/disablessh/localkeys_linux_test.go pkg/cmd/disablessh/localkeys_unsupported.go pkg/cmd/deregister/deregister.go pkg/cmd/deregister/deregister_test.go pkg/cmd/cmd.go pkg/cmd/cmd_test.go
golangci-lint run ./pkg/cmd/... ./pkg/sudo/...
```

Expected: PASS. Fix only defects caused by this branch; do not absorb unrelated lint churn.

- [ ] **Step 7: Verify cross-platform compilation and Linux-only tests**

On the current Darwin host:

```bash
GOOS=linux GOARCH=amd64 go test -c -o /tmp/brev-disablessh-linux.test ./pkg/cmd/disablessh
GOOS=linux GOARCH=amd64 go build -o /tmp/brev-cli-linux .
```

On a Linux runner or Linux development host:

```bash
go test -race ./pkg/cmd/disablessh -run 'TestSystemAuthorizedKeysCleaner' -count=1
```

Expected: Linux package and root-command cross-builds PASS and descriptor/symlink tests PASS on Linux.

- [ ] **Step 8: Inspect the rendered command surface**

Run:

```bash
go run . --help
go run . join --help
go run . leave --help
go run . enable-ssh --help
go run . disable-ssh --help
```

Expected: root help shows canonical `join`, `leave`, `enable-ssh`, and `disable-ssh`; aliases are not independent top-level entries; canonical help emits no warning; `--ssh-port` is hidden.

- [ ] **Step 9: Attempt repository-wide verification and classify baseline failures**

Run:

```bash
go test ./pkg/... -count=1
go test ./... -count=1
```

Expected: all affected command packages pass. If the known macOS-only baseline failures recur in Linux e2e setup, JetBrains Gateway detection, or WSL store tests, capture their exact package/test names and confirm no affected BYON package failed.

- [ ] **Step 10: Review scope and diff**

Run:

```bash
git status --short
git diff --check
git diff --stat 2954f39e..HEAD
git log --oneline 2954f39e..HEAD
```

Confirm there is no backend/proto change, no port closure, no sshd stop, no implicit AddNode from SSH commands, no SSH cleanup from leave, no node removal from disable, and no unrelated user work.

- [ ] **Step 11: Commit documentation and final verification fixes**

```bash
git add README.md CHANGELOG.md docs/BYON.md .agents/skills/brev-cli/SKILL.md .agents/skills/brev-cli/reference/commands.md
git commit -m "docs: explain explicit BYON network and SSH flows"
```

If verification required a source/test correction after Task 6, include only that tightly related correction in this commit and describe it in the commit body.

---
