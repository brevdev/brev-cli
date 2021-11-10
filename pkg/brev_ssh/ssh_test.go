package brev_ssh

// Basic imports
import (
	"strings"
	"testing"

	"github.com/kevinburke/ssh_config"

	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type BrevSSHTestSuite struct {
	suite.Suite
	SSHConfig string
}

// Make sure that VariableThatShouldStartAtFive is set to five
// before each test
func (suite *BrevSSHTestSuite) SetupTest() {
	suite.SSHConfig = `
Host brev
	 Hostname 0.0.0.0
	 IdentityFile ~/.brev/brev.pem
	 User brev
	 Port 2222

Host workspace-images
	 Hostname 0.0.0.0
	 IdentityFile ~/.brev/brev.pem
	 User brev
	 Port 2223

Host brevdev/brev-deploy
	 Hostname 0.0.0.0
	 IdentityFile ~/.brev/brev.pem
	 User brev
	 Port 2224
`
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *BrevSSHTestSuite) TestGetBrevPorts() {
	cfg, err := ssh_config.Decode(strings.NewReader(suite.SSHConfig))
	if err != nil {
		panic(err)
	}
	ports, err := GetBrevPorts(*cfg, []string{"brev", "workspace-images", "brevdev/brev-deploy"})
	suite.Assert().True(ports["2222"])
	suite.Assert().True(ports["2223"])
	suite.Assert().True(ports["2224"])
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestExampleTestSuite(t *testing.T) {
	suite.Run(t, new(BrevSSHTestSuite))
}
