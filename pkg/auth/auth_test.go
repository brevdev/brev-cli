package auth

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type BrevAPIAuthTestSuite struct {
	suite.Suite
}

func (s *BrevAPIAuthTestSuite) SetupTest() {
}

func TestIsAccessTokenValid(t *testing.T) {
	invalidToken := "blah"
	res, err := isAccessTokenValid(invalidToken)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.False(t, res) {
		return
	}

	// expiredToken := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCIsImtpZCI6ImdLTXBESXlRc0ZXSF9zYWdiT2oyViJ9.eyJpc3MiOiJodHRwczovL2JyZXZkZXYudXMuYXV0aDAuY29tLyIsInN1YiI6Imdvb2dsZS1vYXV0aDJ8MTAxNzY0NjMwNTEwODYxNDk5MTgwIiwiYXVkIjpbImh0dHBzOi8vYnJldmRldi51cy5hdXRoMC5jb20vYXBpL3YyLyIsImh0dHBzOi8vYnJldmRldi51cy5hdXRoMC5jb20vdXNlcmluZm8iXSwiaWF0IjoxNjM4NTYyMzY4LCJleHAiOjE2Mzg2NDg3NjgsImF6cCI6IkphcUpSTEVzZGF0NXc3VGIwV3FtVHh6SWVxd3FlcG1rIiwic2NvcGUiOiJvcGVuaWQgcHJvZmlsZSBlbWFpbCBvZmZsaW5lX2FjY2VzcyJ9.YCiO-som26ehT91qGAX5ZfrtVg4eYwamnlMRoCuUljXmg8Nf-ArDyoG32CqZkQ6YJ5XnzrVX9bVk5ZNHP_AFSE9SJvYL6MchoN09nR84WTbevRBCtZedIZUk5ULg6rWo5mszGr-S2gi08od4iTzXtKySPx1JnT60muRj_k9VV3MyixqvngEz5NvmFDdA8glGes5_iOuiBidmjOJzi_CVfKJ9s48BhlxzciSXFC0_DUBnT9OThjYjUP-22ohOuWwJWomRUv6gMSq78hJOALc330LwvmEsLdzlP7a3otIYM43hTtAVJ9QEL6M08GKqm3PdikzTxiGdfuQUhgMDlXygbQ"
	// res, err := isAccessTokenValid(expiredToken)
	// if !assert.Nil(t, err) {
	// 	return
	// }
	// if !assert.False(t, res) {
	// 	return
	// }
}

type MockAuthStore struct {
	authTokens *entity.AuthTokens
	didSave    bool
}

func (m *MockAuthStore) SaveAuthTokens(_ entity.AuthTokens) error {
	m.didSave = true
	return nil
}

func (m MockAuthStore) GetAuthTokens() (*entity.AuthTokens, error) {
	return m.authTokens, nil
}

func (m MockAuthStore) DeleteAuthTokens() error {
	return nil
}

type MockOauth struct {
	authTokens  *entity.AuthTokens
	loginTokens *LoginTokens
	flowDone    bool
}

func (m *MockOauth) DoDeviceAuthFlow(_ func(string, string)) (*LoginTokens, error) {
	m.flowDone = true
	return m.loginTokens, nil
}

func (m MockOauth) GetNewAuthTokensWithRefresh(_ string) (*entity.AuthTokens, error) {
	return m.authTokens, nil
}

const validToken = "abc"

func TestSuccessNoRefreshGetFreshAccessTokenOrLogin(t *testing.T) {
	s := MockAuthStore{authTokens: &entity.AuthTokens{
		AccessToken:  validToken,
		RefreshToken: "",
	}}
	a := Auth{
		&s,
		&MockOauth{}, func(s string) (bool, error) {
			return true, nil
		},
		func() (bool, error) {
			return true, nil
		},
	}
	res, err := a.GetFreshAccessTokenOrLogin()
	if !assert.Nil(t, err) {
		return
	}
	if !assert.Equal(t, validToken, res) {
		return
	}
	if !assert.False(t, s.didSave) {
		return
	}
}

func TestSuccessRefreshGetFreshAccessTokenOrLogin(t *testing.T) {
	s := MockAuthStore{authTokens: &entity.AuthTokens{
		AccessToken:  "bad",
		RefreshToken: "ref",
	}}
	a := Auth{
		&s, &MockOauth{
			authTokens: &entity.AuthTokens{
				AccessToken:  validToken,
				RefreshToken: "",
			},
			loginTokens: &LoginTokens{},
		}, func(s string) (bool, error) {
			return false, nil
		}, func() (bool, error) {
			return true, nil
		},
	}
	res, err := a.GetFreshAccessTokenOrLogin()
	if !assert.Nil(t, err) {
		return
	}
	if !assert.Equal(t, validToken, res) {
		return
	}
	if !assert.True(t, s.didSave) {
		return
	}
}

func TestTokenDoesNotExistGetFreshAccessTokenOrLogin(t *testing.T) {
	o := MockOauth{
		authTokens: &entity.AuthTokens{},
		loginTokens: &LoginTokens{
			AuthTokens: entity.AuthTokens{
				AccessToken:  validToken,
				RefreshToken: "",
			},
			IDToken: "",
		},
	}
	s := MockAuthStore{
		authTokens: nil,
	}
	a := Auth{
		&s, &o, func(s string) (bool, error) {
			return false, nil
		}, func() (bool, error) {
			return true, nil
		},
	}
	res, err := a.GetFreshAccessTokenOrLogin()
	if !assert.Nil(t, err) {
		return
	}
	if !assert.Equal(t, validToken, res) {
		return
	}
	if !assert.True(t, o.flowDone) {
		return
	}
	if !assert.True(t, s.didSave) {
		return
	}
}

func TestDenyLoginGetFreshAccessTokenOrLogin(t *testing.T) {
	s := MockAuthStore{
		authTokens: nil,
	}
	o := MockOauth{
		authTokens: &entity.AuthTokens{},
		loginTokens: &LoginTokens{
			AuthTokens: entity.AuthTokens{
				AccessToken:  "",
				RefreshToken: "",
			},
			IDToken: "",
		},
	}
	a := Auth{
		&s, &o, func(s string) (bool, error) {
			return false, nil
		}, func() (bool, error) {
			return false, nil
		},
	}
	res, err := a.GetFreshAccessTokenOrLogin()
	de := &breverrors.DeclineToLoginError{}
	if !assert.ErrorAs(t, err, &de) {
		return
	}
	if !assert.Empty(t, res) {
		return
	}
	if !assert.False(t, o.flowDone) {
		return
	}
	if !assert.False(t, s.didSave) {
		return
	}
}

func TestFailedRefreshGetFreshAccessTokenOrLogin(t *testing.T) {
	a := Auth{
		&MockAuthStore{
			authTokens: &entity.AuthTokens{
				AccessToken:  "invalid",
				RefreshToken: "",
			},
		}, &MockOauth{
			authTokens: nil,
			loginTokens: &LoginTokens{
				AuthTokens: entity.AuthTokens{
					AccessToken:  validToken,
					RefreshToken: "",
				},
				IDToken: "",
			},
		}, func(s string) (bool, error) {
			return false, nil
		}, func() (bool, error) {
			return true, nil
		},
	}
	res, err := a.GetFreshAccessTokenOrLogin()
	if !assert.Nil(t, err) {
		return
	}
	if !assert.Equal(t, validToken, res) {
		return
	}
}

func TestSSH(t *testing.T) {
	suite.Run(t, new(BrevAPIAuthTestSuite))
}
