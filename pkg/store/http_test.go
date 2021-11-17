package store

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/stretchr/testify/assert"
)

func MakeMockNoHTTPStore() *NoAuthHTTPStore {
	fs := MakeMockFileStore()
	nh := fs.WithNoAuthHTTPClient(NewNoAuthHTTPClient(""))
	return nh
}

func TestWithHTTPClient(t *testing.T) {
	nh := MakeMockNoHTTPStore()
	if !assert.NotNil(t, nh) {
		return
	}
}

func MakeMockAuthHTTPStore() *AuthHTTPStore {
	nh := MakeMockNoHTTPStore()
	ah := nh.WithAuthHTTPClient(NewAuthHTTPClient(brevapi.TempAuth{}, ""))
	return ah
}

func TestWithAuthHTTPClient(t *testing.T) {
	ah := MakeMockAuthHTTPStore()
	if !assert.NotNil(t, ah) {
		return
	}
}

func TestNewNoAuthHTTPClient(t *testing.T) {
	n := NewNoAuthHTTPClient("")
	if !assert.NotNil(t, n) {
		return
	}
}

func TestNewAuthHTTPClient(t *testing.T) {
	n := NewAuthHTTPClient(brevapi.TempAuth{}, "")
	if !assert.NotNil(t, n) {
		return
	}
}
