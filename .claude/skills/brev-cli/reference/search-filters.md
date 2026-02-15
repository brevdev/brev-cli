# GPU Search Filters Reference

Detailed guide to filtering and sorting GPU instance types.

## Filter Options

### GPU Name (`--gpu-name`, `-g`)
Filter by GPU model name. Case-insensitive, partial match.

```bash
# Exact matches
brev search --gpu-name A100
brev search --gpu-name H100
brev search --gpu-name L40S

# Partial matches
brev search --gpu-name A10      # Matches A100, A10G, etc.
brev search --gpu-name RTX      # Matches RTX 4090, RTX 3090, etc.
```

### Provider (`--provider`, `-p`)
Filter by cloud provider. Case-insensitive, partial match.

```bash
brev search --provider aws
brev search --provider gcp
brev search --provider azure
```

### VRAM (`--min-vram`, `-v`)
Minimum VRAM per GPU in GB.

```bash
brev search --min-vram 16    # 16GB+ per GPU
brev search --min-vram 24    # 24GB+ per GPU
brev search --min-vram 40    # 40GB+ per GPU (A100 40GB, etc.)
brev search --min-vram 80    # 80GB+ per GPU (A100 80GB, H100)
```

### Total VRAM (`--min-total-vram`, `-t`)
Minimum total VRAM across all GPUs in GB.

```bash
brev search --min-total-vram 48   # 48GB total (e.g., 2x 24GB)
brev search --min-total-vram 80   # 80GB total (e.g., 1x 80GB or 2x 40GB)
brev search --min-total-vram 160  # 160GB total (e.g., 2x 80GB)
```

### Compute Capability (`--min-capability`, `-c`)
Minimum GPU compute capability (architecture).

| Capability | Architecture | GPUs |
|------------|--------------|------|
| 7.0 | Volta | V100 |
| 7.5 | Turing | T4, RTX 20xx |
| 8.0 | Ampere | A100, A10, A30 |
| 8.6 | Ampere | RTX 30xx, A40 |
| 8.9 | Ada Lovelace | L40, RTX 40xx |
| 9.0 | Hopper | H100 |

```bash
brev search --min-capability 8.0   # Ampere or newer
brev search --min-capability 9.0   # Hopper only
```

### Disk Size (`--min-disk`)
Minimum disk size in GB.

```bash
brev search --min-disk 500    # 500GB+ disk
brev search --min-disk 1000   # 1TB+ disk
```

### Boot Time (`--max-boot-time`)
Maximum boot time in minutes.

```bash
brev search --max-boot-time 3    # Fast boot (< 3 min)
brev search --max-boot-time 5    # Moderate boot (< 5 min)
brev search --max-boot-time 10   # Any boot time
```

## Sort Options

### Sort Column (`--sort`, `-s`)

| Column | Description |
|--------|-------------|
| `price` | Hourly cost (default) |
| `gpu-count` | Number of GPUs |
| `vram` | VRAM per GPU |
| `total-vram` | Total VRAM |
| `vcpu` | Number of vCPUs |
| `type` | Instance type name |
| `provider` | Cloud provider |
| `disk` | Disk size |
| `boot-time` | Boot time |

```bash
brev search --sort price        # Cheapest first (default)
brev search --sort vram         # Highest VRAM first
brev search --sort boot-time    # Fastest boot first
```

### Sort Direction (`--desc`, `-d`)
Sort in descending order.

```bash
brev search --sort price --desc   # Most expensive first
brev search --sort vram --desc    # Highest VRAM first
```

## Common Filter Combinations

### Development (fast, cheap)
```bash
brev search --max-boot-time 3 --sort price
```

### ML Training (high VRAM)
```bash
brev search --min-vram 40 --sort price
```

### Large Model Training
```bash
brev search --min-total-vram 80 --sort price
```

### Production (fast + reliable)
```bash
brev search --gpu-name A100 --max-boot-time 5 --sort price
```

### Budget Conscious
```bash
brev search --min-vram 16 --sort price | head -5
```

### Latest Architecture
```bash
brev search --min-capability 9.0 --sort price
```

## Output Format

### Table Output (default)
```
TYPE            GPU         COUNT  VRAM   TOTAL   $/HR   BOOT  FEATURES
g5.xlarge       A10G        1      24GB   24GB    $1.00  2m    S R P
g5.2xlarge      A10G        1      24GB   24GB    $1.20  2m    S R P
p4d.24xlarge    A100        8      40GB   320GB   $32.77 5m    S R
```

### JSON Output (`--json`)
```bash
brev search --json | jq '.[] | {type, gpu_name, price}'
```

## Features Column

| Code | Meaning |
|------|---------|
| S | Stoppable - can stop/restart without data loss |
| R | Rebootable - can reboot the instance |
| P | Flex Ports - can modify firewall rules |
