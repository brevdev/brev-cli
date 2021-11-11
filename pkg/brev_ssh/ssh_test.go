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
	SSHConfig        ssh_config.Config
	activeWorkspaces []string
}

func (suite *BrevSSHTestSuite) SetupTest() {
	r := strings.NewReader(`Host brev
	 Hostname 0.0.0.0
	 IdentityFile /home/brev/.brev/brev.pem
	 User brev
	 Port 2222

Host workspace-images
	 Hostname 0.0.0.0
	 IdentityFile /home/brev/.brev/brev.pem
	 User brev
	 Port 2223

Host brevdev/brev-deploy
	 Hostname 0.0.0.0
	 IdentityFile /home/brev/.brev/brev.pem
	 User brev
	 Port 2224`)
	SSHConfig, err := ssh_config.Decode(r)
	if err != nil {
		panic(err)
	}
	suite.SSHConfig = *SSHConfig
	suite.activeWorkspaces = []string{"brev", "workspace-images"}
}

func (suite *BrevSSHTestSuite) TestGetBrevPorts() {
	ports, err := GetBrevPorts(suite.SSHConfig, []string{"brev", "workspace-images", "brevdev/brev-deploy"})
	suite.True(ports["2222"])
	suite.True(ports["2223"])
	suite.True(ports["2224"])
	suite.Nil(err)
}

func (suite *BrevSSHTestSuite) TestCheckIfBrevHost() {
	for _, host := range suite.SSHConfig.Hosts {
		if len(host.Nodes) > 0 {
			isBrevHost := checkIfBrevHost(*host)
			suite.True(isBrevHost)
		}
	}
}

func (suite *BrevSSHTestSuite) TestFilterExistingWorkspaceNames() {
	workspaceNames, existingNames := filterExistingWorkspaceNames(suite.activeWorkspaces, suite.SSHConfig)
	suite.ElementsMatch([]string{"brev", "workspace-images"}, workspaceNames)
	suite.ElementsMatch([]string{"brevdev/brev-deploy"}, existingNames)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestExampleTestSuite(t *testing.T) {
	suite.Run(t, new(BrevSSHTestSuite))
}
