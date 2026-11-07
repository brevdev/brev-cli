package config

import (
	"bytes"
	"strings"
	"testing"
)

func TestWarnEndpointOverrides(t *testing.T) {
	for _, name := range []EnvVarName{brevAPIURL, brevPublicAPIURL, brevAuthURL, brevAuthIssuerURL, brevConsoleURL} {
		t.Setenv(string(name), "")
	}
	var output bytes.Buffer
	WarnEndpointOverrides(&output)
	if output.Len() != 0 {
		t.Fatalf("default endpoints warned: %s", output.String())
	}
	t.Setenv(string(brevAPIURL), defaultBrevAPIURL)
	WarnEndpointOverrides(&output)
	if output.Len() != 0 {
		t.Fatalf("explicit default warned: %s", output.String())
	}
	for _, name := range []EnvVarName{brevAPIURL, brevPublicAPIURL, brevAuthURL, brevAuthIssuerURL, brevConsoleURL} {
		t.Setenv(string(name), "https://secret:password@custom.example/?token=private")
	}
	WarnEndpointOverrides(&output)
	warning := output.String()
	for _, name := range []EnvVarName{brevAPIURL, brevPublicAPIURL, brevAuthURL, brevAuthIssuerURL, brevConsoleURL} {
		if strings.Count(warning, string(name)) != 1 {
			t.Errorf("expected one warning for %s: %s", name, warning)
		}
	}
	if strings.Contains(warning, "secret") || strings.Contains(warning, "custom.example") || strings.Contains(warning, "private") {
		t.Fatalf("endpoint value leaked: %s", warning)
	}
	if got := (ConstantsConfig{}).GetBrevAPIURl(); got != "https://secret:password@custom.example/?token=private" {
		t.Fatalf("override changed: %s", got)
	}
}
