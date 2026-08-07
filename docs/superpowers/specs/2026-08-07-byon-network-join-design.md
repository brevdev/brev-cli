# BYON Network Join Command Design

## Summary

Replace `brev register` as the canonical BYON onboarding command with
`brev join`. The command joins the current Linux machine to the selected
organization's Brev network by installing and connecting NetBird, creating the
external node, and persisting its local identity.

SSH access is not part of joining the network. Users explicitly run
`brev enable-ssh` to enable SSH for themselves and `brev grant-ssh` to share
SSH access with another organization member.

`brev register` remains a deprecated Cobra alias for `brev join`. This change
does not set a removal release for the alias.

## Goals

- Make BYON network membership the sole purpose of the onboarding command.
- Use `join` as the user-facing verb for durable membership in an
  organization's Brev network.
- Preserve existing `brev register` automation that does not request SSH.
- Make the removal of implicit SSH enablement explicit and actionable.
- Keep `enable-ssh` and `grant-ssh` as separate, intentional access-control
  operations.

## Non-goals

- Renaming `brev deregister` or introducing `brev leave`.
- Changing `brev enable-ssh`, `brev grant-ssh`, `brev revoke-ssh`, or SSH
  authorization semantics.
- Moving or renaming the `pkg/cmd/register` package, persisted
  `DeviceRegistration` model, registration file, backend RPCs, or proto fields.
- Changing how an organization or its current default Brev network is selected.
- Refactoring shared external-node helpers unrelated to the command boundary.

## Command Surface

The existing command constructor becomes `NewCmdJoin`. Its Cobra metadata is:

```go
Use:     "join",
Aliases: []string{"register"},
```

The public flags are:

- `--name`, `-n`: device name; required with `--org` in non-interactive mode.
- `--org`, `-o`: organization name; required with `--name` in
  non-interactive mode.
- `--approve`: skip the confirmation prompt.

The `--ssh-port`, `-p` flag is removed from the public command contract. A
hidden compatibility flag recognizes both forms and returns an error before
any sudo, authentication, installation, RPC, or local persistence side effect:

```text
--ssh-port is no longer supported by brev join or brev register; run brev join,
then run brev enable-ssh on the joined machine
```

This provides a migration message for existing scripts without retaining SSH
behavior in the join flow.

When Cobra reports that the command was invoked as `register`, the command
writes a warning to stderr and continues through the same join handler:

```text
Warning: "brev register" is deprecated; use "brev join" instead.
This command no longer enables SSH; run "brev enable-ssh" separately.
```

Warnings go to stderr so normal stdout remains usable by scripts. The alias is
accepted in both interactive and non-interactive modes.

## Join Flow

`runJoin` owns orchestration and retains the existing network-membership
sequence:

1. Verify Linux compatibility and obtain sudo authorization.
2. Resolve the authenticated Brev user.
3. If a local device registration exists, reconcile and report NetBird
   connectivity without changing SSH state.
4. Resolve the device name and target organization from prompts or flags.
5. Confirm that the operation will install the Brev tunnel, collect a hardware
   profile, add the node to Brev, persist its identity, and connect it to the
   organization's Brev network.
6. Install NetBird, collect the hardware profile, call `AddNode`, save the
   `DeviceRegistration`, and execute the backend-provided NetBird setup command.
7. Report network membership success and state that SSH access was not enabled.

The success output ends with an explicit optional next step:

```text
SSH access was not enabled. To enable it for your user, run: brev enable-ssh
```

The join flow never prompts for an SSH port, looks up the current Linux user for
an SSH grant, opens a port, installs an authorized key, or calls an SSH-access
RPC.

## Code Boundaries

- `pkg/cmd/cmd.go` registers `register.NewCmdJoin(...)` once. Cobra resolves
  both `join` and its `register` alias to that command.
- The existing `pkg/cmd/register` directory remains because it contains the
  durable device-registration model and shared external-node helpers used by
  other commands.
- Command-specific names in `register.go` and its tests change from register to
  join where they describe the user-facing flow, including `NewCmdJoin`,
  `joinOpts`, `runJoin`, and `runJoinSteps`.
- The SSH orchestration currently appended to `runRegister` is removed. Shared
  SSH helpers still used by `enable-ssh`, `grant-ssh`, `revoke-ssh`, or
  `deregister` remain available.
- User-facing guidance that currently says to run `brev register` first changes
  to `brev join`.
- Internal/backend messages may continue to use registration terminology when
  they describe `AddNode` or persisted registration state rather than the CLI
  action.

## Compatibility and Error Handling

- `brev register`, `brev register --name ... --org ...`, and
  `brev register --approve` continue to invoke the join flow.
- The alias warning is emitted on every non-help execution of `register`; it is
  not emitted for canonical `join` invocations.
- `register --ssh-port`, `register -p`, `join --ssh-port`, and `join -p` fail
  before side effects with the targeted migration error.
- The legacy SSH flag is never ignored and never translated into an implicit
  `enable-ssh` operation.
- Existing idempotent behavior for an already joined machine remains: check the
  backend node status, ensure the local NetBird service is connected, and tell
  the user to run `brev deregister` before joining differently.
- Failures retain operation context and preserve the existing boundary between
  fatal membership failures and warning-only tunnel status checks.

## Testing

Tests will cover:

- Cobra exposes `join` as the canonical command and resolves `register` as its
  alias.
- Canonical and alias invocations reach the same network-membership handler.
- `register` writes the deprecation/SSH-separation warning to stderr, while
  `join` does not.
- Interactive join prompts only for device name, organization, and membership
  confirmation; it never asks about SSH.
- Non-interactive join still requires both `--name` and `--org`.
- All four legacy SSH flag forms fail with the migration error before any
  membership or SSH dependency is called.
- Successful join installs/connects NetBird, creates and persists the node, and
  makes no SSH port or grant call.
- Existing already-joined connectivity reconciliation remains intact.
- Existing focused tests for `enable-ssh`, `grant-ssh`, `revoke-ssh`, and
  `deregister` continue to pass unchanged unless user-facing `join` guidance is
  updated.

Verification will run formatting and the focused command-package tests. The
repository-wide suite will also be attempted, but its current macOS baseline
has unrelated Linux e2e, JetBrains Gateway, and WSL-specific failures; those
failures will be reported separately rather than attributed to this change.

## Documentation and Release Notes

- Cobra help presents `join` as the canonical command and identifies
  `register` as an alias.
- Examples use `brev join --name <node> --org <organization>` and show
  `brev enable-ssh` as a separate optional command.
- Release notes call out the behavior change: `register` no longer offers or
  enables SSH, and `--ssh-port` now returns migration guidance.
- Removing the `register` alias requires a future explicit compatibility
  decision; this design does not schedule its removal.
