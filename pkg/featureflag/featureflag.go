package featureflag

import (
	"strings"

	"github.com/brevdev/brev-cli/pkg/cmd/version"
	"github.com/spf13/viper"
)

func IsDev() bool {
	if viper.IsSet("feature.dev") {
		return viper.GetBool("feature.dev")
	} else {
		return strings.HasPrefix(version.Version, "dev")
	}
}

func DisableSSHProxyVersionCheck() bool {
	return viper.GetBool("feature.disable_ssh_proxy_version_check")
}

func ShowVersionOnRun() bool {
	return viper.GetBool("feature.show_version_on_run")
}

// todo set me via cli flag? this was meant to sort of like verbose but could be
// removed in favor of something like that
func Debug() bool {
	return viper.GetBool("feature.debug")
}

func LoadFeatureFlags(path string) error {
	viper.SetConfigName("config")
	viper.AddConfigPath("/etc/brev/")
	viper.AddConfigPath(path)
	viper.SetEnvPrefix("brev")
	viper.SetConfigType("yaml")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	_ = viper.ReadInConfig() // do not need to fail if can't find config file

	return nil
}
