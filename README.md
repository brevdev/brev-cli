# NVIDIA Brev CLI

[NVIDIA Brev](https://brev.nvidia.com) provides streamlined access to NVIDIA GPU instances on popular cloud platforms, automatic environment setup, and flexible deployment options, enabling developers to start experimenting instantly.

## Install the cli

### From conda-forge

To globally install `brev` [from conda-forge](https://github.com/conda-forge/brev-feedstock/) in an isolated environment with [`Pixi`](https://pixi.sh/), run

```
pixi global install brev
```

### MacOS 
Assumes [Homebrew](https://brew.sh/) (or Workbrew equivalent) are installed. 

```zsh
brew install brevdev/homebrew-brev/brev && brev login
```

### Linux

```bash
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/brevdev/brev-cli/main/bin/install-latest.sh)"
brev login
```

### Windows
**Using Brev With Windows Subsystem for Linux (WSL)**

Brev is supported on windows currently through the Windows Subsystem for Linux (WSL). This guide will walk you through the steps to get Brev up and running on your Windows machine.

**Prerequisites**
- WSL installed and configured
- Virtualization enabled in your BIOS
- Ubuntu 20.04 installed from the Microsoft Store

Once you have WSL installed and configured, you can install Brev by running the following command in your terminal:

```bash 
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/brevdev/brev-cli/main/bin/install-latest.sh)"
```
**Next Steps**

Log in to your Brev account:

```bash 
brev login
```

## Get Started

https://brev.nvidia.com/

## Claude Code Integration

Use Brev with [Claude Code](https://claude.ai/code) for AI-assisted GPU instance management.

### Install the Claude Code Skill

**Option 1: Via Brev CLI (recommended)**

If you have Brev installed, the skill is offered automatically during `brev login`. Or install manually:

```bash
brev claude-skill
```

**Option 2: Standalone installer**

```bash
curl -fsSL https://raw.githubusercontent.com/brevdev/brev-cli/main/scripts/install-claude-skill.sh | bash
```

### Usage

After installing, restart Claude Code and use natural language:
- "Create an A100 instance for ML training"
- "Search for GPUs with 40GB VRAM"
- "Stop all my running instances"

Or invoke directly with `/brev-cli`.

### Uninstall

```bash
brev claude-skill --uninstall
```

See [.claude/skills/brev-cli/INSTALLATION.md](.claude/skills/brev-cli/INSTALLATION.md) for more options.

## Docs

https://docs.nvidia.com/brev/latest/

---

https://user-images.githubusercontent.com/14320477/170176621-6b871798-baef-4d42-affe-063a76eca9da.mp4

## Contributing

We welcome PRs! Checkout [Contributing.md](docs/CONTRIBUTING.md) for more.
