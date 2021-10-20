package config

type configs struct{}

var config configs

// Below vars are exposed to the build-layer (Makefile) so that they be overridden at build time.
var (
	Version         = "unknown"
	CotterAPIKey    = "unknown"
	BrevALPHAAPIEndpoint = "https://app.brev.dev"
	BrevAPIEndpoint = `https://ade5dtvtaa.execute-api.us-east-1.amazonaws.com`;

)

func Init() {
	config = configs{}
}

func GetVersion() string {
	return Version
}

func GetCotterAPIKey() string {
	return CotterAPIKey
}

func GetBrevALPHAAPIEndpoint() string {
	return BrevALPHAAPIEndpoint
}

func GetBrevAPIEndpoint() string {
	return BrevAPIEndpoint
}
