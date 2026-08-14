# Bring Your Own Node (BYON)

Brev separates network membership from Brev-managed SSH credentials on a
machine you bring to your organization.

## Join networking only

```bash
brev join
```

`brev join` establishes this machine's Brev/NetBird organization membership.
It does not enable SSH or create an SSH port. Use `brev register` only for
compatibility with existing automation: it is a deprecated alias for `join` and
prints a warning when executed.

Scripts that used `--ssh-port` must migrate to two commands:

```bash
brev join
brev enable-ssh
```

## Enable and grant SSH

After joining, enable Brev-managed SSH for the invoking Brev user:

```bash
brev enable-ssh
```

`enable-ssh` requires a prior join. It confirms the existing Brev tunnel and
can reconnect it when it is disconnected; it never joins a network or creates
membership. It then enables the invoking user's access on the joined node.

Grant and revoke collaborator access separately:

```bash
brev grant-ssh
brev revoke-ssh
```

These commands manage individual collaborator access tuples. They are not part
of `join`, `enable-ssh`, or the node-wide revocation command.

## Retire Brev access and membership

To explicitly revoke Brev-tracked SSH grants before leaving the network:

```bash
brev disable-ssh
brev leave
```

`disable-ssh` is node-wide. It makes a best-effort attempt to revoke every
backend-tracked Brev SSH access tuple, continuing after individual failures and
revoking the invoking Brev user's own access last. It returns an error if any
revocation fails so the remaining records can be retried. It does not inspect or
modify local `authorized_keys` files. It leaves existing ports allocated, leaves
`sshd` running, does not forcibly terminate active SSH sessions, and does not
change network membership.

`leave` removes the backend node, Brev VPN route, and local registration. It
does not run the per-grant `disable-ssh` flow or modify local `authorized_keys`
files. Run `disable-ssh` first when explicit best-effort revocation of tracked
grants is desired. `brev deregister` is a deprecated alias for `leave` and warns
when executed.

`leave` preserves the existing behavior of uninstalling NetBird even when
NetBird was installed before Brev. Tracking whether Brev owns that installation
is a follow-up improvement, so use `leave` only when that removal is intended.
