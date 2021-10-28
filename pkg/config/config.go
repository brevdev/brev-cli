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

type Config struct{}

func NewConfig() *Config {
	_ = godotenv.Load(".env") // explicitly not handling error
	return &Config{}
}

func (c Config) GetBrevAPIURl() string {
	return getEnvOrDefault(brevAPIURL, "https://ade5dtvtaa.execute-api.us-east-1.amazonaws.com")
}

func (c Config) GetVersion() string {
	return getEnvOrDefault(version, "unknown")
}

func (c Config) GetDefaultClusterID() string {
	return getEnvOrDefault(clusterID, "k8s.brevstack.com")
}

func (c Config) GetKubeAPIURL() string {
	return getEnvOrDefault(k8APIURL, "https://api.k8s.brevstack.com")
}

func getEnvOrDefault(envVarName EnvVarName, defaultVal string) string {
	version := os.Getenv(string(envVarName))
	if version == "" {
		return defaultVal
	}
	return version
}

var GlobalConfig = NewConfig()
