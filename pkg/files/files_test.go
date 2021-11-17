package files

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type filesTestSuite struct {
	suite.Suite
}

func (s *filesTestSuite) TestGetUserSSHConfigPath() {
	_, err := GetUserSSHConfigPath()
	s.Nil(err)
}

func (s *filesTestSuite) TestGetNewBackupSSHConfigFilePath() {
	_, err := GetNewBackupSSHConfigFilePath()
	s.Nil(err)
}

func TestFiles(t *testing.T) {
	suite.Run(t, new(filesTestSuite))
}
