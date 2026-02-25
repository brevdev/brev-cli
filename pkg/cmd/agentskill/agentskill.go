// Package agentskill handles installation of the Brev CLI agent skill
package agentskill

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

const (
	// GitHub raw content base URL template
	baseURLTemplate = "https://raw.githubusercontent.com/brevdev/brev-cli/%s/.agents/skills/brev-cli"

	// Default branch
	defaultBranch = "main"

	// Environment variable to override branch (for testing)
	branchEnvVar = "BREV_SKILL_BRANCH"

	// Skill name
	skillName = "brev-cli"
)

// getBaseURL returns the base URL for downloading skill files
// Uses BREV_SKILL_BRANCH env var if set, otherwise defaults to main
func getBaseURL() string {
	branch := os.Getenv(branchEnvVar)
	if branch == "" {
		branch = defaultBranch
	}
	return fmt.Sprintf(baseURLTemplate, branch)
}

// Files to download (relative to skill directory)
var skillFiles = []string{
	"SKILL.md",
	"INSTALLATION.md",
	"prompts/quick-session.md",
	"prompts/ml-training.md",
	"prompts/cleanup.md",
	"reference/commands.md",
	"reference/search-filters.md",
	"examples/common-patterns.md",
}

// AgentSkillStore interface for any store dependencies
type AgentSkillStore interface {
	UserHomeDir() (string, error)
}

// NewCmdAgentSkill creates the agent-skill command with install/uninstall subcommands
func NewCmdAgentSkill(t *terminal.Terminal, store AgentSkillStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "agent-skill",
		DisableFlagsInUseLine: true,
		Short:                 "Manage the Brev CLI skill for AI coding agents",
		Long:                  "Install or uninstall the Brev CLI skill for AI coding agents (Claude Code, etc.)",
		Example:               "brev agent-skill install\nbrev agent-skill uninstall",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default to install when run without subcommand
			homeDir, err := store.UserHomeDir()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return InstallSkill(t, homeDir, false)
		},
	}

	cmd.AddCommand(newCmdInstall(t, store))
	cmd.AddCommand(newCmdUninstall(t, store))

	return cmd
}

func newCmdInstall(t *terminal.Terminal, store AgentSkillStore) *cobra.Command {
	return &cobra.Command{
		Use:     "install",
		Short:   "Install the Brev CLI skill for AI coding agents",
		Example: "brev agent-skill install",
		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, err := store.UserHomeDir()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return InstallSkill(t, homeDir, false)
		},
	}
}

func newCmdUninstall(t *terminal.Terminal, store AgentSkillStore) *cobra.Command {
	return &cobra.Command{
		Use:     "uninstall",
		Short:   "Uninstall the Brev CLI skill for AI coding agents",
		Example: "brev agent-skill uninstall",
		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, err := store.UserHomeDir()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}
			return UninstallSkill(t, homeDir)
		},
	}
}

// installDirs are the parent directories under $HOME where skills are installed.
// We install to both so the skill works with Claude Code (~/.claude) and other agents (~/.agent).
var installDirs = []string{".claude", ".agents"}

// GetSkillDirs returns all paths where the skill should be installed
func GetSkillDirs(homeDir string) []string {
	dirs := make([]string, len(installDirs))
	for i, dir := range installDirs {
		dirs[i] = filepath.Join(homeDir, dir, "skills", skillName)
	}
	return dirs
}

// GetSkillDir returns the primary skill directory (for backwards compat)
func GetSkillDir(homeDir string) string {
	return filepath.Join(homeDir, ".claude", "skills", skillName)
}

// IsClaudeInstalled checks if Claude Code appears to be installed
func IsClaudeInstalled(homeDir string) bool {
	claudeDir := filepath.Join(homeDir, ".claude")
	_, err := os.Stat(claudeDir)
	return err == nil
}

// IsSkillInstalled checks if the brev-cli skill is installed in any location
func IsSkillInstalled(homeDir string) bool {
	for _, dir := range GetSkillDirs(homeDir) {
		skillFile := filepath.Join(dir, "SKILL.md")
		if _, err := os.Stat(skillFile); err == nil {
			return true
		}
	}
	return false
}

// PromptInstallSkill asks the user if they want to install the agent skill
// Returns true if they want to install, false otherwise
func PromptInstallSkill(t *terminal.Terminal, homeDir string) bool {
	// Skip if skill is already installed
	if IsSkillInstalled(homeDir) {
		return false
	}

	// Check if Claude Code appears to be installed
	if !IsClaudeInstalled(homeDir) {
		return false
	}

	fmt.Println()
	caretType := color.New(color.FgCyan, color.Bold).SprintFunc()
	fmt.Println("  ", caretType("▸"), "    AI Agent Integration")
	fmt.Println()
	fmt.Println("       We detected an AI coding agent on your system.")
	fmt.Println("       Would you like to install the Brev CLI skill?")
	fmt.Println()
	fmt.Println("       This enables natural language commands like:")
	fmt.Println(t.Yellow("         \"Create an A100 instance for ML training\""))
	fmt.Println(t.Yellow("         \"Search for GPUs with 40GB VRAM\""))
	fmt.Println(t.Yellow("         \"Stop all my running instances\""))
	fmt.Println()

	prompt := promptui.Select{
		Label: "Install agent skill",
		Items: []string{"Yes, install it", "No, skip for now"},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return false
	}

	return idx == 0
}

// InstallSkill downloads and installs the agent skill to all install paths
func InstallSkill(t *terminal.Terminal, homeDir string, quiet bool) error {
	skillDirs := GetSkillDirs(homeDir)
	baseURL := getBaseURL()

	if !quiet {
		fmt.Println()
		for _, d := range skillDirs {
			fmt.Printf("  Installing brev-cli skill to %s\n", t.Yellow(d))
		}
		if branch := os.Getenv(branchEnvVar); branch != "" {
			fmt.Printf("  Using branch: %s\n", t.Yellow(branch))
		}
		fmt.Println()
	}

	// Create directory structure for all install paths
	for _, skillDir := range skillDirs {
		for _, sub := range []string{"", "prompts", "reference", "examples"} {
			if err := os.MkdirAll(filepath.Join(skillDir, sub), 0o755); err != nil {
				return breverrors.WrapAndTrace(err)
			}
		}
	}

	// Download files once, then write to all paths
	client := &http.Client{Timeout: 30 * time.Second}
	failed := 0

	for _, file := range skillFiles {
		url := fmt.Sprintf("%s/%s", baseURL, file)

		body, err := downloadBytes(client, url)
		if err != nil {
			if !quiet {
				fmt.Printf("    %s %s\n", t.Red("✗"), file)
			}
			failed++
			continue
		}

		for _, skillDir := range skillDirs {
			destPath := filepath.Join(skillDir, file)
			if writeErr := os.WriteFile(destPath, body, 0o644); writeErr != nil { //nolint:gosec // skill files are not sensitive
				if !quiet {
					fmt.Printf("    %s %s (%s)\n", t.Red("✗"), file, skillDir)
				}
				failed++
			}
		}

		if !quiet {
			fmt.Printf("    %s %s\n", t.Green("✓"), file)
		}
	}

	fmt.Println()

	if failed > 0 {
		fmt.Printf("  %s %d file(s) failed to download\n", t.Yellow("Warning:"), failed)
	} else {
		fmt.Printf("  %s Skill installed successfully!\n", t.Green("✓"))
	}

	fmt.Println()
	fmt.Println("  " + t.Green("Next steps:"))
	fmt.Println("    1. Restart your AI coding agent (or start a new conversation)")
	fmt.Println("    2. Say " + t.Yellow("\"create a gpu instance\"") + " or use " + t.Yellow("/brev-cli"))
	fmt.Println()

	return nil
}

// UninstallSkill removes the agent skill from all install paths
func UninstallSkill(t *terminal.Terminal, homeDir string) error {
	if !IsSkillInstalled(homeDir) {
		fmt.Println("  Skill is not installed")
		return nil
	}

	for _, skillDir := range GetSkillDirs(homeDir) {
		if _, err := os.Stat(skillDir); err != nil {
			continue
		}
		fmt.Printf("  Uninstalling skill from %s\n", skillDir)
		if err := os.RemoveAll(skillDir); err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	fmt.Printf("  %s Skill uninstalled\n", t.Green("✓"))
	fmt.Println("  Restart your AI coding agent to apply changes.")

	return nil
}

// RunInstallSkillIfWanted prompts and installs if user wants it
// This is called from the login flow
func RunInstallSkillIfWanted(t *terminal.Terminal, homeDir string) {
	if PromptInstallSkill(t, homeDir) {
		err := InstallSkill(t, homeDir, false)
		if err != nil {
			// Don't fail login for skill install errors
			fmt.Printf("  %s Failed to install skill: %v\n", t.Yellow("Warning:"), err)
		}
	}
}

// downloadBytes fetches a URL and returns the response body
func downloadBytes(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url) //nolint:gosec,noctx // URL is hardcoded
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, breverrors.NewValidationError(fmt.Sprintf("failed to download %s: %s", url, resp.Status))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return body, nil
}

// PromptInstallSkillSimple is a simpler yes/no prompt for the login flow
func PromptInstallSkillSimple() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Install agent skill? [y/N]: ")
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}
