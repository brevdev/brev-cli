package upgrade

import (
	"fmt"
	"testing"

	"github.com/brevdev/brev-cli/pkg/cmd/version"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
)

type mockVersionStore struct {
	release *store.GithubReleaseMetadata
	err     error
}

func (m *mockVersionStore) GetLatestReleaseMetadata() (*store.GithubReleaseMetadata, error) {
	return m.release, m.err
}

type mockDetector struct{ method InstallMethod }

func (m mockDetector) Detect() InstallMethod { return m.method }

type mockUpgrader struct {
	brewCalled   bool
	directCalled bool
	err          error
}

func (m *mockUpgrader) UpgradeViaBrew(_ *terminal.Terminal) error {
	m.brewCalled = true
	return m.err
}

func (m *mockUpgrader) UpgradeViaInstallScript(_ *terminal.Terminal) error {
	m.directCalled = true
	return m.err
}

type mockConfirmer struct{ confirm bool }

func (m mockConfirmer) ConfirmYesNo(_ string) bool { return m.confirm }

func Test_runUpgrade_AlreadyUpToDate(t *testing.T) {
	origVersion := version.Version
	version.Version = "v1.0.0"
	defer func() { version.Version = origVersion }()

	vs := &mockVersionStore{release: &store.GithubReleaseMetadata{TagName: "v1.0.0"}}
	upgrader := &mockUpgrader{}
	deps := upgradeDeps{
		detector:  mockDetector{method: InstallMethodBrew},
		upgrader:  upgrader,
		confirmer: mockConfirmer{confirm: true},
	}

	term := terminal.New()
	err := runUpgrade(term, vs, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if upgrader.brewCalled || upgrader.directCalled {
		t.Error("upgrader should not be called when already up to date")
	}
}

func Test_runUpgrade_BrewPath(t *testing.T) {
	origVersion := version.Version
	version.Version = "v1.0.0"
	defer func() { version.Version = origVersion }()

	vs := &mockVersionStore{release: &store.GithubReleaseMetadata{TagName: "v2.0.0"}}
	upgrader := &mockUpgrader{}
	deps := upgradeDeps{
		detector:  mockDetector{method: InstallMethodBrew},
		upgrader:  upgrader,
		confirmer: mockConfirmer{confirm: true},
	}

	term := terminal.New()
	err := runUpgrade(term, vs, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !upgrader.brewCalled {
		t.Error("expected brew upgrade to be called")
	}
	if upgrader.directCalled {
		t.Error("direct upgrade should not be called for brew installs")
	}
}

func Test_runUpgrade_DirectPath(t *testing.T) {
	origVersion := version.Version
	version.Version = "v1.0.0"
	defer func() { version.Version = origVersion }()

	vs := &mockVersionStore{release: &store.GithubReleaseMetadata{TagName: "v2.0.0"}}
	upgrader := &mockUpgrader{}
	deps := upgradeDeps{
		detector:  mockDetector{method: InstallMethodDirect},
		upgrader:  upgrader,
		confirmer: mockConfirmer{confirm: true},
	}

	term := terminal.New()
	err := runUpgrade(term, vs, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !upgrader.directCalled {
		t.Error("expected direct upgrade to be called")
	}
	if upgrader.brewCalled {
		t.Error("brew upgrade should not be called for direct installs")
	}
}

func Test_runUpgrade_UserCancels(t *testing.T) {
	origVersion := version.Version
	version.Version = "v1.0.0"
	defer func() { version.Version = origVersion }()

	vs := &mockVersionStore{release: &store.GithubReleaseMetadata{TagName: "v2.0.0"}}
	upgrader := &mockUpgrader{}
	deps := upgradeDeps{
		detector:  mockDetector{method: InstallMethodBrew},
		upgrader:  upgrader,
		confirmer: mockConfirmer{confirm: false},
	}

	term := terminal.New()
	err := runUpgrade(term, vs, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if upgrader.brewCalled || upgrader.directCalled {
		t.Error("upgrader should not be called when user cancels")
	}
}

func Test_runUpgrade_VersionCheckFails(t *testing.T) {
	vs := &mockVersionStore{err: fmt.Errorf("network error")}
	deps := upgradeDeps{
		detector:  mockDetector{method: InstallMethodBrew},
		upgrader:  &mockUpgrader{},
		confirmer: mockConfirmer{confirm: true},
	}

	term := terminal.New()
	err := runUpgrade(term, vs, deps)
	if err == nil {
		t.Fatal("expected error when version check fails")
	}
}

func Test_runUpgrade_UpgraderFails(t *testing.T) {
	origVersion := version.Version
	version.Version = "v1.0.0"
	defer func() { version.Version = origVersion }()

	vs := &mockVersionStore{release: &store.GithubReleaseMetadata{TagName: "v2.0.0"}}
	upgrader := &mockUpgrader{err: fmt.Errorf("brew failed")}
	deps := upgradeDeps{
		detector:  mockDetector{method: InstallMethodBrew},
		upgrader:  upgrader,
		confirmer: mockConfirmer{confirm: true},
	}

	term := terminal.New()
	err := runUpgrade(term, vs, deps)
	if err == nil {
		t.Fatal("expected error when upgrader fails")
	}
}
