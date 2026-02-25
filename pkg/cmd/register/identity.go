package register

import (
	"path/filepath"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
)

const registrationFileName = "spark_registration.json"

// SparkRegistration is the persistent identity file for a registered DGX Spark.
// Fields align with the AddNodeResponse from dev-plane.
type SparkRegistration struct {
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
func SaveRegistration(brevHome string, reg *SparkRegistration) error {
	path := registrationPath(brevHome)
	err := files.OverwriteJSON(files.AppFs, path, reg)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

// LoadRegistration reads the registration from ~/.brev/spark_registration.json.
func LoadRegistration(brevHome string) (*SparkRegistration, error) {
	path := registrationPath(brevHome)
	var reg SparkRegistration
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
func RegistrationExists(brevHome string) bool {
	path := registrationPath(brevHome)
	exists, _ := files.AppFs.Stat(path)
	return exists != nil
}
