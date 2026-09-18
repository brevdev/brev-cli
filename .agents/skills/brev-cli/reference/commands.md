# Brev CLI Command Reference

Complete reference for all brev commands.

## Pipeable Architecture

Brev CLI commands are designed to pipe together. Commands read instance names from stdin and output instance names to stdout, enabling powerful command chains.

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ brev search │ ──▶ │ brev create │ ──▶ │ brev open   │
│ brev ls     │     │ brev stop   │     │ brev exec   │
│ grep/awk    │     │ brev start  │     │ brev shell  │
└─────────────┘     │ brev delete │     │ brev delete │
                    └─────────────┘     └─────────────┘
```

### Piping from `brev ls`

Extract instance names using `awk` and pipe to other commands:

```bash
# Stop all running instances
brev ls | awk '/RUNNING/ {print $1}' | brev stop

# Delete all stopped instances
brev ls | awk '/STOPPED/ {print $1}' | brev delete

# Start all stopped instances
brev ls | awk '/STOPPED/ {print $1}' | brev start

# Open all running instances in Cursor
brev ls | awk '/RUNNING/ {print $1}' | brev open cursor

# Run nvidia-smi on all running instances
brev ls | awk '/RUNNING/ {print $1}' | brev exec "nvidia-smi"
```

### Filtering with grep

```bash
# Stop instances matching a pattern
brev ls | grep "test-" | awk '{print $1}' | brev stop

# Delete old experiment instances
brev ls | grep "experiment" | awk '{print $1}' | brev delete

# Open all "dev-" instances
brev ls | grep "^dev-" | awk '{print $1}' | brev open
```

### Chaining create → access

```bash
# Create and immediately open
brev search --gpu-name A100 | brev create my-box | brev open cursor

# Create and run command
brev create my-box | brev exec "nvidia-smi"

# Create and run setup script
brev create my-box | brev exec @setup.sh

# Create cluster and verify all nodes
brev create my-cluster --count 3 | brev exec "hostname && nvidia-smi"
```

### Safety: Preview before bulk operations

```bash
# Preview what will be stopped
brev ls | awk '/RUNNING/ {print $1}'

# Then actually stop (add | brev stop)
brev ls | awk '/RUNNING/ {print $1}' | brev stop
```

## Instance Commands

### brev create
Create GPU instances with automatic retry and fallback.

```bash
brev create [name] [flags]
```

**Aliases:** `provision`, `gpu-create`, `gpu-retry`, `gcreate`

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--name` | `-n` | Instance name |
| `--type` | `-t` | Comma-separated instance types to try |
| `--count` | `-c` | Number of instances (default: 1) |
| `--parallel` | `-p` | Parallel creation attempts (default: 1) |
| `--startup-script` | `-s` | Script to run on boot (string or @filepath) |
| `--timeout` | | Seconds to wait for ready (default: 300) |
| `--detached` | `-d` | Don't wait for instance to be ready |
| `--dry-run` | | Show matching instance types without creating |

**Search Filter Flags (same as `brev search`):**
| Flag | Short | Description |
|------|-------|-------------|
| `--gpu-name` | `-g` | Filter by GPU name (partial match) |
| `--provider` | | Filter by cloud provider |
| `--min-vram` | `-v` | Minimum VRAM per GPU (GB) |
| `--min-total-vram` | | Minimum total VRAM (GB) |
| `--min-capability` | | Minimum GPU compute capability |
| `--min-disk` | | Minimum disk size (GB) |
| `--max-boot-time` | | Maximum boot time (minutes) |
| `--stoppable` | | Only stoppable instances |
| `--rebootable` | | Only rebootable instances |
| `--flex-ports` | | Only instances with configurable firewall |
| `--sort` | | Sort by: price, vram, boot-time, etc. |
| `--desc` | | Sort descending |

**Smart Defaults (when no --type and no filters):**
- 20GB min total VRAM, 500GB disk, compute 8.0+, <7 min boot
- Results sorted by price (cheapest first)

**Examples:**
```bash
brev create my-instance
brev create my-instance --type g5.xlarge
brev create my-cluster --count 3 --type g5.xlarge
brev search --gpu-name A100 | brev create my-instance
brev create my-instance --gpu-name A100 --min-vram 40
brev create my-instance -s @setup.sh
brev create my-instance -s 'pip install torch'
brev create my-instance --dry-run
```

### brev search
Search and filter available instance types. Has two subcommands: `gpu` (default) and `cpu`.

```bash
brev search [gpu|cpu] [flags]
```

**Aliases:** `gpu-search`, `gpu`, `gpus`, `gpu-list`

#### GPU Search (default)
```bash
brev search [flags]
brev search gpu [flags]
brev search gpu --wide   # shows RAM and ARCH columns
```

**GPU Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--gpu-name` | `-g` | Filter by GPU name (partial match) |
| `--provider` | `-p` | Filter by cloud provider |
| `--min-vram` | `-v` | Minimum VRAM per GPU (GB) |
| `--min-total-vram` | `-t` | Minimum total VRAM (GB) |
| `--min-capability` | `-c` | Minimum GPU compute capability |
| `--min-disk` | | Minimum disk size (GB) |
| `--max-boot-time` | | Maximum boot time (minutes) |
| `--stoppable` | | Only stoppable instances |
| `--rebootable` | | Only rebootable instances |
| `--flex-ports` | | Only instances with configurable firewall |
| `--sort` | `-s` | Sort by: price, gpu-count, vram, total-vram, vcpu, disk, boot-time |
| `--desc` | `-d` | Sort descending |
| `--json` | | Output as JSON |
| `--wide` | `-w` | Show extra columns (RAM, ARCH) — gpu subcommand only |

**GPU Examples:**
```bash
brev search
brev search gpu --wide
brev search --gpu-name A100
brev search --min-vram 40 --sort price
brev search --gpu-name H100 --max-boot-time 3
brev search --stoppable --min-total-vram 40 --sort price
```

#### CPU Search
```bash
brev search cpu [flags]
```

Search for CPU-only instance types (no GPU). Uses shared flags only.

**CPU Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--provider` | `-p` | Filter by cloud provider |
| `--arch` | | Filter by architecture (x86_64, arm64) |
| `--min-ram` | | Minimum RAM in GB |
| `--min-disk` | | Minimum disk size (GB) |
| `--min-vcpu` | | Minimum number of vCPUs |
| `--max-boot-time` | | Maximum boot time (minutes) |
| `--stoppable` | | Only stoppable instances |
| `--rebootable` | | Only rebootable instances |
| `--flex-ports` | | Only instances with configurable firewall |
| `--sort` | `-s` | Sort by: price, vcpu, type, provider, disk, boot-time |
| `--desc` | `-d` | Sort descending |
| `--json` | | Output as JSON |

**CPU Examples:**
```bash
brev search cpu
brev search cpu --provider aws
brev search cpu --min-ram 64 --sort price
brev search cpu --arch arm64
brev search cpu --min-vcpu 16 --sort price
brev search cpu | brev create my-cpu-box
```

### brev ls
List instances in active org.

```bash
brev ls [subcommand] [flags]
```

**Subcommands:**
| Subcommand | Lists |
|---|---|
| *(none)* | cloud instances |
| `instances` | cloud instances (explicit form) |
| `nodes` | Brev Connect machines only |
| `orgs` | organizations |

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--org` | `-o` | Override active org |
| `--all` | | Show all instances in org |
| `--json` | | Output as JSON |

#### Instances vs. Brev Connect machines

Two separate namespaces — a Brev Connect machine never appears in `brev ls`.

```bash
$ brev ls
 NAME         STATUS   BUILD      SHELL  ID         MACHINE         GPU
 my-instance  RUNNING  COMPLETED  READY  jmorx0saw  n2d-standard-2  -

$ brev ls nodes
 NAME     STATUS
 my-node  Connected
```

|  | `brev ls` | `brev ls nodes` |
|---|---|---|
| Status values | `RUNNING` / `STOPPED` | `Connected` / `Disconnected` |
| Columns | NAME, STATUS, BUILD, SHELL, ID, MACHINE, GPU | NAME, STATUS |
| `stop` / `start` / `delete` | yes | no |

The two `--json` shapes differ — `brev ls nodes --json` returns a top-level
array, `brev ls --json` wraps rows in `.workspaces`:

```bash
brev ls nodes --json | jq -r '.[].name'
brev ls       --json | jq -r '.workspaces[].name'

# Only the nodes that are currently connected
brev ls nodes --json | jq -r '.[] | select(.status=="Connected") | .name'
```

When stdout is piped, outputs instance names only (one per line) for chaining.

### brev delete
Delete instances. Supports multiple names and stdin piping.

```bash
brev delete <instance-name>...
echo "instance-name" | brev delete
```

**Examples:**
```bash
# Delete single instance
brev delete my-instance

# Delete multiple instances
brev delete instance1 instance2 instance3

# Pipe from ls (delete all stopped)
brev ls | awk '/STOPPED/ {print $1}' | brev delete

# Delete matching pattern
brev ls | grep "test-" | awk '{print $1}' | brev delete
```

### brev stop
Stop running instances. Supports multiple names and stdin piping.

```bash
brev stop <instance-name>...
brev stop --all
echo "instance-name" | brev stop
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--all` | `-a` | Stop all running instances |

**Examples:**
```bash
# Stop single instance
brev stop my-instance

# Stop multiple instances
brev stop instance1 instance2

# Stop all running instances
brev stop --all

# Pipe from ls (stop matching pattern)
brev ls | grep "dev-" | awk '/RUNNING/ {print $1}' | brev stop
```

### brev start
Start stopped instances. Supports multiple names and stdin piping.

```bash
brev start <instance-name>...
echo "instance-name" | brev start
```

**Examples:**
```bash
# Start single instance
brev start my-instance

# Start multiple instances
brev start instance1 instance2

# Pipe from ls (start all stopped)
brev ls | awk '/STOPPED/ {print $1}' | brev start
```

### brev reset
Reset an instance to recover from errors.

```bash
brev reset <instance-name>
```

## Instance Access Commands

### brev shell
SSH into an instance interactively. For non-interactive command execution, use `brev exec`.

```bash
brev shell <instance> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--host` | SSH to host machine instead of container |

**Examples:**
```bash
# Interactive SSH
brev shell my-instance

# SSH to host machine
brev shell my-instance --host
```

### brev exec
Execute a command on one or more instances non-interactively. Supports `@filepath` syntax to run local scripts on remote instances.

```bash
brev exec [instance...] <command> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--host` | Execute on host machine instead of container |

**The `@filepath` syntax:**

Prefix a file path with `@` to run a local script on the remote instance:
- `@setup.sh` - relative path from current directory
- `@/absolute/path/script.sh` - absolute path
- The script is read locally and executed remotely

**Long-running commands:** `brev exec` holds the session until the command exits,
including for `nohup ... &` children. For long jobs (training runs), have the script
detach fully (`setsid nohup <job> > run.log 2>&1 < /dev/null & disown`) and write a
completion marker (`echo DONE >> ~/progress.log`) that later `brev exec` calls can poll.
- Works with any shell script

**Examples:**
```bash
# Run inline command
brev exec my-instance "nvidia-smi"
brev exec my-instance "pip install torch && python train.py"

# Run local script on remote instance
brev exec my-instance @setup.sh
brev exec my-instance @./scripts/install-deps.sh
brev exec my-instance @/home/user/my-script.sh

# Run on multiple instances
brev exec instance1 instance2 instance3 "nvidia-smi"
brev exec instance1 instance2 instance3 @setup.sh

# Chain with create (reads instance names from stdin)
brev create my-instance | brev exec "nvidia-smi"
brev create my-instance | brev exec @setup.sh

# Pipeline: create, setup, then run
brev create my-gpu | brev exec "pip install torch" | brev exec "python train.py"
```

### brev open
Open an editor connected to the instance.

```bash
brev open <instance> [editor] [flags]
```

**Editors:** `code`, `cursor`, `windsurf`, `terminal`, `tmux`

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--dir` | `-d` | Directory to open |
| `--wait` | `-w` | Wait for setup to finish |
| `--set-default` | | Set default editor |
| `--host` | | Connect to host instead of container |

**Examples:**
```bash
brev open my-instance
brev open my-instance cursor
brev open --set-default cursor
brev create my-instance | brev open code
```

### brev copy
Copy files to/from instances.

```bash
brev copy <source> <destination>
```

**Aliases:** `cp`, `scp`

**Flags:**
| Flag | Description |
|------|-------------|
| `--host` | Copy to/from host instead of container |

**Examples:**
```bash
brev copy ./local-file my-instance:/remote/path/
brev copy my-instance:/remote/file ./local-path/
brev copy ./data/ my-instance:/home/ubuntu/data/
```

### brev port-forward
Forward remote port to local.

```bash
brev port-forward <instance> -p <local>:<remote>
```

**Examples:**
```bash
brev port-forward my-instance -p 8080:8080
brev port-forward my-instance -p 3000:3000
```

### brev ports ls
List Brev-managed ports for a managed instance or Brev Connect machine.

```bash
brev ports ls <instance-or-brev-connect-machine> [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--json` | Output the port mappings as JSON |

The compact table output includes ID, endpoint, public port, destination port,
and protocol. Ports are ordered by protocol (`HTTPS`, `HTTP`, `TCP`, `UDP`,
`SSH`), then by ascending destination port. Use `brev ports get` to inspect
authorization, IP restrictions, and the remaining data for one mapping.

For managed instances, this command reads the Brev-managed network
configuration. It does not synthesize the legacy secure-link or firewall rows
shown by the Brev console, because those rows do not have real port IDs and
cannot be used by port-management automation. If the instance is still
provisioning or uses legacy network access, the command returns an error
directing the user to the console.

The JSON output is an array with the following stable fields:

| Field | Meaning |
|------|---------|
| `port_id` | Unique port mapping ID used by automation |
| `endpoint` | Public URL or `host:port` endpoint |
| `public_port` | Externally addressable port |
| `destination_port` | Port listening on the target machine |
| `protocol` | `HTTP`, `HTTPS`, `SSH`, `TCP`, `UDP`, or `UNKNOWN` |
| `allowed_sources` | Allowed IP addresses or CIDR blocks |
| `authorized_emails` | Identities authorized for an HTTP application |
| `allow_public_unauthenticated` | Whether an HTTP application is public |
| `type` | `system`, `user`, `unspecified`, or `unknown` |

**Examples:**
```bash
brev ports ls my-instance
brev ports ls my-connect-machine
brev ports ls my-instance --json
```

### Get a port

Get all data for one port mapping by its exact `port_id`.

```bash
brev ports get <port-id> [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--json` | Output the port mapping as a JSON object |

Human-readable output includes ID, endpoint, public and destination ports,
protocol, allowed sources, authorized emails, public-access state, and type.
The JSON object uses the same stable fields documented for `brev ports ls`.

**Examples:**
```bash
brev ports get nport-abc123
brev ports get nport-abc123 --json
```

### Open a port

Open a TCP, UDP, SSH, HTTP, or HTTPS port on a managed instance or Brev
Connect machine. TCP and UDP also accept inclusive sequential ranges.

```bash
brev ports open <instance-or-brev-connect-machine> <port-or-range> [flags]
```

**Flags:**
| Flag | Description |
|------|-------------|
| `--protocol` | Port protocol: `tcp` (default), `udp`, `ssh`, `http`, or `https` |
| `--allow` | Source CIDR for TCP, UDP, or SSH; repeat to add more than one |
| `--authorize` | Email authorized for an HTTP endpoint; repeat to add more than one |
| `--hostname` | HTTP endpoint hostname prefix; defaults to the destination port |
| `--public` | Disable authentication for an HTTP endpoint |
| `--json` | Output the opened port as JSON |

Omit `--allow` to allow raw-port connections from any source. HTTP endpoints
default to authorizing the current user's email; use `--public` to make one
available without authentication. `--protocol http` connects the public HTTPS
endpoint to a plain-HTTP service, while `https` expects TLS on the destination.

**Examples:**
```bash
brev ports open my-instance 8080
brev ports open my-instance 8000-8031
brev ports open my-connect-machine 53 --protocol udp
brev ports open my-instance 8080 --allow 203.0.113.10/32
brev ports open my-connect-machine 2222 --protocol ssh --json
brev ports open my-instance 3000 --protocol http --public
brev ports open my-instance 8888 --protocol http --authorize me@example.com
```

### Update a port

Update a mapping in place while preserving its globally unique `port_id` and
public endpoint. The CLI resolves the owning environment or Brev Connect
machine automatically. `edit` is an alias for `update`.

```bash
brev ports update <port-id> [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--destination-port` | Change the destination port (1-65535) |
| `--allow` | Replace source restrictions with this CIDR; repeat to add more than one |
| `--allow-anywhere` | Clear all source restrictions |
| `--protocol` | Change an HTTP mapping's origin protocol to `http` or `https` |
| `--authorize` | Replace an HTTP mapping's authorized emails; repeat to add more than one |
| `--public` | Allow unauthenticated public access to an HTTP mapping |
| `--json` | Output the updated mapping as JSON |

`--allow` and `--allow-anywhere` cannot be combined. `--authorize` and
`--public` cannot be combined. HTTP access and protocol flags are rejected for
raw TCP, UDP, and SSH mappings. When one command updates multiple field groups,
the API applies them in destination, source, protocol, then access order; those
separate mutations are not transactional.

**Examples:**
```bash
brev ports update nport-abc123 --destination-port 8081
brev ports update nport-abc123 --allow 203.0.113.10/32
brev ports edit nport-abc123 --allow-anywhere
brev ports update nport-abc123 --protocol https
brev ports update nport-abc123 --authorize me@example.com
brev ports update nport-abc123 --public --json
```

### Remove ports

Remove one port by its globally unique `port_id` without specifying its owner.
To identify a port by destination number instead, also provide the environment
or Brev Connect machine. If multiple mappings use that destination, use an
exact `port_id`. `rm` is an alias for `remove`.

```bash
brev ports remove <port-id>
brev ports remove <instance-or-brev-connect-machine> <destination-port>
```

Use `brev ports ls <instance-or-brev-connect-machine> --json` to find port IDs
for automation.

On success, the command reports the protocol, destination port, and owning
machine, for example: `Removed TCP port 8080 on my-instance.`

**Examples:**
```bash
brev ports remove my-instance 8080
brev ports rm nport-abc123
brev ports remove nport-abc123
```

## Organization Commands

### brev org ls
List organizations.

```bash
brev org ls
```

### brev org set / brev set
Set active organization.

```bash
brev org set <org-name>
brev set <org-name>
```

### brev org create
Create a new organization.

```bash
brev org create <org-name>
```

### brev invite
Generate an invite link.

```bash
brev invite
```

## Configuration Commands

### brev login / brev logout
Authenticate with Brev.

```bash
brev login
brev logout
```

### brev refresh
Refresh SSH config.

```bash
brev refresh
```

### brev healthcheck
Check backend health.

```bash
brev healthcheck
```

### brev ssh-key
Get your public SSH key.

```bash
brev ssh-key
```
