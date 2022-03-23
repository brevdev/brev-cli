package featureflag

import (
	"os"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/version"
)

func IsDev() bool {
	if os.Getenv("BREV_FEATURE_FLAG_DEV") != "" {
		return os.Getenv("BREV_FEATURE_FLAG_DEV") == "1"
	}
	return version.Version == "" || strings.HasPrefix(version.Version, "dev")
}

func IsAdmin(userType string) bool {
	if os.Getenv("BREV_FEATURE_NOT_ADMIN") == "1" {
		return false
	}
	return userType == "Admin"
}

func ServiceMeshSSH() bool {
	if os.Getenv("BREV_FEATURE_FLAG_SERVICE_MESH_SSH") != "" {
		return os.Getenv("BREV_FEATURE_FLAG_SERVICE_MESH_SSH") == "1"
	} else {
		return false
	}
}
