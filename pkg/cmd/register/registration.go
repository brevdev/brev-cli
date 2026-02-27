package register

import (
	"encoding/json"
	"os"
	"path/filepath"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/spf13/afero"
)

const registrationFileName = "device_registration.json"

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

// FileRegistrationStore implements RegistrationStore using the local filesystem.
type FileRegistrationStore struct {
	brevHome string
}

// NewFileRegistrationStore returns a FileRegistrationStore rooted at brevHome.
func NewFileRegistrationStore(brevHome string) *FileRegistrationStore {
	return &FileRegistrationStore{brevHome: brevHome}
}

func (s *FileRegistrationStore) path() string {
	return filepath.Join(s.brevHome, registrationFileName)
}

func (s *FileRegistrationStore) Save(reg *DeviceRegistration) error {
	path := s.path()
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if err := files.AppFs.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if err := afero.WriteFile(files.AppFs, path, data, 0o600); err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (s *FileRegistrationStore) Load() (*DeviceRegistration, error) {
	path := s.path()
	var reg DeviceRegistration
	err := files.ReadJSON(files.AppFs, path, &reg)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &reg, nil
}

func (s *FileRegistrationStore) Delete() error {
	path := s.path()
	err := files.DeleteFile(files.AppFs, path)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
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
