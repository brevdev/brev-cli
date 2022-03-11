package featureflag

import (
	"os"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/version"
)

func IsDev() bool {
	return os.Getenv("BREV_FEATURE_FLAG_DEV") == "dev" || version.Version == "" || strings.HasPrefix(version.Version, "dev")
}
