# Quick GPU Session Workflow

Get a GPU instance up and running quickly.

## Step 1: Determine Requirements

Use AskUserQuestion:
```
question: "What do you need the GPU for?"
header: "Use Case"
options:
  - label: "ML Training"
    description: "Need high VRAM (40GB+), longer session"
  - label: "Inference/Testing"
    description: "Moderate VRAM (16-24GB), quick access"
  - label: "Development"
    description: "Any GPU, focus on fast boot time"
  - label: "Specific GPU"
    description: "I know exactly what I need"
```

## Step 2: Search for GPUs

Based on use case:

**ML Training:**
```bash
brev search --min-vram 40 --sort price
```

**Inference/Testing:**
```bash
brev search --min-vram 16 --max-boot-time 5 --sort price
```

**Development:**
```bash
brev search --max-boot-time 3 --sort price
```

**Specific GPU (ask user):**
```bash
brev search --gpu-name <user-specified>
```

## Step 3: Get Instance Name

Ask: "What should we name this instance?"

Suggest: Based on use case (e.g., `ml-training`, `dev-box`, `inference-test`)

## Step 4: Create Instance

```bash
# Show what we're creating
echo "Creating instance with: <selected-type>"

# Create and capture name
brev search <filters> | brev create <name>
```

Wait for instance to be ready.

## Step 5: Open Editor

Ask which editor:
```
question: "How do you want to connect?"
header: "Editor"
options:
  - label: "VS Code"
  - label: "Cursor"
  - label: "Terminal/SSH"
  - label: "Just create, I'll connect later"
```

```bash
brev open <name> <editor>
```

## Step 6: Confirm

Report:
- Instance name
- Instance type
- GPU specs
- How to reconnect later: `brev shell <name>` or `brev open <name>`
