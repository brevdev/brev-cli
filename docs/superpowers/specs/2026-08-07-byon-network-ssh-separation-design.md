# BYON Network and SSH Command Separation Design

## Summary

Separate BYON network membership from Brev-managed SSH access through four
explicit command boundaries:

| Capability | Canonical command | Compatibility alias |
| --- | --- | --- |
| Join the organization's Brev network | `brev join` | `brev register` |
| Leave the organization's Brev network | `brev leave` | `brev deregister` |
| Enable Brev-managed SSH for the current Brev/Linux user | `brev enable-ssh` | None |
| Revoke all backend-tracked Brev SSH grants on the node | `brev disable-ssh` | None |

`join` and `leave` own only durable Brev/NetBird membership. `enable-ssh` and
`disable-ssh` own Brev-managed SSH authorization records. `grant-ssh` and
`revoke-ssh` remain the commands for individual collaborator grants.

`register` and `deregister` remain deprecated Cobra aliases with no scheduled
removal release. Their handlers and behavior are the same as their canonical
commands, including the new separation from SSH.

## Goals

- Make joining an organization's Brev network the sole purpose of the onboarding
  command.
- Use the durable membership pair `join` and `leave` rather than registration
  terminology.
- Require an established and connected Brev tunnel before SSH can be enabled.
- Make node-wide SSH disablement an explicit operation independent of network
  membership.
- Preserve individual collaborator management through `grant-ssh` and
  `revoke-ssh`.
- Preserve compatible automation through deprecated `register` and
  `deregister` aliases, with actionable migration output.
- Make partial backend revocation failures visible and safely retryable.
- Let `disable-ssh` make best-effort progress across every tracked grant while
  revoking the invoking Brev user's own access last.

## Non-goals

- Stopping or reconfiguring the host's SSH daemon.
- Closing Brev port allocations from `disable-ssh`. Existing ports may have been
  reused for purposes other than SSH, and the current model does not record
  ownership.
- Adding port-ownership metadata, changing backend RPC or proto shapes, or
  introducing a new backend bulk-revocation RPC.
- Changing the authorization semantics of `grant-ssh` or `revoke-ssh`.
- Renaming the internal `pkg/cmd/register` package, the persisted
  `DeviceRegistration` model, registration file, or backend node terminology.
- Changing organization or default-network selection.
- Tracking whether Brev installed NetBird. `leave` preserves today's NetBird
  uninstall behavior; protecting a pre-existing user-managed NetBird
  installation is a separate follow-up.
- Editing host `authorized_keys` files from `disable-ssh`, including removing
  Brev-tagged, orphaned, or otherwise static local keys.
- Forcibly terminating already-established SSH sessions. Revoking backend
  authorization records does not kill active sessions.
- Adding a sudo gate, privileged helper, or same-binary re-execution path for
  `disable-ssh`.

## Naming Rationale

The names follow established networking conventions:

- ZeroTier uses `join` and `leave` for durable network membership.
- Tailscale uses `down` for a reversible disconnect and separates SSH enablement
  from tailnet membership.
- NetBird also treats `down` as a temporary idle state and manages peer
  membership separately.

For Brev, `leave` therefore means authoritative membership removal rather than a
temporary tunnel stop. `logout` is avoided because it conventionally describes
human or device authentication state. `peer` is avoided as the command verb
because it describes the resulting network object rather than the user action.

References:

- [ZeroTier CLI](https://docs.zerotier.com/cli/)
- [Tailscale CLI](https://tailscale.com/docs/reference/tailscale-cli)
- [Tailscale SSH](https://tailscale.com/docs/features/tailscale-ssh)
- [NetBird CLI](https://docs.netbird.io/get-started/cli)
- [NetBird SSH](https://docs.netbird.io/manage/peers/ssh)

## Command Model

The intended user workflow is:

```text
brev join
brev enable-ssh
brev grant-ssh
```

Explicit tracked-grant revocation and membership retirement are intentionally
two operations:

```text
brev disable-ssh
brev leave
```

Running `leave` without `disable-ssh` is allowed. Brev-routed SSH stops because
the node leaves the network. Neither operation removes local keys from
`authorized_keys`; any such keys may still work through another network path.

### Join and Register Alias

The existing command constructor becomes `NewCmdJoin` with canonical Cobra
metadata:

```go
Use:     "join",
Aliases: []string{"register"},
Args:    cobra.NoArgs,
```

Public flags remain:

- `--name`, `-n`: device name; required with `--org` in non-interactive mode.
- `--org`, `-o`: organization name; required with `--name` in non-interactive
  mode.
- `--approve`: skip the confirmation prompt.

The legacy `--ssh-port`, `-p` flag is removed from the public contract. A hidden
compatibility flag recognizes both forms and returns this error before sudo,
authentication, installation, RPC, or persistence side effects:

```text
--ssh-port is no longer supported by brev join or brev register; run brev join,
then run brev enable-ssh on the joined machine
```

When `cmd.CalledAs()` reports `register`, the command writes this warning to
stderr and continues through the join handler:

```text
Warning: "brev register" is deprecated; use "brev join" instead.
This command no longer enables SSH; run "brev enable-ssh" separately.
```

The warning is emitted for executions, not help rendering. Stdout remains
available for normal command output and scripts.

### Join Flow

`runJoin` retains only network-membership orchestration:

1. Verify Linux compatibility and obtain sudo authorization.
2. Resolve the authenticated Brev user.
3. If local registration already exists, reconcile the backend node and local
   NetBird connection without changing SSH state.
4. Resolve device name and organization from prompts or flags.
5. Confirm that the operation installs the Brev tunnel, collects a hardware
   profile, creates the external node, persists identity, and joins the
   organization's Brev network.
6. Install NetBird, collect hardware, call `AddNode`, save
   `DeviceRegistration`, and run the backend-provided `netbird up` command.
7. Report network membership success and the optional next step:

```text
SSH access was not enabled. To enable it for your user, run: brev enable-ssh
```

The flow never prompts for an SSH port, looks up a Linux account for a grant,
opens a port, installs an authorized key, or calls an SSH-access RPC.

### Enable SSH

`brev enable-ssh` remains the self-enablement command for the current Brev user
and current Linux account. It has a hard network-membership precondition:

1. Verify Linux compatibility.
2. Load local `DeviceRegistration`. If none exists, fail with targeted guidance:

   ```text
   This machine has not joined a Brev network; run "brev join" first.
   ```

3. Authenticate and verify that the registered backend node still exists.
4. Ensure the existing NetBird service is running and connected. A temporarily
   disconnected tunnel is started or reconnected automatically.
5. Wait for bounded, positive connectivity confirmation. A status error or
   unconfirmed connection is a failure rather than assumed success.
6. Only then select or open a port, install the current user's tagged key, and
   create the current user's SSH access record.

`enable-ssh` may reconnect existing membership, but it never calls `AddNode`,
selects an organization, writes a new registration, or otherwise performs an
implicit join. A failed membership, node, or tunnel check occurs before any SSH
port, key, or access-record mutation.

### Grant and Revoke SSH

`brev grant-ssh` and `brev revoke-ssh` retain their existing roles:

- `grant-ssh` creates one exact collaborator access tuple for a node, port, and
  Linux account.
- `revoke-ssh` removes one exact access tuple.

They do not become aliases or modes of `enable-ssh` or `disable-ssh`.

### Disable SSH

Add a canonical top-level command:

```go
Use:  "disable-ssh",
Args: cobra.NoArgs,
```

It accepts `--approve` to skip confirmation. It operates only on the locally
registered node and means "revoke every backend-tracked Brev SSH grant on this
node." It does not mean "stop sshd" or "clean authorized_keys."

The flow is:

1. Load local registration; if absent, direct the user to `brev join`.
2. Authenticate the invoking Brev user, fetch the registered backend node, and
   take a fresh snapshot of every remaining `SSHAccess` tuple.
3. If the snapshot is empty, report that there are no grants to revoke and
   return successfully without prompting.
4. Show a node-wide confirmation with the active grant count. State that active
   sessions are not forcibly terminated. `--approve` skips this prompt but not
   the active-session warning.
5. Stable-partition the snapshot so grants for collaborators remain first and
   every grant belonging to the invoking Brev user is last. Preserve snapshot
   order within each group.
6. Call `RevokeNodeSSHAccess` sequentially for every tuple. Continue after
   individual failures, including through the invoking user's final record, and
   aggregate contextual errors with user, Linux account, and port details.
7. Return nonzero if authentication, lookup, or any revocation is incomplete.
   Report overall success only after every record in the fresh snapshot has
   been revoked.

A no-access state is successful, making the command safely repeatable. Each
retry fetches a fresh backend access snapshot, skips records already removed,
and attempts only work that remains. Membership and registration remain intact
after every outcome so revocation can be retried.

`disable-ssh` does not remove the backend node, stop or uninstall NetBird, delete
registration, stop sshd, close ports, terminate active sessions, or inspect or
modify `authorized_keys`. Ports remain because the current API cannot
distinguish ports created for SSH from pre-existing ports selected by the SSH
flow.

### Leave and Deregister Alias

The existing deregistration constructor becomes `NewCmdLeave`:

```go
Use:     "leave",
Aliases: []string{"deregister"},
Args:    cobra.NoArgs,
```

It retains `--approve`. When invoked as `deregister`, it writes this warning to
stderr:

```text
Warning: "brev deregister" is deprecated; use "brev leave" instead.
This command does not revoke SSH access grants; run "brev disable-ssh" before
leaving if you want to revoke them.
```

The leave flow owns only membership teardown:

1. Verify Linux compatibility, load local registration, and authenticate.
2. Fetch the registered node when it still exists and inspect its SSH access
   records. Backend not-found is the idempotent retry case; any other lookup
   error stops before mutation.
3. Always warn that removing the Brev tunnel may interrupt a command running
   through Brev SSH. Recommend running locally or through out-of-band access.
4. If SSH access records remain, explain that Brev-routed SSH will stop and that
   `leave` will not run the per-grant revocation flow. Tell the user to cancel
   and run `brev disable-ssh` first if explicit best-effort revocation is
   desired.
5. Confirm unless `--approve` was supplied. Warnings still print with
   `--approve`.
6. Obtain sudo authorization before network removal so local teardown will not
   require a new password prompt after connectivity is lost.
7. Call `RemoveNode`. Backend removal authoritatively removes network membership,
   deallocates node ports, and deletes SSH access metadata, but it does not
   remove physical host keys.
8. Uninstall NetBird using the existing Brev tunnel teardown behavior.
9. Delete local registration last.
10. Report completion only after membership and local teardown succeed.

`leave` never calls `RevokeNodeSSHAccess` and never edits
`authorized_keys`. This preserves the command boundary even when grants exist.

An already-removed backend node is treated as success during a retry. If backend
removal succeeds but NetBird uninstall or registration deletion fails, the
registration is retained when possible so `leave` can resume. Local failures
return nonzero rather than producing a false successful completion.

## Code Boundaries

- `pkg/cmd/cmd.go` registers `register.NewCmdJoin(...)`,
  `deregister.NewCmdLeave(...)`, and the new
  `disablessh.NewCmdDisableSSH(...)` exactly once. Cobra resolves aliases.
- `pkg/cmd/register` remains the home of the durable registration model and
  shared external-node helpers. User-facing orchestration names change to
  `joinOpts`, `runJoin`, and `runJoinSteps`.
- The SSH tail currently appended to registration is removed. Existing SSH
  helpers remain available to SSH commands.
- `pkg/cmd/deregister` retains its internal package name but owns only leave
  orchestration. Its direct authorized-key removal dependency is removed.
- `pkg/cmd/disablessh` is a focused new package with injected dependencies for
  registration, current-user lookup, node lookup, confirmation, and grant
  revocation.
- `disable-ssh` has no local key-cleanup abstraction, sudo gate, hidden helper
  argument, same-binary privileged re-execution, or special dispatch in
  `main.go`. The shared `pkg/sudo` behavior remains for commands that still
  require it.
- Tunnel management gains a strict connected operation suitable for SSH
  preconditions. It can start the service and run `netbird up` for existing
  membership, but returns an error unless connectivity is positively confirmed.
- `enable-ssh` always uses that strict tunnel operation. `disable-ssh`, like
  `revoke-ssh`, calls the public revocation API without managing the local
  tunnel.
- User-facing guidance throughout the CLI changes from `brev register` to
  `brev join`. Internal and backend registration terminology remains where it
  describes persisted state or `AddNode`.

## Compatibility

- `brev register`, including its `--name`, `--org`, and `--approve` flags,
  continues through the join flow.
- `brev deregister --approve` continues through the leave flow.
- Both aliases warn on stderr with their canonical replacement.
- The aliases do not retain legacy SSH side effects.
- All forms of the old SSH flag, `register --ssh-port`, `register -p`,
  `join --ssh-port`, and `join -p`, fail before side effects with migration
  guidance.
- `deregister` retains the same membership-only behavior as `leave`. Its warning
  tells callers to run `disable-ssh` first when they want explicit per-grant
  backend revocation.
- Removing either alias requires a future explicit compatibility decision.

## Error Handling and Recovery

- Membership validation and strict tunnel connectivity precede every
  `enable-ssh` mutation.
- `disable-ssh` attempts every backend revocation in a fresh snapshot, reports
  each failed tuple with user, Linux account, and port context, and joins the
  failures into one final error.
- Collaborator records are attempted first and the invoking Brev user's own
  records are attempted last, even after earlier failures.
- A partial revocation returns nonzero. A retry fetches a fresh snapshot and
  attempts only records that remain.
- `leave` deletes registration last and treats backend not-found as an
  idempotent retry condition.
- Neither teardown command prints success after an incomplete operation.
- Errors wrap the failed operation while preserving the underlying error for
  callers and tests.
- Deprecation and safety warnings use stderr; ordinary status and success output
  use the terminal's normal output path.

## Testing

### Command Surface

Tests verify:

- Cobra exposes `join` and `leave` as canonical commands.
- `register` and `deregister` resolve as aliases to the same handlers.
- Canonical invocations do not warn; aliases warn on stderr.
- All four affected commands reject positional arguments.
- Help and examples use canonical names.

### Join

Tests verify:

- Interactive join prompts only for device name, organization, and membership
  confirmation.
- Non-interactive join still requires both `--name` and `--org`.
- Legacy SSH flags fail before membership or SSH dependencies are called.
- Successful join installs and connects NetBird, creates and persists the node,
  and makes no port or SSH-access call.
- Existing joined-node reconciliation remains intact.

### Enable SSH

Tests verify:

- Missing registration returns targeted `brev join` guidance.
- A missing backend node fails without joining or mutating SSH.
- A connected tunnel proceeds normally.
- A disconnected existing tunnel reconnects and then proceeds.
- Failed or unconfirmed reconnection causes no port, key, or SSH-access side
  effect.
- The command never calls `AddNode`.

### Disable SSH

Tests verify:

- Confirmation describes node-wide scope and can be bypassed with `--approve`.
- `--approve` skips confirmation without suppressing the active-session warning.
- No disable flow invokes a key-cleanup dependency, sudo gate, subprocess
  runner, hidden helper mode, or other automatic elevation path.
- Every active access tuple is revoked exactly once.
- Collaborator tuples preserve their snapshot order and are attempted before
  every tuple owned by the invoking Brev user.
- All tuples are attempted even when one fails, and errors are aggregated.
- Aggregated failures retain their underlying causes, include `failed to revoke
  one or more SSH access grants`, and do not print overall success.
- A no-access run succeeds without prompting or making revocation calls.
- A retry does not re-revoke records absent from the fresh backend snapshot.
- No `authorized_keys` file is inspected or modified.
- No node removal, NetBird teardown, registration deletion, sshd operation, or
  port close occurs, and active sessions are not forcibly terminated.

### Leave

Tests verify:

- Remaining grants produce the tracked-access warning but do not block leave.
- `--approve` skips confirmation but not warnings.
- No SSH revoke or authorized-key dependency is called.
- Ordering is backend removal, NetBird uninstall, then registration deletion.
- Backend removal failure stops local teardown.
- Backend not-found is accepted during retry.
- Other backend lookup failures stop before confirmation or mutation.
- NetBird or registration cleanup failure returns nonzero and preserves
  recoverable state where possible.
- Success is printed only after complete membership teardown.

### Verification Commands

Implementation verification will include `gofmt` on touched Go files, focused
command-package tests, and `golangci-lint` when configured and practical. The
repository-wide suite will also be attempted.

The current macOS baseline has unrelated failures in Linux e2e setup, JetBrains
Gateway detection, and WSL-specific store tests. Those failures will be reported
separately and will not be attributed to this change.

## Documentation and Release Notes

- CLI help and examples use `join` and `leave` as the primary verbs.
- Onboarding documents show `enable-ssh` as an explicit post-join choice.
- Offboarding documents show `disable-ssh` followed by `leave` for explicit
  tracked-grant revocation followed by membership removal.
- `disable-ssh` documentation explains best-effort backend revocation, its
  nonzero partial-failure result, current-user-last ordering, and that local
  `authorized_keys` files are outside its scope.
- Documentation states that `leave` alone makes the node unreachable over the
  Brev network but does not remove host keys.
- Release notes call out both deprecated aliases and the SSH behavior change.
- The existing risk that `leave` may uninstall a pre-existing user-managed
  NetBird installation is documented as a follow-up, not silently changed in
  this implementation.
