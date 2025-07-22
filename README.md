<boolp align="center">
<img width="230" src="https://raw.githubusercontent.com/brevdev/assets/main/logo.svg"/>
</p>

# Brev.dev

[![](https://uohmivykqgnnbiouffke.supabase.co/storage/v1/object/public/landingpage/createdevenv1.svg)](https://console.brev.dev/environment/new?repo=https://github.com/brevdev/brev-cli&instance=2x8)

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

## Docs

https://docs.nvidia.com/brev/latest/

---

https://user-images.githubusercontent.com/14320477/170176621-6b871798-baef-4d42-affe-063a76eca9da.mp4

## Contributing

We welcome PRs! Checkout [Contributing.md](docs/CONTRIBUTING.md) for more.
