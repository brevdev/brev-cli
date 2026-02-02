# Common Brev CLI Patterns

Frequently used command combinations and workflows.

## Piping Patterns

Brev CLI is fully pipeable. Commands read from stdin and output instance names to stdout.

### The Pipe Architecture

```
brev ls → awk/grep → brev stop/start/delete/shell/open
brev search → brev create → brev open/shell
```

### Bulk Operations from `brev ls`

```bash
# Stop all running instances
brev ls | awk '/RUNNING/ {print $1}' | brev stop

# Delete all stopped instances
brev ls | awk '/STOPPED/ {print $1}' | brev delete

# Start all stopped instances
brev ls | awk '/STOPPED/ {print $1}' | brev start

# Open all running instances in editor
brev ls | awk '/RUNNING/ {print $1}' | brev open cursor

# Run command on all running instances
brev ls | awk '/RUNNING/ {print $1}' | brev shell -c "nvidia-smi"
```

### Pattern Matching with grep

```bash
# Stop all "test-*" instances
brev ls | grep "test-" | awk '{print $1}' | brev stop

# Delete all "experiment-*" instances
brev ls | grep "experiment-" | awk '{print $1}' | brev delete

# Open all "dev-*" instances
brev ls | grep "^dev-" | awk '{print $1}' | brev open

# Stop instances with specific GPU type
brev ls | grep "A100" | awk '{print $1}' | brev stop
```

### Preview Before Destructive Operations

```bash
# Preview: see what would be deleted
brev ls | awk '/STOPPED/ {print $1}'

# Execute: actually delete them
brev ls | awk '/STOPPED/ {print $1}' | brev delete

# Preview: see what would be stopped
brev ls | grep "test-" | awk '/RUNNING/ {print $1}'

# Execute: stop them
brev ls | grep "test-" | awk '/RUNNING/ {print $1}' | brev stop
```

### Create → Access Chains

```bash
# Create and open in editor
brev search --gpu-name A100 | brev create my-box | brev open cursor

# Create and SSH immediately
brev create my-box | brev shell

# Create and run setup command
brev create my-box | brev shell -c "pip install torch"

# Create cluster and verify all nodes
brev create my-cluster --count 3 | brev shell -c "hostname && nvidia-smi"
```

## Running Scripts with @filepath

Use `@` prefix to run local scripts on remote instances without copying files first.

### Basic Script Execution
```bash
# Run a local script on remote instance
brev shell my-instance -c @setup.sh

# Absolute path
brev shell my-instance -c @/home/user/scripts/install.sh

# Relative path
brev shell my-instance -c @./scripts/deploy.sh
```

### Create + Setup in One Command
```bash
# Create instance and run setup script
brev create my-instance | brev shell -c @setup.sh

# With specific GPU
brev search --gpu-name A100 | brev create ml-box | brev shell -c @ml-setup.sh
```

### Run Script on Multiple Instances
```bash
# Same script on all cluster nodes
brev shell node-1 node-2 node-3 -c @setup.sh

# Or pipe from create
brev create my-cluster --count 3 | brev shell -c @setup.sh
```

### Example Setup Scripts

**ml-setup.sh** - ML environment:
```bash
#!/bin/bash
pip install torch torchvision torchaudio --index-url https://download.pytorch.org/whl/cu121
pip install transformers accelerate datasets wandb
nvidia-smi
python -c "import torch; print(f'CUDA available: {torch.cuda.is_available()}')"
```

**dev-setup.sh** - Development environment:
```bash
#!/bin/bash
apt-get update && apt-get install -y vim tmux htop
pip install ipython jupyter black ruff
git config --global user.name "Your Name"
git config --global user.email "you@example.com"
```

## Quick Start Patterns

### Create and Connect (One-Liner)
```bash
# Create and open in VS Code
brev search --gpu-name A100 | brev create my-box | brev open code

# Create and SSH in
brev shell $(brev create my-box)

# Create and run a command
brev create my-box | brev shell -c "nvidia-smi"
```

### Find Cheapest GPU
```bash
# Cheapest with minimum specs
brev search --min-vram 24 --sort price | head -5

# Cheapest A100
brev search --gpu-name A100 --sort price | head -1
```

## Training Workflows

### PyTorch Training Setup
```bash
# Create with PyTorch pre-installed
brev search --min-vram 40 | brev create ml-train -s 'pip install torch transformers'

# Copy training code
brev copy ./train.py ml-train:/home/ubuntu/

# Copy dataset
brev copy ./data/ ml-train:/home/ubuntu/data/

# Start training
brev shell ml-train -c "cd /home/ubuntu && python train.py"
```

### Background Training
```bash
# Start training in background with logs
brev shell ml-train -c "nohup python train.py > training.log 2>&1 &"

# Monitor logs
brev shell ml-train -c "tail -f training.log"

# Check GPU usage
brev shell ml-train -c "nvidia-smi"
```

### Multi-GPU Training
```bash
# Find multi-GPU instances
brev search --min-total-vram 160 --sort price

# Create 8-GPU cluster
brev search --gpu-name A100 | brev create training-cluster --count 8
```

## Development Workflows

### Quick Development Box
```bash
# Fast-booting cheap GPU
brev search --max-boot-time 3 --sort price | brev create dev-box

# Open in preferred editor
brev open dev-box cursor
```

### Remote Development Session
```bash
# Start tmux session
brev open my-box tmux

# Later: reconnect to same session
brev shell my-box -c "tmux attach"
```

### Port Forwarding for Jupyter
```bash
# Start Jupyter on instance
brev shell my-box -c "jupyter notebook --no-browser --port 8888 &"

# Forward port locally
brev port-forward my-box -p 8888:8888

# Access at http://localhost:8888
```

## Cluster Patterns

### Create Named Cluster
```bash
# Creates: my-cluster-1, my-cluster-2, my-cluster-3
brev create my-cluster --count 3 --type g5.xlarge
```

### Run Command on All Cluster Nodes
```bash
# Run nvidia-smi on all nodes
brev shell my-cluster-1 my-cluster-2 my-cluster-3 -c "nvidia-smi"

# Or pipe from create
brev create my-cluster --count 3 | brev shell -c "nvidia-smi"
```

### Open All Cluster Nodes
```bash
brev open my-cluster-1 my-cluster-2 my-cluster-3 cursor
```

## File Transfer Patterns

### Sync Project Directory
```bash
# Upload project
brev copy ./myproject/ my-box:/home/ubuntu/myproject/

# Download results
brev copy my-box:/home/ubuntu/results/ ./results/
```

### Backup Before Delete
```bash
# Save checkpoints
brev copy my-box:/home/ubuntu/checkpoints/ ./backups/

# Save logs
brev copy my-box:/home/ubuntu/*.log ./logs/

# Now safe to delete
brev delete my-box
```

## Organization Patterns

### Switch Orgs for Different Projects
```bash
# List orgs
brev org ls

# Switch to work org
brev set work-org

# Create work instance
brev create work-project

# Switch back to personal
brev set personal-org
```

### Check All Instances Across Orgs
```bash
# Check current org
brev ls

# Check specific org
brev ls --org other-org
```

## Troubleshooting Patterns

### Debug Connection Issues
```bash
# Refresh SSH config
brev refresh

# Check instance status
brev ls

# Try host connection
brev shell my-box --host
```

### Recover Stuck Instance
```bash
# Reset instance
brev reset my-box

# Wait and reconnect
sleep 30 && brev shell my-box
```

### Clean Up All Instances
```bash
# List all
brev ls

# Delete each (confirm each one)
brev delete instance-1
brev delete instance-2
```
