// config
package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct{}

func NewConfig() *Config {
	_ = godotenv.Load(".env") // explicitly not handling error
	return &Config{}
}

func (c Config) GetBrevAPIURl() string {
	url := os.Getenv("BREV_API_URL")
	if url == "" {
		return "https://ade5dtvtaa.execute-api.us-east-1.amazonaws.com"
	}
	return os.Getenv("BREV_API_URL")
}

func (c Config) GetVersion() string {
	version := os.Getenv("VERSION")

	if version == "" {
		return "unknown"
	}
	return os.Getenv("VERSION")
}

var GlobalConfig = NewConfig()
