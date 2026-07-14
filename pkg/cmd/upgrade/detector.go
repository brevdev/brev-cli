package upgrade

import "os/exec"

// InstallMethod represents how brev was installed on the system.
type InstallMethod int

const (
	// InstallMethodDirect means brev was installed via direct binary download.
	InstallMethodDirect InstallMethod = 0
	// InstallMethodBrew means brev was installed via Homebrew.
	InstallMethodBrew InstallMethod = 1
)

// Detector determines how brev was installed.
type Detector interface {
	Detect() InstallMethod
}

// SystemDetector checks the actual system for install method.
type SystemDetector struct{}

// Detect checks whether brev was installed via Homebrew or direct download.
func (SystemDetector) Detect() InstallMethod {
	if _, err := exec.LookPath("brew"); err != nil {
		return InstallMethodDirect
	}
	if exec.Command("brew", "list", "brev").Run() == nil { //nolint:gosec // intentional brew probe
		return InstallMethodBrew
	}
	return InstallMethodDirect
}
