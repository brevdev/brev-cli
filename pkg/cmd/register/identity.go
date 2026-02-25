package register

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
)

const registrationFileName = "spark_registration.json"

// SparkRegistration is the persistent identity file for a registered DGX Spark.
type SparkRegistration struct {
	BrevCloudNodeID       string          `json:"brev_cloud_node_id"`
	DisplayName           string          `json:"display_name"`
	OrgID                 string          `json:"org_id"`
	HardwareFingerprint   string          `json:"hardware_fingerprint"`
	DeviceFingerprintHash string          `json:"device_fingerprint_hash"`
	RegisteredAt          string          `json:"registered_at"`
	HardwareProfile       HardwareProfile `json:"hardware_profile"`
}

// HardwareDescriptor captures high-level hardware traits for fingerprinting.
// Must produce byte-identical JSON to dev-plane's HardwareDescriptor.
type HardwareDescriptor struct {
	GPUs []GPUDescriptor `json:"gpus"`
	RAM  int64           `json:"ram_bytes"`
	CPUs int             `json:"cpus"`
}

// GPUDescriptor describes a single GPU for fingerprinting.
type GPUDescriptor struct {
	Model  string `json:"model"`
	Memory int64  `json:"memory_bytes"`
}

// ComputeHardwareFingerprint returns a deterministic SHA-256 fingerprint.
// This must produce identical output to dev-plane's ComputeHardwareFingerprint
// for the same input.
func ComputeHardwareFingerprint(desc HardwareDescriptor) (string, error) {
	if desc.CPUs < 1 {
		return "", fmt.Errorf("CPUs must be at least 1")
	}
	if desc.RAM < 1 {
		return "", fmt.Errorf("RAM must be at least 1")
	}
	for _, gpu := range desc.GPUs {
		if gpu.Model == "" {
			return "", fmt.Errorf("GPU model must not be empty")
		}
		if gpu.Memory < 1 {
			return "", fmt.Errorf("GPU memory must be at least 1")
		}
	}

	// Sort GPUs by model then memory for stable ordering.
	gpus := make([]GPUDescriptor, len(desc.GPUs))
	copy(gpus, desc.GPUs)
	sort.Slice(gpus, func(i, j int) bool {
		if gpus[i].Model == gpus[j].Model {
			return gpus[i].Memory < gpus[j].Memory
		}
		return gpus[i].Model < gpus[j].Model
	})
	desc.GPUs = gpus

	payload, err := json.Marshal(desc)
	if err != nil {
		return "", breverrors.WrapAndTrace(err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}

// HardwareProfileToDescriptor converts a HardwareProfile to a HardwareDescriptor
// for fingerprinting.
func HardwareProfileToDescriptor(p *HardwareProfile) HardwareDescriptor {
	desc := HardwareDescriptor{
		RAM:  p.RAMBytes,
		CPUs: p.CPUCount,
	}
	for _, gpu := range p.GPUs {
		desc.GPUs = append(desc.GPUs, GPUDescriptor{
			Model:  gpu.Name,
			Memory: gpu.MemoryMB * 1024 * 1024, // convert MB to bytes
		})
	}
	return desc
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
