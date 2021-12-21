package featureflag

import (
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/version"
)

func IsDev() bool {
	return version.Version == "" || strings.HasPrefix(version.Version, "dev")
}
