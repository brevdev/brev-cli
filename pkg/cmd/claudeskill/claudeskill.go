// Package claudeskill handles installation of the Brev CLI Claude Code skill
package claudeskill

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
	baseURLTemplate = "https://raw.githubusercontent.com/brevdev/brev-cli/%s/.claude/skills/brev-cli"

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

// ClaudeSkillStore interface for any store dependencies
type ClaudeSkillStore interface {
	UserHomeDir() (string, error)
}

// NewCmdClaudeSkill creates the claude-skill command
func NewCmdClaudeSkill(t *terminal.Terminal, store ClaudeSkillStore) *cobra.Command {
	var uninstall bool

	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "claude-skill",
		DisableFlagsInUseLine: true,
		Short:                 "Install the Brev CLI skill for Claude Code",
		Long:                  "Install or uninstall the Brev CLI skill for Claude Code AI assistant",
		Example:               "brev claude-skill\nbrev claude-skill --uninstall",
		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, err := store.UserHomeDir()
			if err != nil {
				return breverrors.WrapAndTrace(err)
			}

			if uninstall {
				return UninstallSkill(t, homeDir)
			}
			return InstallSkill(t, homeDir, false)
		},
	}

	cmd.Flags().BoolVarP(&uninstall, "uninstall", "u", false, "Uninstall the Claude Code skill")

	return cmd
}

// GetSkillDir returns the path to the Claude skill directory
func GetSkillDir(homeDir string) string {
	return filepath.Join(homeDir, ".claude", "skills", skillName)
}

// IsClaudeInstalled checks if Claude Code appears to be installed
func IsClaudeInstalled(homeDir string) bool {
	claudeDir := filepath.Join(homeDir, ".claude")
	_, err := os.Stat(claudeDir)
	return err == nil
}

// IsSkillInstalled checks if the brev-cli skill is already installed
func IsSkillInstalled(homeDir string) bool {
	skillFile := filepath.Join(GetSkillDir(homeDir), "SKILL.md")
	_, err := os.Stat(skillFile)
	return err == nil
}

// PromptInstallSkill asks the user if they want to install the Claude skill
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
	fmt.Println("  ", caretType("▸"), "    Claude Code Integration")
	fmt.Println()
	fmt.Println("       We detected Claude Code on your system.")
	fmt.Println("       Would you like to install the Brev CLI skill?")
	fmt.Println()
	fmt.Println("       This enables natural language commands like:")
	fmt.Println(t.Yellow("         \"Create an A100 instance for ML training\""))
	fmt.Println(t.Yellow("         \"Search for GPUs with 40GB VRAM\""))
	fmt.Println(t.Yellow("         \"Stop all my running instances\""))
	fmt.Println()

	prompt := promptui.Select{
		Label: "Install Claude Code skill",
		Items: []string{"Yes, install it", "No, skip for now"},
	}

	idx, _, err := prompt.Run()
	if err != nil {
		return false
	}

	return idx == 0
}

// InstallSkill downloads and installs the Claude skill
func InstallSkill(t *terminal.Terminal, homeDir string, quiet bool) error {
	skillDir := GetSkillDir(homeDir)
	baseURL := getBaseURL()

	if !quiet {
		fmt.Println()
		fmt.Printf("  Installing brev-cli skill to %s\n", t.Yellow(skillDir))
		// Show branch if not default
		if branch := os.Getenv(branchEnvVar); branch != "" {
			fmt.Printf("  Using branch: %s\n", t.Yellow(branch))
		}
		fmt.Println()
	}

	// Create directory structure
	dirs := []string{
		skillDir,
		filepath.Join(skillDir, "prompts"),
		filepath.Join(skillDir, "reference"),
		filepath.Join(skillDir, "examples"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return breverrors.WrapAndTrace(err)
		}
	}

	// Download files
	client := &http.Client{Timeout: 30 * time.Second}
	failed := 0

	for _, file := range skillFiles {
		url := fmt.Sprintf("%s/%s", baseURL, file)
		destPath := filepath.Join(skillDir, file)

		if err := downloadFile(client, url, destPath); err != nil {
			if !quiet {
				fmt.Printf("    %s %s\n", t.Red("✗"), file)
			}
			failed++
		} else if !quiet {
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
	fmt.Println("    1. Restart Claude Code (or start a new conversation)")
	fmt.Println("    2. Say " + t.Yellow("\"create a gpu instance\"") + " or use " + t.Yellow("/brev-cli"))
	fmt.Println()

	return nil
}

// UninstallSkill removes the Claude skill
func UninstallSkill(t *terminal.Terminal, homeDir string) error {
	skillDir := GetSkillDir(homeDir)

	if !IsSkillInstalled(homeDir) {
		fmt.Printf("  Skill not installed at %s\n", skillDir)
		return nil
	}

	fmt.Printf("  Uninstalling skill from %s\n", skillDir)

	if err := os.RemoveAll(skillDir); err != nil {
		return breverrors.WrapAndTrace(err)
	}

	fmt.Printf("  %s Skill uninstalled\n", t.Green("✓"))
	fmt.Println("  Restart Claude Code to apply changes.")

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

// downloadFile downloads a file from URL to destPath
func downloadFile(client *http.Client, url, destPath string) error {
	resp, err := client.Get(url) //nolint:gosec,noctx // URL is hardcoded
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return breverrors.NewValidationError(fmt.Sprintf("failed to download %s: %s", url, resp.Status))
	}

	// Read body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Write file
	if err := os.WriteFile(destPath, body, 0o644); err != nil { //nolint:gosec // skill files are not sensitive
		return breverrors.WrapAndTrace(err)
	}

	return nil
}

// PromptInstallSkillSimple is a simpler yes/no prompt for the login flow
func PromptInstallSkillSimple() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Install Claude Code skill? [y/N]: ")
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}
