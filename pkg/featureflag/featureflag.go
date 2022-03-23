package featureflag

import (
	"fmt"
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/version"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/spf13/viper"
)

func IsDev() bool {
	if viper.IsSet("feature.dev") {
		return viper.GetBool("feature.dev")
	} else {
		return strings.HasPrefix(version.Version, "dev")
	}
}

func IsAdmin(userType string) bool {
	if viper.IsSet("feature.not_admin") && viper.GetBool("feature.not_admin") {
		return false
	} else {
		return userType == "Admin"
	}
}

func ServiceMeshSSH() bool {
	return viper.GetBool("feature.service_mesh_ssh")
}

func LoadFeatureFlags(path string) error {
	viper.SetConfigName("config")
	if path == "" {
		viper.AddConfigPath("$HOME/.brev")
		viper.AddConfigPath(".")
	} else {
		fmt.Println(path)
		viper.AddConfigPath(path)
	}
	viper.SetEnvPrefix("brev")
	viper.SetConfigType("yaml")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return breverrors.WrapAndTrace(err)
	}

	return nil
}
