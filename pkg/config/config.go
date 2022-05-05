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
	segmentKey               EnvVarName = "DEFAULT_SEGMENT_KEY"
	sentryURL                EnvVarName = "DEFAULT_SENTRY_URL"
)

type ConstantsConfig struct{}

func NewConstants() *ConstantsConfig {
	return &ConstantsConfig{}
}

func (c ConstantsConfig) GetBrevAPIURl() string {
	return getEnvOrDefault(brevAPIURL, "https://brevapi.us-west-2-prod.control-plane.brev.dev")
}

func (c ConstantsConfig) GetServiceMeshCoordServerURL() string {
	return getEnvOrDefault(coordURL, "")
}

func (c ConstantsConfig) GetVersion() string {
	return getEnvOrDefault(version, "unknown")
}

func (c ConstantsConfig) GetDefaultClusterID() string {
	return getEnvOrDefault(clusterID, "")
}

func (c ConstantsConfig) GetDefaultWorkspaceClass() string {
	return getEnvOrDefault(defaultWorkspaceClass, "")
}

func (c ConstantsConfig) GetDefaultWorkspaceTemplate() string {
	// "test-template-aws"
	return getEnvOrDefault(defaultWorkspaceTemplate, "")
}

func (c ConstantsConfig) GetSegmentKey() string {
	return getEnvOrDefault(segmentKey, "4RndpJDxkZViNDMSjMaSxb9IANpqkV4X")
}

func (c ConstantsConfig) GetSentryURL() string {
	return getEnvOrDefault(sentryURL, "https://4f3dca96f17e4c7995588dda4a31b37f@o410659.ingest.sentry.io/6383105")
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

type InitConfig interface{}

type AllConfig interface {
	InitConfig
	GetBrevAPIURl() string
	GetVersion() string
	GetDefaultClusterID() string
}
