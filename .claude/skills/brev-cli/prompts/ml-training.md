# ML Training Setup Workflow

Set up a GPU instance for machine learning training.

## Step 1: Determine GPU Requirements

Use AskUserQuestion:
```
question: "What size model are you training?"
header: "Model Size"
options:
  - label: "Small (< 1B params)"
    description: "16-24GB VRAM sufficient"
  - label: "Medium (1-7B params)"
    description: "40-48GB VRAM recommended"
  - label: "Large (7B+ params)"
    description: "80GB+ VRAM, multi-GPU"
  - label: "Not sure"
    description: "I'll describe the task"
```

## Step 2: Search for Suitable GPUs

**Small models:**
```bash
brev search --min-vram 16 --sort price
```

**Medium models:**
```bash
brev search --min-vram 40 --sort price
```

**Large models:**
```bash
brev search --min-total-vram 80 --sort price
```

Show top 3-5 options with prices.

## Step 3: Setup Script

Ask: "Do you have a setup script, or should we use defaults?"

**Default setup script:**
```bash
cat > /tmp/ml-setup.sh << 'EOF'
#!/bin/bash
pip install torch torchvision torchaudio --index-url https://download.pytorch.org/whl/cu121
pip install transformers accelerate datasets
pip install wandb tensorboard
nvidia-smi
EOF
```

**User-provided:**
- Accept path to script file
- Or inline commands

## Step 4: Create Instance

```bash
# With setup script
brev search <filters> | brev create <name> --startup-script @/tmp/ml-setup.sh

# Or inline
brev search <filters> | brev create <name> -s 'pip install torch transformers'
```

## Step 5: Copy Training Data (Optional)

Ask: "Do you need to copy training data to the instance?"

```bash
# Copy dataset
brev copy ./data/ <name>:/home/ubuntu/data/

# Copy training script
brev copy ./train.py <name>:/home/ubuntu/
```

## Step 6: Verify Setup

```bash
# Check GPU is available
brev shell <name> -c "nvidia-smi"

# Check PyTorch sees GPU
brev shell <name> -c "python -c 'import torch; print(torch.cuda.is_available())'"
```

## Step 7: Start Training

Options:
1. **Interactive:** `brev shell <name>` then run manually
2. **Background:** `brev shell <name> -c "nohup python train.py &"`
3. **Editor:** `brev open <name> cursor`

## Step 8: Monitor

```bash
# Check GPU usage
brev shell <name> -c "nvidia-smi"

# Check training logs
brev shell <name> -c "tail -f training.log"
```

## Cleanup Reminder

When training completes:
```bash
# Copy results back
brev copy <name>:/home/ubuntu/checkpoints/ ./checkpoints/

# Delete instance to stop billing
brev delete <name>
```
