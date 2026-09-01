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

const (
	RegistrationStatusPending    = "pending"
	RegistrationStatusRegistered = "registered"
)

// DeviceRegistration is the persistent identity file for a registered device.
// Fields align with the AddNodeResponse from dev-plane.
type DeviceRegistration struct {
	ExternalNodeID       string          `json:"external_node_id"`
	DisplayName          string          `json:"display_name"`
	OrgID                string          `json:"org_id"`
	OrgName              string          `json:"org_name"`
	DeviceID             string          `json:"device_id"`
	RegisteredAt         string          `json:"registered_at"`
	HardwareProfile      HardwareProfile `json:"hardware_profile"`
	Status               string          `json:"status,omitempty"`
	CertificateAuthority string          `json:"certificate_authority,omitempty"`
}

// RegistrationStore defines the contract for persisting device registration data.
type RegistrationStore interface {
	Save(reg *DeviceRegistration) error
	Load() (*DeviceRegistration, error)
	LoadAll() (*DeviceRegistration, error)
	Delete() error
	Exists() (bool, error)
}

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

// Load returns the registration only if it is completed, any other status returns an error
func (s *FileRegistrationStore) Load() (*DeviceRegistration, error) {
	reg, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	if reg.Status != "" && reg.Status != RegistrationStatusRegistered {
		return nil, breverrors.New("device registration is incomplete; re-run 'brev register' to finish")
	}
	return reg, nil
}

// LoadAll returns the registration regardless of status
func (s *FileRegistrationStore) LoadAll() (*DeviceRegistration, error) {
	reg, err := read(s)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if err := validateRegistration(reg); err != nil {
		return nil, err
	}
	return reg, nil
}

func validateRegistration(reg *DeviceRegistration) error {
	switch reg.Status {
	case "", RegistrationStatusRegistered:
		if reg.ExternalNodeID == "" || reg.OrgID == "" {
			return breverrors.New("malformed registration, try registering")
		}
	case RegistrationStatusPending:
		if reg.OrgID == "" || reg.DeviceID == "" {
			return breverrors.New("malformed registration, try re-registering")
		}
	default:
		return fmt.Errorf("unknown registration status %q, try re-registering", reg.Status)
	}
	return nil
}

func read(s *FileRegistrationStore) (*DeviceRegistration, error) {
	path := s.path()
	exists, err := s.Exists()
	if !exists {
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		return nil, breverrors.New("device registration not found, run 'brev register' first")
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
	script := fmt.Sprintf("mkdir -p '%s' && tee '%s' > /dev/null && chmod 644 '%s'", dir, path, path)
	cmd := exec.Command("sudo", "bash", "-c", script) //nolint:gosec // fixed base path
	cmd.Stdin = bytes.NewReader(data)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to write %s: %w", path, err)
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
