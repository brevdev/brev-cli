package config

type configs struct{}

var config configs

// TODO add auth0 stuff here instead of being hardcoded in pkg/auth/auth.go
// Below vars are exposed to the build-layer (Makefile) so that they be overridden at build time.
var (
	Version         = "unknown"
	BrevAPIEndpoint = `https://ade5dtvtaa.execute-api.us-east-1.amazonaws.com`
	// BrevAPIEndpoint = `http://localhost:8080`
)

func Init() {
	config = configs{}
}

func GetVersion() string {
	return Version
}

func GetBrevAPIEndpoint() string {
	return BrevAPIEndpoint
}
