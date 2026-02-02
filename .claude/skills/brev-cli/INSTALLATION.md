# Installing the Brev CLI Claude Code Skill

## One-Liner Install

```bash
curl -fsSL https://raw.githubusercontent.com/brevdev/brev-cli/main/scripts/install-claude-skill.sh | bash
```

## What Gets Installed

```
~/.claude/skills/brev-cli/
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

```bash
curl -fsSL https://raw.githubusercontent.com/brevdev/brev-cli/main/scripts/install-claude-skill.sh | bash -s -- --branch my-branch
```

### Uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/brevdev/brev-cli/main/scripts/install-claude-skill.sh | bash -s -- --uninstall
```

### Manual Installation

If you prefer to install manually:

```bash
# Clone the repo
git clone https://github.com/brevdev/brev-cli.git
cd brev-cli

# Copy skill to Claude directory
mkdir -p ~/.claude/skills/
cp -r .claude/skills/brev-cli ~/.claude/skills/
```

## After Installation

1. **Restart Claude Code** or start a new conversation
2. **Verify installation:**
   ```bash
   ls ~/.claude/skills/brev-cli/
   ```
3. **Test the skill:**
   - Say "search for A100 GPUs" or
   - Use `/brev-cli`

## Troubleshooting

### Skill not appearing

- Make sure you restarted Claude Code
- Check the file exists: `cat ~/.claude/skills/brev-cli/SKILL.md`
- Verify YAML frontmatter is valid (no tabs, proper formatting)

### Permission denied

```bash
# Fix permissions
chmod -R 755 ~/.claude/skills/brev-cli/
```

### Update to latest version

Just run the installer again - it will overwrite existing files:

```bash
curl -fsSL https://raw.githubusercontent.com/brevdev/brev-cli/main/scripts/install-claude-skill.sh | bash
```
