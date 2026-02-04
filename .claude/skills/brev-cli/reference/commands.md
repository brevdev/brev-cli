# Brev CLI Command Reference

Complete reference for all brev commands.

## Pipeable Architecture

Brev CLI commands are designed to pipe together. Commands read instance names from stdin and output instance names to stdout, enabling powerful command chains.

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ brev search │ ──▶ │ brev create │ ──▶ │ brev open   │
│ brev ls     │     │ brev stop   │     │ brev shell  │
│ grep/awk    │     │ brev start  │     │ brev delete │
└─────────────┘     │ brev delete │     └─────────────┘
                    └─────────────┘
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
brev ls | awk '/RUNNING/ {print $1}' | brev shell -c "nvidia-smi"
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
brev create my-box | brev shell -c "nvidia-smi"

# Create cluster and run command on all
brev create my-cluster --count 3 | brev shell -c "hostname"
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

**Examples:**
```bash
brev create my-instance
brev create my-instance --type g5.xlarge
brev create my-cluster --count 3 --type g5.xlarge
brev search --gpu-name A100 | brev create my-instance
brev create my-instance -s @setup.sh
brev create my-instance -s 'pip install torch'
```

### brev search
Search and filter available GPU instance types.

```bash
brev search [flags]
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--gpu-name` | `-g` | Filter by GPU name (partial match) |
| `--provider` | `-p` | Filter by cloud provider |
| `--min-vram` | `-v` | Minimum VRAM per GPU (GB) |
| `--min-total-vram` | `-t` | Minimum total VRAM (GB) |
| `--min-capability` | `-c` | Minimum GPU compute capability |
| `--min-disk` | | Minimum disk size (GB) |
| `--max-boot-time` | | Maximum boot time (minutes) |
| `--sort` | `-s` | Sort by: price, gpu-count, vram, total-vram, vcpu, disk, boot-time |
| `--desc` | `-d` | Sort descending |
| `--json` | | Output as JSON |

**Examples:**
```bash
brev search
brev search --gpu-name A100
brev search --min-vram 40 --sort price
brev search --gpu-name H100 --max-boot-time 3
```

### brev ls
List instances in active org.

```bash
brev ls [flags]
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--org` | `-o` | Override active org |
| `--all` | | Show all instances in org |

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
SSH into an instance or run commands.

```bash
brev shell [instance...] [flags]
```

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--command` | `-c` | Command to run (use @filename for script) |
| `--host` | | SSH to host machine instead of container |

**The `@filepath` syntax:**

Prefix a file path with `@` to run a local script on the remote instance:
- `@setup.sh` - relative path from current directory
- `@/absolute/path/script.sh` - absolute path
- The script is read locally and executed remotely
- Works with any shell script

**Examples:**
```bash
# Interactive SSH
brev shell my-instance

# Run inline command
brev shell my-instance -c "nvidia-smi"
brev shell my-instance -c "pip install torch && python train.py"

# Run local script on remote instance
brev shell my-instance -c @setup.sh
brev shell my-instance -c @./scripts/install-deps.sh
brev shell my-instance -c @/home/user/my-script.sh

# Run on multiple instances
brev shell instance1 instance2 -c "nvidia-smi"
brev shell instance1 instance2 -c @setup.sh

# Chain with create
brev create my-instance | brev shell -c "nvidia-smi"
brev create my-instance | brev shell -c @setup.sh
```

### brev open
Open an editor connected to the instance.

```bash
brev open [instance...] [editor] [flags]
```

**Editors:** `code`, `cursor`, `windsurf`, `terminal`, `tmux`

**Flags:**
| Flag | Short | Description |
|------|-------|-------------|
| `--editor` | `-e` | Editor to use |
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
