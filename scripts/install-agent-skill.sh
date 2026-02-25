#!/bin/bash
#
# Brev CLI Agent Skill Installer
#
# Installs the brev-cli skill to both ~/.claude/skills/brev-cli/ and ~/.agents/skills/brev-cli/
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/brevdev/brev-cli/main/scripts/install-agent-skill.sh | bash
#
# Or with a specific branch:
#   curl -fsSL https://raw.githubusercontent.com/brevdev/brev-cli/main/scripts/install-agent-skill.sh | bash -s -- --branch my-branch
#

set -e

# Configuration
REPO="brevdev/brev-cli"
BRANCH="main"
SKILL_NAME="brev-cli"
INSTALL_DIRS=("$HOME/.claude/skills/$SKILL_NAME" "$HOME/.agents/skills/$SKILL_NAME")
BASE_URL="https://raw.githubusercontent.com/$REPO"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --branch|-b)
            BRANCH="$2"
            shift 2
            ;;
        --uninstall|-u)
            UNINSTALL=true
            shift
            ;;
        --help|-h)
            echo "Brev CLI Agent Skill Installer"
            echo ""
            echo "Usage:"
            echo "  install-agent-skill.sh [options]"
            echo ""
            echo "Options:"
            echo "  --branch, -b <branch>  Install from specific branch (default: main)"
            echo "  --uninstall, -u        Uninstall the skill"
            echo "  --help, -h             Show this help message"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

# Uninstall
if [[ "$UNINSTALL" == "true" ]]; then
    for dir in "${INSTALL_DIRS[@]}"; do
        if [[ -d "$dir" ]]; then
            echo -e "${YELLOW}Uninstalling brev-cli skill from $dir...${NC}"
            rm -rf "$dir"
            echo -e "${GREEN}✓ Removed $dir${NC}"
        fi
    done
    echo -e "${BLUE}Restart your AI coding agent to apply changes.${NC}"
    exit 0
fi

echo -e "${BLUE}Installing brev-cli agent skill...${NC}"
echo -e "  Branch: ${YELLOW}$BRANCH${NC}"
for dir in "${INSTALL_DIRS[@]}"; do
    echo -e "  Target: ${YELLOW}$dir${NC}"
done
echo ""

# Files to download (relative to .agents/skills/brev-cli/)
FILES=(
    "SKILL.md"
    "prompts/quick-session.md"
    "prompts/ml-training.md"
    "prompts/cleanup.md"
    "reference/commands.md"
    "reference/search-filters.md"
    "examples/common-patterns.md"
)

# Create directory structure for all install paths
echo -e "${BLUE}Creating directory structure...${NC}"
for dir in "${INSTALL_DIRS[@]}"; do
    mkdir -p "$dir"/{prompts,reference,examples}
done

# Download files once, copy to all paths
echo -e "${BLUE}Downloading skill files...${NC}"
TMPDIR=$(mktemp -d)
FAILED=0
for file in "${FILES[@]}"; do
    url="$BASE_URL/$BRANCH/.agents/skills/$SKILL_NAME/$file"
    tmp_dest="$TMPDIR/$file"
    mkdir -p "$(dirname "$tmp_dest")"

    if curl -fsSL "$url" -o "$tmp_dest" 2>/dev/null; then
        for dir in "${INSTALL_DIRS[@]}"; do
            cp "$tmp_dest" "$dir/$file"
        done
        echo -e "  ${GREEN}✓${NC} $file"
    else
        echo -e "  ${RED}✗${NC} $file (failed to download)"
        FAILED=$((FAILED + 1))
    fi
done
rm -rf "$TMPDIR"

echo ""

if [[ $FAILED -gt 0 ]]; then
    echo -e "${YELLOW}Warning: $FAILED file(s) failed to download.${NC}"
    echo -e "${YELLOW}The skill may not work correctly.${NC}"
else
    echo -e "${GREEN}✓ Skill installed successfully!${NC}"
fi

echo ""
echo -e "${BLUE}Next steps:${NC}"
echo -e "  1. Restart your AI coding agent (or start a new conversation)"
echo -e "  2. Use ${YELLOW}/brev-cli${NC} or say ${YELLOW}\"create a gpu instance\"${NC}"
echo ""
echo -e "${BLUE}Quick commands:${NC}"
echo -e "  ${YELLOW}brev search${NC}                    # Find available GPUs"
echo -e "  ${YELLOW}brev create my-instance${NC}        # Create an instance"
echo -e "  ${YELLOW}brev ls${NC}                        # List instances"
echo -e "  ${YELLOW}brev shell my-instance${NC}         # SSH into instance"
echo ""
