package store

import (
	"net/http"
	"strings"
	"testing"

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

type MockAuth struct{ token *string }

func (a MockAuth) GetAccessToken() (string, error) {
	if a.token == nil {
		return "mock-token", nil
	} else {
		return *a.token, nil
	}
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
	n := NewAuthHTTPClient(MockAuth{}, "")
	if !assert.NotNil(t, n) {
		return
	}
}

func makeCheckTokenResponder(validToken string) httpmock.Responder {
	return func(r *http.Request) (*http.Response, error) {
		h := r.Header.Get("Authorization")
		splitStr := strings.Split(h, "Bearer")
		if len(splitStr) != 2 {
			return &http.Response{StatusCode: 403}, nil
		}
		if strings.TrimSpace(splitStr[1]) == validToken {
			return &http.Response{StatusCode: 200}, nil
		} else {
			return &http.Response{StatusCode: 403}, nil
		}
	}
}

func TestRetryAuthSuccess(t *testing.T) {
	nh := MakeMockNoHTTPStore()
	invalidToken := "invalid-token"
	validToken := "valid-token"
	s := nh.WithAuthHTTPClient(NewAuthHTTPClient(MockAuth{&invalidToken}, ""))

	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	url := "/test"
	responder := makeCheckTokenResponder(validToken)
	httpmock.RegisterResponder("GET", url, responder)

	calledTimes := 0
	err := s.SetForbiddenStatusRetryHandler(func() error {
		invalidToken = validToken
		calledTimes++
		return nil
	})
	if !assert.Nil(t, err) {
		return
	}
	r, err := s.authHTTPClient.restyClient.R().Get(url)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.Equal(t, 200, r.StatusCode()) {
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
	err := s.SetForbiddenStatusRetryHandler(func() error {
		calledTimes++
		return nil
	})
	if !assert.Nil(t, err) {
		return
	}
	_, err = s.authHTTPClient.restyClient.R().Get(url)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.Equal(t, 1, calledTimes) {
		return
	}
}
