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
	return os.Getenv("BREV_API_URL")
}

func (c Config) GetVersion() string {
	return os.Getenv("VERSION")
}

var GlobalConfig = NewConfig()
