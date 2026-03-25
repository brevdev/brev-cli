# NVIDIA Brev CLI

[NVIDIA Brev](https://brev.nvidia.com) provides streamlined access to NVIDIA GPU instances on popular cloud platforms, automatic environment setup, and flexible deployment options, enabling developers to start experimenting instantly.

## Install the cli

### MacOS 
Assumes [Homebrew](https://brew.sh/) (or Workbrew equivalent) are installed. 

```zsh
brew install brevdev/homebrew-brev/brev
```

### Linux

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/brevdev/brev-cli/main/bin/install-latest.sh)"
```

### Windows
**Using Brev With Windows Subsystem for Linux (WSL)**

Brev is supported on windows currently through the Windows Subsystem for Linux (WSL). This guide will walk you through the steps to get Brev up and running on your Windows machine.

**Prerequisites**
- WSL installed and configured
- Virtualization enabled in your BIOS
- Ubuntu >=22.04 installed from the Microsoft Store

Once you have WSL installed and configured, you can install Brev by running the following command in your terminal:

```bash 
bash -c "$(curl -fsSL https://raw.githubusercontent.com/brevdev/brev-cli/main/bin/install-latest.sh)"
```

### From conda-forge

To globally install `brev` [from conda-forge](https://github.com/conda-forge/brev-feedstock/) in an isolated environment with [`Pixi`](https://pixi.sh/), run

```
pixi global install brev
```
## Get Started

Log in to your Brev account:

```bash
brev login
```

Create a new GPU instance:

```bash
brev create awesome-gpu-name
```

See the instance:

```bash
brev ls
```

## Docs

https://docs.nvidia.com/brev/latest/

---

## AI Agent Integration

Brev CLI includes a skill for AI coding agents (like [Claude Code](https://claude.com/claude-code)) that enables natural language GPU instance management.

```bash
# Install via CLI
brev agent-skill

# Or via standalone installer
curl -fsSL https://raw.githubusercontent.com/brevdev/brev-cli/main/scripts/install-agent-skill.sh | bash
```

Once installed, you can say things like "create an A100 instance for ML training" or "search for GPUs with 40GB VRAM" in your AI coding agent.

## Contributing

We welcome PRs! Checkout [Contributing.md](docs/CONTRIBUTING.md) for more.
