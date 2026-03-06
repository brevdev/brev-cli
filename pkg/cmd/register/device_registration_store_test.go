package register

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/spf13/afero"
)

func setupTestFs(t *testing.T) func() {
	t.Helper()
	origFs := files.AppFs
	files.AppFs = afero.NewMemMapFs()
	if err := files.AppFs.MkdirAll(globalRegistrationDir, 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}
	return func() { files.AppFs = origFs }
}

func Test_SaveAndLoadRegistration_RoundTrip(t *testing.T) {
	cleanup := setupTestFs(t)
	defer cleanup()

	store := NewFileRegistrationStore()

	cpuCount := int32(12)
	ramBytes := int64(137438953472)
	reg := &DeviceRegistration{
		ExternalNodeID: "unode_abc123",
		DisplayName:    "My Spark",
		OrgID:          "org_xyz",
		DeviceID:       "device-uuid-123",
		RegisteredAt:   "2026-02-25T00:00:00Z",
		NodeSpec: NodeSpec{
			CPUCount:     &cpuCount,
			RAMBytes:     &ramBytes,
			Architecture: "arm64",
		},
	}

	if err := store.Save(reg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ExternalNodeID != reg.ExternalNodeID {
		t.Errorf("ExternalNodeID mismatch: got %s, want %s", loaded.ExternalNodeID, reg.ExternalNodeID)
	}
	if loaded.DisplayName != reg.DisplayName {
		t.Errorf("DisplayName mismatch: got %s, want %s", loaded.DisplayName, reg.DisplayName)
	}
	if loaded.OrgID != reg.OrgID {
		t.Errorf("OrgID mismatch: got %s, want %s", loaded.OrgID, reg.OrgID)
	}
	if loaded.DeviceID != reg.DeviceID {
		t.Errorf("DeviceID mismatch: got %s, want %s", loaded.DeviceID, reg.DeviceID)
	}
	if loaded.NodeSpec.Architecture != "arm64" {
		t.Errorf("Architecture mismatch: got %s", loaded.NodeSpec.Architecture)
	}
	if loaded.NodeSpec.CPUCount == nil || *loaded.NodeSpec.CPUCount != 12 {
		t.Errorf("CPUCount mismatch: got %v", loaded.NodeSpec.CPUCount)
	}
}

func Test_RegistrationExists_ReturnsFalseWhenMissing(t *testing.T) {
	cleanup := setupTestFs(t)
	defer cleanup()

	store := NewFileRegistrationStore()

	exists, err := store.Exists()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected Exists to return false")
	}
}

func Test_RegistrationExists_ReturnsTrueAfterSave(t *testing.T) {
	cleanup := setupTestFs(t)
	defer cleanup()

	store := NewFileRegistrationStore()

	reg := &DeviceRegistration{
		ExternalNodeID: "unode_abc123",
		DisplayName:    "Test",
	}
	if err := store.Save(reg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	exists, err := store.Exists()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected Exists to return true")
	}
}

func Test_DeleteRegistration_RemovesFile(t *testing.T) {
	cleanup := setupTestFs(t)
	defer cleanup()

	store := NewFileRegistrationStore()

	reg := &DeviceRegistration{
		ExternalNodeID: "unode_abc123",
		DisplayName:    "Test",
	}
	if err := store.Save(reg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := store.Delete(); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	exists, err := store.Exists()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected Exists to return false after delete")
	}
}

func Test_LoadRegistration_FailsWhenMissing(t *testing.T) {
	cleanup := setupTestFs(t)
	defer cleanup()

	store := NewFileRegistrationStore()

	_, err := store.Load()
	if err == nil {
		t.Error("expected error loading missing registration")
	}
}

func Test_LoadRegistration_RejectsMissingExternalNodeID(t *testing.T) {
	cleanup := setupTestFs(t)
	defer cleanup()

	store := NewFileRegistrationStore()

	reg := &DeviceRegistration{
		ExternalNodeID: "",
		DisplayName:    "Test",
		OrgID:          "org_xyz",
	}
	if err := store.Save(reg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	_, err := store.Load()
	if err == nil {
		t.Fatal("expected error loading registration with empty ExternalNodeID")
	}
}

func Test_LoadRegistration_RejectsMissingOrgID(t *testing.T) {
	cleanup := setupTestFs(t)
	defer cleanup()

	store := NewFileRegistrationStore()

	reg := &DeviceRegistration{
		ExternalNodeID: "unode_abc",
		DisplayName:    "Test",
		OrgID:          "",
	}
	if err := store.Save(reg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	_, err := store.Load()
	if err == nil {
		t.Fatal("expected error loading registration with empty OrgID")
	}
}

func Test_DeleteRegistration_FailsWhenMissing(t *testing.T) {
	cleanup := setupTestFs(t)
	defer cleanup()

	store := NewFileRegistrationStore()

	err := store.Delete()
	if err == nil {
		t.Error("expected error deleting missing registration")
	}
}
