package auth

import (
	"testing"

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

func TestSSH(t *testing.T) {
	suite.Run(t, new(BrevAPIAuthTestSuite))
}
