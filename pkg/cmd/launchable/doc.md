# `brev launchable create`

Create a Brev launchable from the CLI using either:

- VM mode
- Docker Compose mode

This command is intended to mirror the current launchable create flow in the console for the supported scope in the CLI.

## Synopsis

```bash
brev launchable create [name] [flags]
```

The launchable name can be passed either:

- as the positional argument: `brev launchable create my-launchable ...`
- or with `--name`: `brev launchable create --name my-launchable ...`

## What This Command Creates

A launchable is a reusable environment definition that captures:

- the compute shape
- the build mode
- the optional preload file/repo URL
- the network access model
- the launchable visibility

When a launchable is created, the CLI sends a request containing:

- `name`
- `description`
- `viewAccess`
- `createWorkspaceRequest`
- `buildRequest`
- optional `file`

## Supported Build Modes

### `vm`

VM mode creates a launchable backed by a VM build.

Use VM mode when you want:

- host-based Jupyter installation
- a lifecycle/setup script
- simple launchables that do not require Docker Compose

### `compose`

Compose mode creates a launchable backed by Docker Compose.

Use Compose mode when you want:

- a multi-service environment
- a compose file hosted remotely
- a local compose file
- inline compose YAML

The CLI validates the compose definition before sending the create request.

## Current CLI Scope

The current CLI implementation supports:

- `vm`
- `compose`

The current CLI implementation does not support:

- featured container / verb-based launchables
- raw custom container build mode
- private registry flags
- API-only location and image overrides

## Required Inputs

At minimum, you must provide:

- a launchable name
- `--instance-type`
- `--mode`

For `compose` mode you must also provide exactly one compose source:

- `--compose-url`
- `--compose-file`
- `--compose-yaml`

## Full Flag Reference

### Core Flags

#### `--name`, `-n`

Explicit launchable name.

```bash
brev launchable create --name my-launchable --mode vm --instance-type n2-standard-4
```

#### Positional name

Equivalent to `--name`.

```bash
brev launchable create my-launchable --mode vm --instance-type n2-standard-4
```

#### `--description`

Optional human-readable description stored on the launchable.

```bash
--description "Stable VM launchable for onboarding"
```

#### `--view-access`

Controls who can access the launchable.

Allowed values:

- `public`
- `organization`
- `published`

Default:

```bash
--view-access organization
```

Examples:

```bash
--view-access organization
--view-access published
```

#### `--mode`, `-m`

Build mode.

Allowed values:

- `vm`
- `compose`

Default:

```bash
--mode vm
```

#### `--instance-type`, `-t`

Required. The exact instance type to attach to the launchable.

Examples:

```bash
--instance-type n2-standard-4
--instance-type g5.xlarge
```

#### `--storage`

Optional root disk size in GiB.

Behavior:

- If the selected instance type supports flexible storage, this value is sent to the API.
- If the selected instance type has fixed storage, the CLI rejects `--storage`.
- If flexible storage is supported and `--storage` is omitted, the CLI defaults to `256` GiB, clamped to the instance type's min/max storage range.

Examples:

```bash
--storage 256
--storage 512
```

### File Source Flag

#### `--file-url`

Optional public file or repo URL to preload into the launchable.

Typical uses:

- GitHub repo URL
- GitLab repo URL
- notebook URL
- markdown URL

Normalization:

- GitHub `/blob/` URLs are converted to `/raw/`
- GitLab `/-/blob/` URLs are converted to `/-/raw/`

Rejected URL patterns:

- trailing slash URLs
- GitHub directory tree URLs
- GitLab directory tree URLs

Example:

```bash
--file-url https://github.com/acme/project
--file-url https://github.com/acme/project/blob/main/README.md
```

## VM Mode Flags

These flags are valid only when `--mode vm`.

### `--jupyter`

Controls host-side Jupyter installation.

Default:

```bash
--jupyter=true
```

Example:

```bash
--jupyter=false
```

### `--lifecycle-script`

Inline lifecycle/setup script.

The script:

- must start with `#!/bin/bash`
- must be at most 16 KiB

Example:

```bash
--lifecycle-script '#!/bin/bash
echo hello
apt-get update'
```

For anything more than a few lines, prefer `--lifecycle-script-file`.

### `--lifecycle-script-file`

Path to a local lifecycle/setup script file.

This is mutually exclusive with `--lifecycle-script`.

Example:

```bash
--lifecycle-script-file ./scripts/setup.sh
```

## Compose Mode Flags

These flags are valid only when `--mode compose`.

Exactly one compose source is required.

### `--compose-url`

Public remote Docker Compose URL.

Blob URLs are normalized to raw URLs before validation and create.

Example:

```bash
--compose-url https://github.com/acme/project/blob/main/docker-compose.yml
```

### `--compose-file`

Path to a local Docker Compose file.

Example:

```bash
--compose-file ./docker-compose.yml
```

### `--compose-yaml`

Inline Docker Compose YAML.

Best for short examples. For real-world compose content, prefer `--compose-file`.

Example:

```bash
--compose-yaml $'version: "3"\nservices:\n  web:\n    image: nginx\n    ports:\n      - "8080:80"'
```

### `--jupyter`

Also applies in compose mode.

In compose mode this controls host-side Jupyter installation in the launchable build request.

## Network Flags

The CLI exposes two separate networking concepts:

- secure links
- firewall rules

Secure links are user-facing exposed entry points stored in `buildRequest.ports`.

Firewall rules are lower-level ingress rules stored in `createWorkspaceRequest.firewallRules`.

### `--secure-link`

Repeatable flag.

Format:

```text
name=<name>,port=<port>[,cta=true|false][,cta-label=<label>]
```

Fields:

- `name` is required
- `port` is required
- `cta` is optional
- `cta-label` is optional and only valid when `cta=true`

Examples:

```bash
--secure-link name=web,port=3000
--secure-link name=notebook,port=8888,cta=true
--secure-link name=api,port=8080,cta=true,cta-label=OpenAPI
```

Validation:

- secure link names must be lowercase DNS-label style names
- only lowercase letters, digits, and hyphens are allowed
- names must start and end with a lowercase letter or digit
- max length is 63 characters
- ports must be a single port, not a range
- ports must be between `1` and `65535`
- duplicate secure link names are rejected
- at most 2 secure links may use `cta=true`

Example of a valid name:

```text
web
demo-app
jupyter1
```

Example of invalid names:

```text
Web
demo_app
-demo
demo-
```

### `--firewall-rule`

Repeatable flag.

Format:

```text
ports=<port|start-end>[,allowed-ips=all|user-ip]
```

Fields:

- `ports` is required
- `allowed-ips` is optional

Default:

```text
allowed-ips=all
```

Examples:

```bash
--firewall-rule ports=8080
--firewall-rule ports=8000-8100
--firewall-rule ports=8000-8100,allowed-ips=user-ip
```

Validation:

- single ports must be between `1` and `65535`
- ranges must use `start-end`
- ranges must satisfy `start < end`
- only `all` and `user-ip` are currently accepted
- firewall rules cannot overlap each other
- firewall rules cannot overlap secure-link ports

## Dry Run

### `--dry-run`

Prints the final request JSON and exits without creating anything.

This is the best way to verify:

- which workspace group was selected
- whether storage was included
- how URLs were normalized
- how ports and firewall rules were serialized

Example:

```bash
brev launchable create demo \
  --mode vm \
  --instance-type n2-standard-4 \
  --dry-run
```

## Validation Rules and Mutual Exclusivity

The CLI intentionally catches several issues before sending the API request.

### Name and Core Input Validation

- launchable name is required
- instance type is required
- mode must be `vm` or `compose`
- view access must be `public`, `organization`, or `published`

### VM vs Compose Exclusivity

When `--mode vm`:

- `--compose-url` is invalid
- `--compose-file` is invalid
- `--compose-yaml` is invalid

When `--mode compose`:

- `--lifecycle-script` is invalid
- `--lifecycle-script-file` is invalid
- exactly one compose source is required

### Lifecycle Script Validation

- empty script is allowed
- non-empty scripts must start with `#!/bin/bash`
- max size is 16 KiB
- inline and file-based script flags are mutually exclusive

### Compose Validation

Before creation, the CLI sends the compose source to the Docker Compose validation API.

That means:

- malformed compose definitions fail early
- invalid remote compose URLs fail before create
- local compose files are read and validated before create

### Port Conflict Validation

The CLI rejects:

- secure-link port conflicts with other secure links
- secure-link port conflicts with firewall rules
- firewall rule overlaps with firewall rules

## Examples

## Example 1: Minimal VM Launchable

```bash
brev launchable create my-vm \
  --mode vm \
  --instance-type n2-standard-4
```

What this does:

- creates a VM launchable
- uses default `organization` visibility
- enables Jupyter by default
- uses default storage behavior for the instance type
- sends no preload file URL
- sends no secure links
- sends no firewall rules

## Example 2: VM Launchable With Script and Secure Link

```bash
brev launchable create ds-notebook \
  --mode vm \
  --instance-type n2-standard-4 \
  --description "Notebook-based onboarding launchable" \
  --lifecycle-script-file ./scripts/bootstrap.sh \
  --secure-link name=notebook,port=8888,cta=true,cta-label=OpenNotebook
```

What this does:

- installs Jupyter on the host
- attaches a lifecycle script
- exposes a user-facing secure link named `notebook`
- makes that secure link a CTA entry point

## Example 3: VM Launchable With Repo Preload and Firewall Rules

```bash
brev launchable create training-vm \
  --mode vm \
  --instance-type g5.xlarge \
  --storage 512 \
  --file-url https://github.com/acme/training-project \
  --secure-link name=jupyter,port=8888,cta=true \
  --firewall-rule ports=8000-8100,allowed-ips=user-ip
```

What this does:

- creates a GPU-backed VM launchable
- sets storage to 512 GiB if the instance type supports flexible storage
- preloads the repo into the workspace
- exposes Jupyter as a secure link
- opens a user-IP-scoped firewall range

## Example 4: Compose Launchable From Remote URL

```bash
brev launchable create compose-demo \
  --mode compose \
  --instance-type g5.xlarge \
  --compose-url https://github.com/acme/project/blob/main/docker-compose.yml \
  --secure-link name=web,port=3000,cta=true,cta-label=LaunchApp
```

What this does:

- converts the GitHub blob URL to a raw URL
- validates the compose document before create
- creates a compose-backed launchable
- exposes port `3000` as the main user-facing link

## Example 5: Compose Launchable From Local File

```bash
brev launchable create local-compose \
  --mode compose \
  --instance-type g5.xlarge \
  --compose-file ./docker-compose.yml \
  --file-url https://github.com/acme/project \
  --secure-link name=ui,port=8080,cta=true \
  --secure-link name=api,port=8000
```

## Example 6: Compose Launchable From Inline YAML

```bash
brev launchable create inline-compose \
  --mode compose \
  --instance-type g5.xlarge \
  --compose-yaml $'version: "3"\nservices:\n  web:\n    image: nginx\n    ports:\n      - "8080:80"' \
  --secure-link name=web,port=8080,cta=true
```

## Example 7: Inspect Request Without Creating

```bash
brev launchable create dry-run-demo \
  --mode compose \
  --instance-type g5.xlarge \
  --compose-file ./docker-compose.yml \
  --secure-link name=web,port=3000,cta=true \
  --firewall-rule ports=8000-8010,allowed-ips=user-ip \
  --dry-run
```

## Request Shape

The CLI builds a request that looks conceptually like this:

```json
{
  "name": "demo",
  "description": "Example launchable",
  "viewAccess": "public",
  "createWorkspaceRequest": {
    "workspaceGroupId": "wg-123",
    "instanceType": "g5.xlarge",
    "storage": "256",
    "firewallRules": [
      {
        "port": "8000-8100",
        "allowedIPs": "user-ip"
      }
    ]
  },
  "buildRequest": {
    "dockerCompose": {
      "fileUrl": "https://github.com/acme/project/raw/main/docker-compose.yml",
      "jupyterInstall": true
    },
    "ports": [
      {
        "name": "web",
        "port": "3000",
        "labels": {
          "cta-enabled": "true",
          "cta-value": "LaunchApp"
        }
      }
    ]
  },
  "file": {
    "url": "https://github.com/acme/project",
    "path": "./"
  }
}
```

## Successful Output

On success, the CLI prints:

- the created launchable ID
- a deploy URL based on the configured console URL

Conceptually:

```text
Created launchable "demo" (env-abc123)
Deploy URL: https://brev.nvidia.com/launchable/deploy?launchableID=env-abc123
```

## Troubleshooting

### `--storage` is rejected

That means the instance type is exposing fixed storage in the current API response. Omit `--storage`.

### Compose create fails before launchable creation

That usually means compose validation failed. Check:

- YAML syntax
- remote URL accessibility
- whether the URL points to a file instead of a directory

### Secure link name is rejected

Rename it to a lowercase DNS-label style value such as:

```text
web
demo-app
jupyter
```

### Firewall rule conflicts

The CLI checks secure-link ports and firewall rule ranges together. Make sure:

- no secure-link port is duplicated
- no secure-link port falls inside a firewall range
- firewall ranges do not overlap each other

## Recommended Usage Patterns

### Prefer `--compose-file` over `--compose-yaml`

Use inline YAML only for quick tests. For anything real, a file is easier to maintain and quote safely.

### Use `--dry-run` before large or shared launchables

This is the fastest way to verify the final request.

### Keep CTA links intentional

Use `cta=true` only on the 1-2 main entry points you want users to click first.

### Use `--file-url` for preload content, not for compose source

Use:

- `--file-url` for repo/notebook/markdown preload content
- `--compose-*` for the actual Docker Compose definition

## Quick Reference

### Minimal VM

```bash
brev launchable create demo --mode vm --instance-type n2-standard-4
```

### VM With Script

```bash
brev launchable create demo --mode vm --instance-type n2-standard-4 --lifecycle-script-file ./setup.sh
```

### Compose From File

```bash
brev launchable create demo --mode compose --instance-type g5.xlarge --compose-file ./docker-compose.yml
```

### With Secure Link

```bash
brev launchable create demo --mode vm --instance-type n2-standard-4 --secure-link name=web,port=8080,cta=true
```

### With Firewall Rule

```bash
brev launchable create demo --mode vm --instance-type n2-standard-4 --firewall-rule ports=8000-8100,allowed-ips=user-ip
```

### Preview Request

```bash
brev launchable create demo --mode vm --instance-type n2-standard-4 --dry-run
```
