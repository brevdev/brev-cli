# Installing the Brev CLI Agent Skill

## One-Liner Install

```bash
curl -fsSL https://raw.githubusercontent.com/brevdev/brev-cli/main/scripts/install-agent-skill.sh | bash
```

## What Gets Installed

The skill is installed to both `~/.claude/skills/brev-cli/` and `~/.agents/skills/brev-cli/`:

```
~/.claude/skills/brev-cli/    (for Claude Code)
~/.agents/skills/brev-cli/    (for other AI agents)
├── SKILL.md                 # Main skill definition
├── prompts/
│   ├── quick-session.md     # Quick GPU session workflow
│   ├── ml-training.md       # ML training setup workflow
│   └── cleanup.md           # Instance cleanup workflow
├── reference/
│   ├── commands.md          # Full command reference
│   └── search-filters.md    # GPU search options
└── examples/
    └── common-patterns.md   # Common command patterns
```

## Options

### Install from a specific branch

**Using the standalone script:**
```bash
curl -fsSL https://raw.githubusercontent.com/brevdev/brev-cli/main/scripts/install-agent-skill.sh | bash -s -- --branch my-branch
```

**Using the CLI command:**
```bash
BREV_SKILL_BRANCH=my-branch brev agent-skill
```

### Uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/brevdev/brev-cli/main/scripts/install-agent-skill.sh | bash -s -- --uninstall
```

### Manual Installation

If you prefer to install manually:

```bash
# Clone the repo
git clone https://github.com/brevdev/brev-cli.git
cd brev-cli

# Copy skill to both directories
mkdir -p ~/.claude/skills/ ~/.agents/skills/
cp -r .agents/skills/brev-cli ~/.claude/skills/
cp -r .agents/skills/brev-cli ~/.agents/skills/
```

## After Installation

1. **Restart your AI coding agent** or start a new conversation
2. **Verify installation:**
   ```bash
   ls ~/.claude/skills/brev-cli/
   ls ~/.agents/skills/brev-cli/
   ```
3. **Test the skill:**
   - Say "search for A100 GPUs" or
   - Use `/brev-cli`

## Troubleshooting

### Skill not appearing

- Make sure you restarted your AI coding agent
- Check the file exists: `cat ~/.claude/skills/brev-cli/SKILL.md`
- Verify YAML frontmatter is valid (no tabs, proper formatting)

### Permission denied

```bash
# Fix permissions
chmod -R 755 ~/.claude/skills/brev-cli/
chmod -R 755 ~/.agents/skills/brev-cli/
```

### Update to latest version

Just run the installer again - it will overwrite existing files:

```bash
curl -fsSL https://raw.githubusercontent.com/brevdev/brev-cli/main/scripts/install-agent-skill.sh | bash
```
