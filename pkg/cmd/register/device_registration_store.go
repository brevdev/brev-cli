package register

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/spf13/afero"
)

const (
	registrationFileName  = "device_registration.json"
	globalRegistrationDir = "/etc/brev"
)

// DeviceRegistration is the persistent identity file for a registered device.
// Fields align with the AddNodeResponse from dev-plane.
type DeviceRegistration struct {
	ExternalNodeID string   `json:"external_node_id"`
	DisplayName    string   `json:"display_name"`
	OrgID          string   `json:"org_id"`
	DeviceID       string   `json:"device_id"`
	RegisteredAt   string   `json:"registered_at"`
	NodeSpec       NodeSpec `json:"node_spec"`
}

// RegistrationStore defines the contract for persisting device registration data.
type RegistrationStore interface {
	Save(reg *DeviceRegistration) error
	Load() (*DeviceRegistration, error)
	Delete() error
	Exists() (bool, error)
}

// FileRegistrationStore implements RegistrationStore using the global /etc/brev/ path.
type FileRegistrationStore struct{}

// NewFileRegistrationStore returns a FileRegistrationStore that reads/writes
// from /etc/brev/device_registration.json.
func NewFileRegistrationStore() *FileRegistrationStore {
	return &FileRegistrationStore{}
}

func (s *FileRegistrationStore) path() string {
	return filepath.Join(globalRegistrationDir, registrationFileName)
}

func (s *FileRegistrationStore) Save(reg *DeviceRegistration) error {
	path := s.path()
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	// Try direct write first (works in tests with in-memory FS and when running as root).
	mkdirErr := files.AppFs.MkdirAll(filepath.Dir(path), 0o755)
	if mkdirErr == nil {
		if writeErr := afero.WriteFile(files.AppFs, path, data, 0o644); writeErr == nil {
			return nil
		}
	}

	// Fall back to sudo for non-root users writing to /etc/brev/.
	return sudoWriteFile(path, data)
}

// Load reads the registration file and returns the parsed DeviceRegistration
func (s *FileRegistrationStore) Load() (*DeviceRegistration, error) {
	path := s.path()
	exists, err := s.Exists()
	if !exists {
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		return nil, breverrors.WrapAndTrace(breverrors.New("device registration not found, run 'brev register' first"))
	}
	var reg DeviceRegistration
	if err := files.ReadJSON(files.AppFs, path, &reg); err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &reg, nil
}

func (s *FileRegistrationStore) Delete() error {
	path := s.path()
	err := files.DeleteFile(files.AppFs, path)
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrPermission) {
		return breverrors.WrapAndTrace(err)
	}
	// Fall back to sudo for non-root users.
	return sudoDeleteFile(path)
}

func (s *FileRegistrationStore) Exists() (bool, error) {
	path := s.path()
	_, err := files.AppFs.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, breverrors.WrapAndTrace(err)
}

// sudoWriteFile creates the parent directory and writes data to path using sudo.
func sudoWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := exec.Command("sudo", "mkdir", "-p", dir).Run(); err != nil { //nolint:gosec // fixed base path
		return fmt.Errorf("sudo mkdir %s failed: %w", dir, err)
	}
	cmd := exec.Command("sudo", "tee", path) //nolint:gosec // fixed base path
	cmd.Stdin = bytes.NewReader(data)
	cmd.Stdout = nil // suppress tee's stdout echo
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo tee %s failed: %w", path, err)
	}
	return nil
}

// sudoDeleteFile removes a file using sudo.
func sudoDeleteFile(path string) error {
	if err := exec.Command("sudo", "rm", "-f", path).Run(); err != nil { //nolint:gosec // fixed base path
		return fmt.Errorf("sudo rm %s failed: %w", path, err)
	}
	return nil
}
