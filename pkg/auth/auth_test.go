package auth

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type BrevAPIAuthTestSuite struct {
	suite.Suite
}

func (s *BrevAPIAuthTestSuite) SetupTest() {
}

func TestSSH(t *testing.T) {
	suite.Run(t, new(BrevAPIAuthTestSuite))
}
