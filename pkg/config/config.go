// config
package config

import (
	"os"

	"github.com/joho/godotenv"
)

type EnvVarName string // should be caps with underscore

const (
	brevAPIURL EnvVarName = "BREV_API_URL"
	version    EnvVarName = "VERSION"
	clusterID  EnvVarName = "DEFAULT_CLUSTER_ID"
	k8APIURL   EnvVarName = "K8_API_URL"
)

type ConstantsConfig struct{}

func NewConstants() *ConstantsConfig {
	_ = godotenv.Load(".env") // explicitly not handling error
	return &ConstantsConfig{}
}

func (c ConstantsConfig) GetBrevAPIURl() string {
	return getEnvOrDefault(brevAPIURL, "https://ade5dtvtaa.execute-api.us-east-1.amazonaws.com")
}

func (c ConstantsConfig) GetVersion() string {
	return getEnvOrDefault(version, "unknown")
}

func (c ConstantsConfig) GetDefaultClusterID() string {
	return getEnvOrDefault(clusterID, "k8s.brevstack.com")
}

func getEnvOrDefault(envVarName EnvVarName, defaultVal string) string {
	version := os.Getenv(string(envVarName))
	if version == "" {
		return defaultVal
	}
	return version
}

var GlobalConfig = NewConstants()

type EnvVarConfig struct {
	ConstantsConfig
}

func (c *ConstantsConfig) WithEnvVars() *EnvVarConfig {
	return &EnvVarConfig{*c}
}

type FileConfig struct {
	EnvVarConfig
}

func (c *EnvVarConfig) WithFileConfig() *FileConfig {
	return &FileConfig{*c}
}

type FlagsConfig struct {
	FileConfig
}

func (c *FileConfig) WithFlags() *FlagsConfig {
	return &FlagsConfig{*c}
}
