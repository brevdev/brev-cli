package config

import (
	"os"
)

type EnvVarName string // should be caps with underscore

const (
	brevAPIURL               EnvVarName = "BREV_API_URL"
	coordURL                 EnvVarName = "BREV_COORD_URL"
	version                  EnvVarName = "VERSION"
	clusterID                EnvVarName = "DEFAULT_CLUSTER_ID"
	defaultWorkspaceClass    EnvVarName = "DEFAULT_WORKSPACE_CLASS"
	defaultWorkspaceTemplate EnvVarName = "DEFAULT_WORKSPACE_TEMPLATE"
	sentryURL                EnvVarName = "DEFAULT_SENTRY_URL"
	debugHTTP                EnvVarName = "DEBUG_HTTP"
	ollamaAPIURL             EnvVarName = "OLLAMA_API_URL"
)

var ConsoleBaseURL = "https://brev.nvidia.com"

type ConstantsConfig struct{}

func NewConstants() *ConstantsConfig {
	return &ConstantsConfig{}
}

func (c ConstantsConfig) GetBrevAPIURl() string {
	return getEnvOrDefault(brevAPIURL, "https://brevapi.us-west-2-prod.control-plane.brev.dev")
}

func (c ConstantsConfig) GetOllamaAPIURL() string {
	return getEnvOrDefault(ollamaAPIURL, "https://registry.ollama.ai")
}

func (c ConstantsConfig) GetDefaultClusterID() string {
	return getEnvOrDefault(clusterID, "devplane-brev-1")
}

func (c ConstantsConfig) GetDefaultWorkspaceClass() string {
	return getEnvOrDefault(defaultWorkspaceClass, "")
}

func (c ConstantsConfig) GetDefaultWorkspaceTemplate() string {
	// "test-template-aws"
	return getEnvOrDefault(defaultWorkspaceTemplate, "")
}

func (c ConstantsConfig) GetDebugHTTP() bool {
	return getEnvOrDefault(debugHTTP, "") != ""
}

func getEnvOrDefault(envVarName EnvVarName, defaultVal string) string {
	val := os.Getenv(string(envVarName))
	if val == "" {
		return defaultVal
	}
	return val
}

var GlobalConfig = NewConstants()

type EnvVarConfig struct {
	ConstantsConfig
}

type FileConfig struct {
	EnvVarConfig
}

type FlagsConfig struct {
	FileConfig
}

type InitConfig interface{}

type AllConfig interface {
	InitConfig
	GetBrevAPIURl() string
	GetVersion() string
	GetDefaultClusterID() string
}
