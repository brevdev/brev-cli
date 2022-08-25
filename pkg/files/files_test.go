package files

// Basic imports
import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type filesTestSuite struct {
	suite.Suite
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (s *filesTestSuite) TestGetUserSSHConfigPath() {
	home, _ := os.UserHomeDir()
	_, err := GetUserSSHConfigPath(home)
	s.Nil(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestFiles(t *testing.T) {
	suite.Run(t, new(filesTestSuite))
}
