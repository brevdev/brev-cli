package store

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/jarcoal/httpmock"
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

type MockAuth struct{}

func (a MockAuth) GetAccessToken() (string, error) {
	return "token!", nil
}

func MakeMockAuthHTTPStore() *AuthHTTPStore {
	nh := MakeMockNoHTTPStore()
	ah := nh.WithAuthHTTPClient(NewAuthHTTPClient(MockAuth{}, ""))
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

func TestRetryAuthSuccess(t *testing.T) {
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	url := "/test"
	res := httpmock.NewStringResponder(403, "")
	httpmock.RegisterResponder("GET", url, res)

	calledTimes := 0
	s.SetOnAuthFailureHandler(func() error {
		calledTimes++
		res := httpmock.NewStringResponder(200, "")
		httpmock.RegisterResponder("GET", url, res)
		return nil
	})
	_, err := s.authHTTPClient.restyClient.R().Get(url)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.Equal(t, 1, calledTimes) {
		return
	}
}

func TestRetryAuthFailure(t *testing.T) {
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	url := "/test"
	res := httpmock.NewStringResponder(403, "")
	httpmock.RegisterResponder("GET", url, res)

	calledTimes := 0
	s.SetOnAuthFailureHandler(func() error {
		calledTimes++
		return nil
	})
	_, err := s.authHTTPClient.restyClient.R().Get(url)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.Equal(t, 1, calledTimes) {
		return
	}
}
