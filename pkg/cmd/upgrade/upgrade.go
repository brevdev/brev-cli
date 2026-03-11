// Package upgrade provides the brev upgrade command.
package upgrade

import (
	"fmt"
	"os"

	"github.com/brevdev/brev-cli/pkg/cmd/agentskill"
	"github.com/brevdev/brev-cli/pkg/cmd/register"
	"github.com/brevdev/brev-cli/pkg/cmd/version"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/sudo"
	"github.com/brevdev/brev-cli/pkg/terminal"

	"github.com/spf13/cobra"
)

// VersionStore fetches the latest release metadata from GitHub.
type VersionStore interface {
	GetLatestReleaseMetadata() (*store.GithubReleaseMetadata, error)
}

// SkillInstaller updates agent skill files after a binary upgrade.
type SkillInstaller interface {
	InstallSkill(t *terminal.Terminal) error
}

// defaultSkillInstaller calls agentskill.InstallSkill using the real home directory.
type defaultSkillInstaller struct{}

func (defaultSkillInstaller) InstallSkill(t *terminal.Terminal) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not determine home directory: %w", err)
	}
	if err := agentskill.InstallSkill(t, homeDir, false); err != nil {
		return fmt.Errorf("install skill: %w", err)
	}
	return nil
}

type upgradeDeps struct {
	detector       Detector
	upgrader       Upgrader
	confirmer      terminal.Confirmer
	gater          sudo.Gater
	skillInstaller SkillInstaller
}

func defaultUpgradeDeps() upgradeDeps {
	return upgradeDeps{
		detector:       SystemDetector{},
		upgrader:       SystemUpgrader{},
		confirmer:      register.TerminalPrompter{},
		gater:          sudo.Default,
		skillInstaller: defaultSkillInstaller{},
	}
}

var (
	upgradeLong    = "Upgrade brev to the latest version."
	upgradeExample = "  brev upgrade"
)

// NewCmdUpgrade creates the brev upgrade command.
func NewCmdUpgrade(t *terminal.Terminal, versionStore VersionStore) *cobra.Command {
	cmd := &cobra.Command{
		Annotations:           map[string]string{"configuration": ""},
		Use:                   "upgrade",
		DisableFlagsInUseLine: true,
		Short:                 "Upgrade brev to the latest version",
		Long:                  upgradeLong,
		Example:               upgradeExample,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(t, versionStore, defaultUpgradeDeps())
		},
	}
	return cmd
}

func runUpgrade(t *terminal.Terminal, vs VersionStore, deps upgradeDeps) error {
	t.Vprint("")
	t.Vprintf("Current version: %s\n", version.Version)

	release, err := vs.GetLatestReleaseMetadata()
	if err != nil {
		return fmt.Errorf("failed to check latest version: %w", err)
	}

	if release.TagName == version.Version {
		t.Vprint(t.Green("Already up to date."))
		return nil
	}

	t.Vprintf("New version available: %s\n", release.TagName)
	t.Vprint("")

	method := deps.detector.Detect()

	var (
		upgraded   bool
		upgradeErr error
	)
	switch method {
	case InstallMethodBrew:
		upgraded, upgradeErr = upgradeViaBrew(t, deps)
	case InstallMethodDirect:
		upgraded, upgradeErr = upgradeViaDirect(t, deps)
	default:
		return fmt.Errorf("unknown install method")
	}

	if upgradeErr != nil {
		return upgradeErr
	}

	if upgraded {
		if err := deps.skillInstaller.InstallSkill(t); err != nil {
			t.Vprintf("  Warning: skill update failed: %v\n", err)
			t.Vprintf("  You can retry with: brev agent-skill install\n")
		}
	}

	return nil
}

func upgradeViaBrew(t *terminal.Terminal, deps upgradeDeps) (bool, error) {
	t.Vprint("Detected install method: Homebrew")
	t.Vprint("This will run: brew upgrade brev")
	t.Vprint("")

	if !deps.confirmer.ConfirmYesNo("Proceed with upgrade?") {
		t.Vprint("Upgrade canceled.")
		return false, nil
	}

	t.Vprint("")
	if err := deps.upgrader.UpgradeViaBrew(t); err != nil {
		return false, fmt.Errorf("brew upgrade: %w", err)
	}

	t.Vprint("")
	t.Vprint(t.Green("Upgrade complete."))
	return true, nil
}

func upgradeViaDirect(t *terminal.Terminal, deps upgradeDeps) (bool, error) {
	t.Vprint("Detected install method: direct binary install")
	t.Vprint("This will download the latest release and install it to /usr/local/bin/brev")
	t.Vprint("")

	if err := deps.gater.Gate(t, deps.confirmer, "Upgrade", false); err != nil {
		return false, fmt.Errorf("sudo issue: %w", err)
	}

	t.Vprint("")
	if err := deps.upgrader.UpgradeViaInstallScript(t); err != nil {
		return false, fmt.Errorf("direct upgrade: %w", err)
	}

	t.Vprint("")
	t.Vprint(t.Green("Upgrade complete."))
	return true, nil
}
