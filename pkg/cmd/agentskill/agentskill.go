// Package agentskill handles installation of the Brev CLI agent skill
package agentskill

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/terminal"
	"github.com/spf13/cobra"
)

const (
	// GitHub raw content base URL template
	baseURLTemplate = "https://raw.githubusercontent.com/brevdev/brev-cli/%s/.agents/skills/brev-cli"

	// GitHub API URL template for resolving branch to commit SHA
	githubAPICommitTemplate = "https://api.github.com/repos/brevdev/brev-cli/commits/%s"

	// Default branch
	defaultBranch = "main"

	// Environment variable to override branch (for testing)
	branchEnvVar = "BREV_SKILL_BRANCH"

	// Skill name
	skillName = "brev-cli"

	// Version file name
	versionFileName = ".version"
)

// getBaseURL returns the base URL for downloading skill files
// Uses BREV_SKILL_BRANCH env var if set, otherwise defaults to main
func getBaseURL() string {
	return fmt.Sprintf(baseURLTemplate, getBranch())
}

// getBranch returns the branch used for downloading skill files
func getBranch() string {
	branch := os.Getenv(branchEnvVar)
	if branch == "" {
		branch = defaultBranch
	}
	return branch
}

// resolveCommitSHA uses the GitHub API to resolve a branch/ref to a commit SHA
func resolveCommitSHA(client *http.Client, ref string) (string, error) {
	url := fmt.Sprintf(githubAPICommitTemplate, ref)
	req, err := http.NewRequest("GET", url, nil) //nolint:noctx // simple API call
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req) //nolint:bodyclose // closed below
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", breverrors.NewValidationError(fmt.Sprintf("failed to resolve commit for %s: %s", ref, resp.Status))
	}

	var result struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", breverrors.WrapAndTrace(err)
	}

	return result.SHA, nil
}

// writeVersionFile writes a .version file to all skill directories
func writeVersionFile(skillDirs []string, branch, commitSHA string) {
	content := fmt.Sprintf("branch=%s\ncommit=%s\n", branch, commitSHA)
	for _, dir := range skillDirs {
		_ = os.WriteFile(filepath.Join(dir, versionFileName), []byte(content), 0o644) //nolint:gosec // not sensitive
	}
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
var installDirs = []string{".claude", ".agents", ".codex"}

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

// IsAnyAgentInstalled returns true if any of installDirs exists under homeDir.
func IsAnyAgentInstalled(homeDir string) bool {
	for _, dir := range installDirs {
		if _, err := os.Stat(filepath.Join(homeDir, dir)); err == nil {
			return true
		}
	}
	return false
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
		if !downloadAndInstallFile(client, baseURL, file, skillDirs, t, quiet) {
			failed++
		}
	}

	// Resolve commit SHA and write .version file
	branch := getBranch()
	if commitSHA, err := resolveCommitSHA(client, branch); err == nil {
		writeVersionFile(skillDirs, branch, commitSHA)
		if !quiet {
			fmt.Printf("    %s .version (%s)\n", t.Green("✓"), commitSHA[:12])
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

// downloadAndInstallFile downloads a single file and writes it to all skill dirs.
// Returns true on success, false if the download or any write failed.
func downloadAndInstallFile(client *http.Client, baseURL, file string, skillDirs []string, t *terminal.Terminal, quiet bool) bool {
	url := fmt.Sprintf("%s/%s", baseURL, file)

	body, err := downloadBytes(client, url)
	if err != nil {
		if !quiet {
			fmt.Printf("    %s %s\n", t.Red("✗"), file)
		}
		return false
	}

	fileFailed := false
	for _, skillDir := range skillDirs {
		destPath := filepath.Join(skillDir, file)
		if writeErr := os.WriteFile(destPath, body, 0o644); writeErr != nil { //nolint:gosec // skill files are not sensitive
			if !quiet {
				fmt.Printf("    %s %s (%s)\n", t.Red("✗"), file, skillDir)
			}
			fileFailed = true
		}
	}

	if fileFailed {
		return false
	}
	if !quiet {
		fmt.Printf("    %s %s\n", t.Green("✓"), file)
	}
	return true
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

