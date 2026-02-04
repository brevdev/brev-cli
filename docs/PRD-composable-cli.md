# PRD: Composable & Agentic Brev CLI

## Vision

Make the Brev CLI idiomatic, programmable, and agent-friendly. Users and AI agents should be able to compose commands using standard Unix patterns (`|`, `grep`, `awk`, `jq`) while also having structured output options for programmatic access.

## Goals

1. **Unix Idiomatic** - Commands work naturally with pipes and standard tools
2. **Programmable** - JSON output mode for all commands that return data
3. **Agentic** - Claude Code skills can orchestrate complex workflows
4. **Composable** - Output of one command feeds into input of another

## Design Principles

### Pipe Detection
- Commands detect when stdout is piped (`os.Stdout.Stat()`)
- Piped output: clean table format (no colors, no help text)
- Interactive output: colored, with contextual help

### Input Handling
- Commands accept arguments directly OR from stdin
- Stdin is read line-by-line when piped
- First column of table input is parsed as the primary identifier

### Output Formats
| Mode | Trigger | Format |
|------|---------|--------|
| Interactive | TTY | Colored table + help text |
| Piped | `cmd \| ...` | Plain table (greppable) |
| JSON | `--json` | Structured JSON array |

### Data Passthrough
- Filter flags (e.g., `--min-disk`) should propagate through pipes
- Table output includes computed fields (e.g., `TARGET_DISK`)
- JSON output includes all relevant fields

## Implemented Features

### Pipeable Commands
| Command | Stdin | Stdout (piped) | Status |
|---------|-------|----------------|--------|
| `brev ls` | - | Plain table | ✅ |
| `brev ls orgs` | - | Plain table | ✅ |
| `brev search` | - | Plain table w/ TARGET_DISK | ✅ |
| `brev stop` | Instance names | Instance names | ✅ |
| `brev start` | Instance names | Instance names | ✅ |
| `brev delete` | Instance names | Instance names | ✅ |
| `brev create` | Instance types (table or JSON) | Instance names | ✅ |
| `brev shell` | - | - (interactive) | ✅ |
| `brev shell -c` | Instance names | Command stdout/stderr | ✅ |
| `brev open` | Instance names | - | ✅ |

### Shell Enhancements (`brev shell`)

Non-interactive command execution with `-c` flag, enabling scripted and agentic workflows.

**Run commands directly**:
```bash
brev shell my-gpu -c "nvidia-smi"
brev shell my-gpu -c "python train.py && echo done"
```

**Run local scripts remotely** (`@filepath` syntax):
```bash
brev shell my-gpu -c @setup.sh           # Runs local setup.sh on remote
brev shell my-gpu -c @scripts/deploy.sh  # Relative paths supported
```

**Multi-instance support**:
```bash
# Run on multiple instances
brev shell gpu-1 gpu-2 gpu-3 -c "nvidia-smi"

# Pipe from create
brev create my-cluster --count 3 | brev shell -c "nvidia-smi"

# Chain with other commands
brev ls | grep RUNNING | brev shell -c "df -h"
```

**Output for chaining**:
When using `-c`, outputs instance names after execution completes, enabling pipelines:
```bash
brev create my-gpu | brev shell -c "pip install torch" | brev shell -c "python train.py"
```

### Open Enhancements (`brev open`)

Open instances in editors/terminals with multi-instance and cross-platform support.

**Editor options**:
```bash
brev open my-gpu vscode      # VS Code (default)
brev open my-gpu cursor      # Cursor
brev open my-gpu vim         # Vim over SSH
brev open my-gpu terminal    # Terminal/SSH session
brev open my-gpu tmux        # Tmux session
```

**Multi-instance support**:
```bash
# Open multiple instances (each in separate window)
brev open gpu-1 gpu-2 gpu-3 cursor

# Pipe from create
brev create my-cluster --count 3 | brev open cursor
```

**Cross-platform support**:
- macOS: Terminal.app, iTerm2
- Linux: Default terminal emulator
- Windows/WSL: Fixed exec format errors

### Search Filters
```bash
brev search --gpu-name H100      # Filter by GPU
brev search --min-vram 40        # Min VRAM per GPU
brev search --min-total-vram 80  # Min total VRAM
brev search --min-disk 500       # Min disk size (GB)
brev search --max-boot-time 5    # Max boot time (minutes)
brev search --stoppable          # Can stop/restart
brev search --rebootable         # Can reboot
brev search --flex-ports         # Configurable firewall
```

### JSON Mode
```bash
brev ls --json
brev ls orgs --json
brev search --json
```

## Example Workflows

### Filter and Create
```bash
# Find stoppable H100s with 500GB disk, create first match
brev search --min-disk 500 --stoppable | grep H100 | head -1 | brev create --name my-gpu
```

### Batch Operations
```bash
# Stop all running instances
brev ls | grep RUNNING | awk '{print $1}' | brev stop

# Delete all stopped instances
brev ls | grep STOPPED | awk '{print $1}' | brev delete
```

### Chained Lifecycle
```bash
# Create, use, cleanup
brev search --gpu-name A100 | head -1 | brev create --name job-1 | brev shell -c "python train.py" && brev delete job-1
```

### JSON Processing
```bash
# Get cheapest H100 with jq
brev search --json | jq '[.[] | select(.gpu_name == "H100")] | sort_by(.price_per_hour) | .[0]'
```

## Claude Code Integration

### Why Skills Matter

The composable CLI is necessary but not sufficient for agentic use. Skills bridge the gap between:

1. **Raw CLI** - Powerful but requires knowing exact flags and syntax
2. **Natural Language** - How users actually describe intent

Without skills, an agent must:
- Know that `--min-total-vram` exists (not `--vram`, `--gpu-memory`, etc.)
- Remember flag combinations for common tasks
- Handle error messages and retry logic
- Understand which commands can be piped together

Skills encode this domain knowledge, turning "spin up a cheap GPU for testing" into the correct `brev search --stoppable --sort price | head -1 | brev create` pipeline.

### Skill Capabilities

The `/brev-cli` skill provides:

**Natural Language → CLI Translation**
- "Create an A100 instance for ML training" → selects appropriate flags
- "Find GPUs with 40GB VRAM under $2/hr" → `--min-total-vram 40` + price filter
- "Stop all my running instances" → `brev ls | grep RUNNING | ... | brev stop`

**Context-Aware Defaults**
- Knows common GPU requirements for ML workloads
- Suggests `--stoppable` for dev instances (cost savings)
- Recommends disk sizes based on use case

**Error Recovery**
- Retries with fallback instance types on capacity errors
- Suggests alternatives when requested GPU unavailable
- Handles "instance already exists" gracefully

**Workflow Orchestration**
- Multi-step operations (create → wait → execute → cleanup)
- Monitors instance health during long-running jobs
- Streams logs and captures results

### Agentic Patterns

With composable CLI + skills, agents can autonomously:

1. **Provision** - Search, filter, and create instances matching workload requirements
2. **Deploy** - Stream code/data to instances via pipeable `cp`
3. **Execute** - Run workloads via `brev shell -c`, capture output
4. **Monitor** - Poll status via `brev ls --json`, stream logs
5. **Scale** - Spin up parallel instances, distribute work
6. **Cleanup** - Stop/delete instances, manage costs

### Example: Autonomous Training Job

```
User: "Train my model on an H100, save checkpoints every hour"

Agent:
1. brev search --gpu-name H100 --stoppable --min-disk 500 | head -1 | brev create --name training-job
2. brev wait training-job --state ready
3. tar czf - ./src | brev cp - training-job:/app/
4. brev shell training-job -c "cd /app && python train.py --checkpoint-interval 3600"
5. brev cp training-job:/app/checkpoints - | tar xzf - -C ./results/
6. brev delete training-job
```

The skill handles the translation, error recovery, and orchestration—the composable CLI makes each step possible.

## Future Considerations

### Planned

#### `brev logs` - Stream/tail instance logs
```bash
brev logs my-gpu                    # Follow logs
brev logs my-gpu --since 5m         # Last 5 minutes
brev logs my-gpu | grep ERROR       # Filter logs
```

#### `brev wait` - Block until instance reaches state
```bash
brev create --name my-gpu ... && brev wait my-gpu --state ready
brev stop my-gpu && brev wait my-gpu --state stopped
```

#### `brev cp` - Pipeable file copy (stdin/stdout)

Stream data directly through stdin/stdout without intermediate files. Uses `-` to indicate stdin/stdout (standard Unix convention).

**Current behavior** (requires temp files):
```bash
brev cp local.tar.gz my-gpu:/data/
brev cp my-gpu:/results/output.csv ./output.csv
```

**Proposed pipeable behavior**:
```bash
# Stream archive directly to instance
tar czf - ./data | brev cp - my-gpu:/data/archive.tar.gz

# Pipe file content to instance
cat model.pt | brev cp - my-gpu:/models/model.pt

# Stream from instance and process locally
brev cp my-gpu:/results/output.csv - | grep "success" > filtered.csv

# Transfer between instances without local storage
brev cp gpu-1:/checkpoint.pt - | brev cp - gpu-2:/checkpoint.pt
```

**Agentic use cases**:
```bash
# Agent streams training data, captures results
cat dataset.jsonl | brev shell my-gpu -c "python train.py" > results.log

# Agent deploys code without temp files
tar czf - ./src | brev cp - my-gpu:/app/src.tar.gz

# Agent extracts specific results
brev cp my-gpu:/logs/metrics.json - | jq '.accuracy'
```
