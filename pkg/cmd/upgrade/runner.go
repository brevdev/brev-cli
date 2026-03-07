package upgrade

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/brevdev/brev-cli/pkg/terminal"
)

// Upgrader executes the actual upgrade process.
type Upgrader interface {
	UpgradeViaBrew(t *terminal.Terminal) error
	UpgradeViaInstallScript(t *terminal.Terminal) error
}

// SystemUpgrader executes upgrade commands on the real system.
type SystemUpgrader struct{}

// UpgradeViaBrew runs "brew upgrade brev" with output connected to the terminal.
func (SystemUpgrader) UpgradeViaBrew(t *terminal.Terminal) error {
	t.Vprint("Running: brew upgrade brev")
	t.Vprint("")

	cmd := exec.Command("brew", "upgrade", "brev")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("brew upgrade failed: %w", err)
	}
	return nil
}

// UpgradeViaInstallScript downloads and installs the latest brev binary from GitHub.
func (SystemUpgrader) UpgradeViaInstallScript(t *terminal.Terminal) error {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	t.Vprintf("Detected platform: %s/%s\n", osName, arch)
	t.Vprint("")

	// Fetch latest release download URL
	t.Vprint("Fetching latest release...")
	pattern := fmt.Sprintf("browser_download_url.*%s.*%s", osName, arch)
	curlCmd := fmt.Sprintf(
		`curl -s https://api.github.com/repos/brevdev/brev-cli/releases/latest | grep "%s" | cut -d '"' -f 4`,
		pattern,
	)
	out, err := exec.Command("bash", "-c", curlCmd).Output() //nolint:gosec // constructing URL pattern from known values
	if err != nil {
		return fmt.Errorf("failed to fetch latest release: %w", err)
	}
	downloadURL := string(out)
	if downloadURL == "" {
		return fmt.Errorf("could not find release for %s/%s", osName, arch)
	}
	// Trim trailing newline
	downloadURL = downloadURL[:len(downloadURL)-1]

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "brev-upgrade-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Download
	t.Vprintf("Downloading %s...\n", downloadURL)
	tarPath := filepath.Join(tmpDir, "brev.tar.gz")
	dlCmd := exec.Command("curl", "-sL", downloadURL, "-o", tarPath)
	dlCmd.Stderr = os.Stderr
	if err := dlCmd.Run(); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Extract
	extractCmd := exec.Command("tar", "-xzf", tarPath, "-C", tmpDir)
	if err := extractCmd.Run(); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Install with sudo
	brevPath := filepath.Join(tmpDir, "brev")
	t.Vprint("Installing to /usr/local/bin/brev...")
	mvCmd := exec.Command("sudo", "mv", brevPath, "/usr/local/bin/brev")
	mvCmd.Stdout = os.Stdout
	mvCmd.Stderr = os.Stderr
	if err := mvCmd.Run(); err != nil {
		return fmt.Errorf("failed to install binary: %w", err)
	}

	chmodCmd := exec.Command("sudo", "chmod", "+x", "/usr/local/bin/brev")
	if err := chmodCmd.Run(); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	return nil
}
