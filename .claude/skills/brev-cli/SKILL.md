---
name: brev-cli
description: Manage GPU cloud instances with the Brev CLI. Use when users want to create GPU instances, search for GPUs, SSH into instances, open editors, copy files, port forward, manage organizations, or work with cloud compute. Trigger keywords - brev, gpu, instance, create instance, ssh, vram, A100, H100, cloud gpu, remote machine.
allowed-tools: Bash, Read, AskUserQuestion
argument-hint: [create|search|shell|open|ls|delete] [instance-name]
---
<!--
Token Budget:
- Level 1 (YAML): ~100 tokens
- Level 2 (This file): ~1900 tokens (target <2000)
- Level 3 (prompts/, reference/): Loaded on demand
-->

# Brev CLI

Manage GPU cloud instances from the command line. Create, search, connect, and manage remote GPU machines.

## When to Use

Use this skill when users want to:
- Create GPU instances (with smart defaults or specific types)
- Search for available GPU types (A100, H100, L40S, etc.)
- SSH into instances or run commands remotely
- Open editors (VS Code, Cursor, Windsurf) on remote instances
- Copy files to/from instances
- Port forward from remote to local
- Manage organizations and instances

**Trigger Keywords:** brev, gpu, instance, create instance, ssh, vram, A100, H100, cloud gpu, remote machine, shell

## Quick Start

```bash
# Search for GPUs (sorted by price)
brev search

# Create an instance with smart defaults
brev create my-instance

# Create with specific GPU
brev create my-instance --type g5.xlarge

# List your instances
brev ls

# SSH into an instance
brev shell my-instance

# Open in VS Code/Cursor
brev open my-instance code
brev open my-instance cursor
```

## Core Commands

### Search GPUs
```bash
# All available GPUs
brev search

# Filter by GPU name
brev search --gpu-name A100
brev search --gpu-name H100

# Filter by VRAM, sort by price
brev search --min-vram 40 --sort price

# Filter by boot time
brev search --max-boot-time 5 --sort price
```

### Create Instances
```bash
# Smart defaults (cheapest matching GPU)
brev create my-instance

# Specific type
brev create my-instance --type g5.xlarge

# Multiple types (fallback chain)
brev create my-instance --type g5.xlarge,g5.2xlarge

# Pipe from search
brev search --gpu-name A100 | brev create my-instance

# Multiple instances
brev create my-cluster --count 3

# With startup script
brev create my-instance --startup-script @setup.sh
brev create my-instance --startup-script 'pip install torch'
```

### Instance Access
```bash
# SSH into instance
brev shell my-instance

# Run command remotely
brev shell my-instance -c "nvidia-smi"
brev shell my-instance -c "python train.py"

# Run a local script on the instance (use @filepath)
brev shell my-instance -c @setup.sh
brev shell my-instance -c @/path/to/script.sh

# Open in editor
brev open my-instance           # default editor
brev open my-instance code      # VS Code
brev open my-instance cursor    # Cursor
brev open my-instance windsurf  # Windsurf
brev open my-instance terminal  # Terminal window
brev open my-instance tmux      # Terminal + tmux

# Copy files
brev copy ./local-file my-instance:/remote/path/
brev copy my-instance:/remote/file ./local-path/

# Port forward
brev port-forward my-instance -p 8080:8080
```

### Instance Management
```bash
# List instances
brev ls

# Delete instance
brev delete my-instance

# Stop/start (if supported)
brev stop my-instance
brev start my-instance

# Reset (recover from errors)
brev reset my-instance
```

### Pipeable Workflows
```bash
# Stop all running instances
brev ls | awk '/RUNNING/ {print $1}' | brev stop

# Delete all stopped instances
brev ls | awk '/STOPPED/ {print $1}' | brev delete

# Start all stopped instances
brev ls | awk '/STOPPED/ {print $1}' | brev start

# Stop instances matching pattern
brev ls | grep "test-" | awk '{print $1}' | brev stop

# Run command on all running instances
brev ls | awk '/RUNNING/ {print $1}' | brev shell -c "nvidia-smi"

# Create and open in one command
brev search --gpu-name A100 | brev create my-box | brev open cursor
```

### Organizations
```bash
# List orgs
brev org ls

# Set active org
brev org set my-org
brev set my-org  # alias

# Generate invite link
brev invite
```

## Common Workflows

1. **Quick GPU Session** ([prompts/quick-session.md](prompts/quick-session.md))
   - Search → Create → Open editor

2. **ML Training Setup** ([prompts/ml-training.md](prompts/ml-training.md))
   - Find high-VRAM GPU → Create with startup script → Copy data → Run training

3. **Instance Cleanup** ([prompts/cleanup.md](prompts/cleanup.md))
   - List instances → Identify unused → Delete

## Safety Rules - CRITICAL

**NEVER do these without explicit user confirmation:**
- Delete instances (`brev delete`)
- Stop running instances (`brev stop`)
- Create multiple instances (`--count > 1`)
- Create expensive instances (H100, multi-GPU)

**ALWAYS do these:**
- Show instance cost/type before creating
- Confirm instance name before deletion
- Check `brev ls` before assuming instance exists

## Troubleshooting

**"Instance not found":**
- Run `brev ls` to see available instances
- Check if you're in the correct org: `brev org ls`

**"Failed to create instance":**
- Try a different instance type: `brev search --sort price`
- Check quota/credits with org admin

**SSH connection fails:**
- Run `brev refresh` to update SSH config
- Ensure instance is running: `brev ls`

**Editor won't open:**
- Verify editor is in PATH: `which code` / `which cursor`
- Set default: `brev open --set-default code`

## References

- **[reference/commands.md](reference/commands.md)** - Full command reference
- **[reference/search-filters.md](reference/search-filters.md)** - GPU search options
- **[prompts/](prompts/)** - Workflow guides
- **[examples/](examples/)** - Common patterns
