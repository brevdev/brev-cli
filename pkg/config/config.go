// config
package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type configs struct{}

var config configs

// TODO add auth0 stuff here instead of being hardcoded in pkg/auth/auth.go
// Below vars are exposed to the build-layer (Makefile) so that they be overridden at build time.
// var err Error

// var (
// 	Version = os.Getenv("VERSION")
// 	BrevAPIEndpoint = os.Getenv("BREVAPIENDPOINT")
// )

// var (
// 	Version = os.Getenv("VERSION")
// 	BrevAPIEndpoint = os.Getenv("BREVAPIENDPOINT")
// )

func GetVersion() string {
	err := godotenv.Load(".env")
	if err != nil {
	  log.Fatal("Error loading .env file")
	}
	return 	os.Getenv("VERSION")

}

func GetBrevAPIEndpoint() string {
	err := godotenv.Load(".env")
	if err != nil {
	  log.Fatal("Error loading .env file")
	}
	return 	os.Getenv("BREVAPIENDPOINT")
}
