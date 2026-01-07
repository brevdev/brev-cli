# brevd service artifacts

This directory contains minimal assets for running the DevPlane `brevd` daemon as a background service on Linux hosts.

## Files

- `systemd/brevd.service` - systemd unit referencing `/usr/local/bin/brevd` and `/etc/default/brevd`.
- `install.sh` - helper script to copy the agent binary, drop configuration stubs, and enable the service.

## Building the `brevd` binary

Use `go build` from the repo root. For Linux target nodes (including OrbStack), cross-compile for the desired architecture:

```
GOOS=linux GOARCH=amd64 go build -o brevd ./cmd/brevd
```

Adjust `GOARCH` (e.g., `arm64`) to match the target hardware. The resulting `brevd` binary can be copied directly into the VM or passed to `install.sh`.

## Environment file (`/etc/default/brevd`)

The unit loads configuration from `/etc/default/brevd`. Set the same environment variables used when running the binary manually:

- `BREV_AGENT_BREVCLOUD_URL`
- `BREV_AGENT_REGISTRATION_TOKEN`
- `BREV_AGENT_DISPLAY_NAME`
- `BREV_AGENT_CLOUD_NAME`
- `BREV_AGENT_CLOUD_CRED_ID`
- `BREV_AGENT_STATE_DIR`
- `BREV_AGENT_DEVICE_TOKEN_PATH`
- `BREV_AGENT_HEARTBEAT_INTERVAL`
- `BREV_AGENT_ENABLE_TUNNEL`
- `BREV_AGENT_TUNNEL_SSH_PORT`

Unset values fall back to the agent defaults (see `internal/agent/config`).

> **Important:** `BREV_AGENT_BREVCLOUD_URL` must target the agent ingress exposed by the control plane. The public server mounts the Connect handler under `/agent/v1` (see `internal/cmd/devplane/public_server.go`), so your URL should look like `https://<control-plane-domain>/agent/v1`. The agent will append the Connect RPC paths on top of that base.

## Manual install

1. Build or download the `brevd` binary.
2. Copy it to `/usr/local/bin/brevd` and ensure it is executable.
3. Copy `systemd/brevd.service` to `/etc/systemd/system/brevd.service`.
4. Create `/etc/default/brevd`, populate the environment variables above, and protect it (`chmod 600`).
5. Reload systemd: `sudo systemctl daemon-reload`.
6. Enable and start: `sudo systemctl enable --now brevd`.

The `install.sh` script automates these steps and can be re-run safely to update the binary or unit.

## Monitoring and logs

- Inspect current status and last few log lines:

  ```
  sudo systemctl status brevd
  ```

- Stream live logs from the agent:

  ```
  sudo journalctl -u brevd -f
  ```

- Show logs for a specific boot/session or time window (example: last hour):

  ```
  sudo journalctl -u brevd --since "1 hour ago"
  ```

- If you installed using `install.sh`, the environment file resides at `/etc/default/brevd`. You can check the active configuration with:

  ```
  sudo cat /etc/default/brevd
  ```

These commands work the same on OrbStack VMs and physical Linux hosts.
