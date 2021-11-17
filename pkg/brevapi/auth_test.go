package brevapi

import (
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/suite"
)

type BrevAPIAuthTestSuite struct {
	suite.Suite
	fs afero.Fs
}

func (s *BrevAPIAuthTestSuite) SetupTest() {
	credentialFileContentes := `{"access_token":"act","expires_in":86400,"id_token":"idt","refresh_token":"rft"}`
	mmfs := afero.NewMemMapFs()
	credentialFilePath, err := getBrevCredentialsFile()
	if err != nil {
		panic(err)
	}
	err = afero.WriteFile(mmfs, *credentialFilePath, []byte(credentialFileContentes), 0o644)
	if err != nil {
		panic(err)
	}
	s.fs = mmfs
}

func (s *BrevAPIAuthTestSuite) TestGetTokenFromBrevConfigFile() {
	token, err := getTokenFromBrevConfigFile(s.fs)
	s.Nil(err)
	s.Equal(token.AccessToken, "act")
	s.Equal(token.IDToken, "idt")
	s.Equal(token.RefreshToken, "rft")
	s.Equal(token.ExpiresIn, 86400)
}

func TestSSH(t *testing.T) {
	suite.Run(t, new(BrevAPIAuthTestSuite))
}
