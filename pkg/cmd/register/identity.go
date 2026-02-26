package register

import (
	"encoding/json"
	"os"
	"path/filepath"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/spf13/afero"
)

const registrationFileName = "spark_registration.json"

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

func registrationPath(brevHome string) string {
	return filepath.Join(brevHome, registrationFileName)
}

// SaveRegistration writes the registration to ~/.brev/spark_registration.json.
func SaveRegistration(brevHome string, reg *DeviceRegistration) error {
	path := registrationPath(brevHome)
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

// LoadRegistration reads the registration from ~/.brev/spark_registration.json.
func LoadRegistration(brevHome string) (*DeviceRegistration, error) {
	path := registrationPath(brevHome)
	var reg DeviceRegistration
	err := files.ReadJSON(files.AppFs, path, &reg)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &reg, nil
}

// DeleteRegistration removes ~/.brev/spark_registration.json.
func DeleteRegistration(brevHome string) error {
	path := registrationPath(brevHome)
	err := files.DeleteFile(files.AppFs, path)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

// RegistrationExists checks if a registration file exists.
// Returns (exists, error) so callers can distinguish "not found" from real errors.
func RegistrationExists(brevHome string) (bool, error) {
	path := registrationPath(brevHome)
	_, err := files.AppFs.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, breverrors.WrapAndTrace(err)
}
