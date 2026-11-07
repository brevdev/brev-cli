package config

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type EnvVarName string // should be caps with underscore

const (
	brevAPIURL               EnvVarName = "BREV_API_URL"
	brevAuthURL              EnvVarName = "BREV_AUTH_URL"
	brevAuthIssuerURL        EnvVarName = "BREV_AUTH_ISSUER_URL"
	brevConsoleURL           EnvVarName = "BREV_CONSOLE_URL"
	coordURL                 EnvVarName = "BREV_COORD_URL"
	version                  EnvVarName = "VERSION"
	clusterID                EnvVarName = "DEFAULT_CLUSTER_ID"
	defaultWorkspaceTemplate EnvVarName = "DEFAULT_WORKSPACE_TEMPLATE"
	sentryURL                EnvVarName = "DEFAULT_SENTRY_URL"
	debugHTTP                EnvVarName = "DEBUG_HTTP"
	ollamaAPIURL             EnvVarName = "OLLAMA_API_URL"
	brevPublicAPIURL         EnvVarName = "BREV_PUBLIC_API_URL"
)

type ConstantsConfig struct{}

func NewConstants() *ConstantsConfig {
	return &ConstantsConfig{}
}

func (c ConstantsConfig) GetBrevAPIURl() string {
	return getEnvOrDefault(brevAPIURL, defaultBrevAPIURL)
}

// GetBrevPublicAPIURL returns the base URL of dev-plane's public API server
// (BREV_PUBLIC_API_URL). That server serves both the public HTTP routes (/v1/...)
// and the ConnectRPC services the CLI calls (pkg/cmd/register, pkg/environment).
func (c ConstantsConfig) GetBrevPublicAPIURL() string {
	return getEnvOrDefault(brevPublicAPIURL, defaultBrevPublicAPIURL)
}

func (c ConstantsConfig) GetBrevAuthURL() string {
	return getEnvOrDefault(brevAuthURL, defaultBrevAuthURL)
}

func (c ConstantsConfig) GetBrevAuthIssuerURL() string {
	return getEnvOrDefault(brevAuthIssuerURL, defaultBrevAuthIssuerURL)
}

func (c ConstantsConfig) GetConsoleURL() string {
	return getEnvOrDefault(brevConsoleURL, defaultBrevConsoleURL)
}

func (c ConstantsConfig) GetOllamaAPIURL() string {
	return getEnvOrDefault(ollamaAPIURL, "https://registry.ollama.ai")
}

func (c ConstantsConfig) GetDefaultClusterID() string {
	return getEnvOrDefault(clusterID, "devplane-brev-1")
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

const (
	defaultBrevAPIURL        = "https://brevapi.us-west-2-prod.control-plane.brev.dev"
	defaultBrevPublicAPIURL  = "https://api.brev.dev"
	defaultBrevAuthURL       = "https://api.ngc.nvidia.com"
	defaultBrevAuthIssuerURL = "https://login.nvidia.com"
	defaultBrevConsoleURL    = "https://brev.nvidia.com/cli-login"
)

// WarnEndpointOverrides reports custom authentication/API destinations without
// printing URLs, which may contain credentials or sensitive query parameters.
func WarnEndpointOverrides(w io.Writer) {
	var overridden []string
	for _, endpoint := range []struct {
		name       EnvVarName
		defaultURL string
	}{
		{brevAPIURL, defaultBrevAPIURL},
		{brevPublicAPIURL, defaultBrevPublicAPIURL},
		{brevAuthURL, defaultBrevAuthURL},
		{brevAuthIssuerURL, defaultBrevAuthIssuerURL},
		{brevConsoleURL, defaultBrevConsoleURL},
	} {
		value := os.Getenv(string(endpoint.name))
		if value != "" && value != endpoint.defaultURL {
			overridden = append(overridden, string(endpoint.name))
		}
	}
	if len(overridden) > 0 {
		_, _ = fmt.Fprintf(w, "Warning: non-default Brev endpoints configured via %s. Authentication and API traffic may be sent to custom servers.\n", strings.Join(overridden, ", "))
	}
}
