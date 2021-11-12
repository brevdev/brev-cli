package brev_ssh

// Basic imports
import (
	"strings"
	"testing"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/kevinburke/ssh_config"
	"github.com/spf13/afero"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

var MemAppFs = afero.NewMemMapFs()

type MockWorkspaceGetter struct {
	mock.Mock
}

func (m *MockWorkspaceGetter) GetMyWorkspaces(orgID string) ([]brev_api.Workspace, error) {
	return []brev_api.Workspace{}, nil
}

func (m *MockWorkspaceGetter) GetWorkspaceMetaData(wsID string) (*brev_api.WorkspaceMetaData, error) {
	return &brev_api.WorkspaceMetaData{}, nil
}

type MockWorkspaceGetterActiveOrgs struct {
	mock.Mock
}

func (m *MockWorkspaceGetterActiveOrgs) GetMyWorkspaces(orgID string) ([]brev_api.Workspace, error) {
	return []brev_api.Workspace{
		{
			ID:               "foo",
			Name:             "testWork",
			WorkspaceGroupID: "lkj",
			OrganizationID:   "lkjlasd",
			WorkspaceClassID: "lkjas'lkf",
			CreatedByUserID:  "lkasfjas",
			DNS:              "lksajdalk",
			Status:           "lkjgdflk",
			Password:         "sdfal",
			GitRepo:          "lkdfjlksadf",
		},
	}, nil
}

func (m *MockWorkspaceGetterActiveOrgs) GetWorkspaceMetaData(wsID string) (*brev_api.WorkspaceMetaData, error) {
	return &brev_api.WorkspaceMetaData{}, nil
}

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type BrevSSHTestSuite struct {
	suite.Suite
	SSHConfig ssh_config.Config
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
	suite.SSHConfig.Hosts = suite.SSHConfig.Hosts[1:]
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

func (suite *BrevSSHTestSuite) TestPruneInactiveWorkspaces() {
	newConfig := PruneInactiveWorkspaces(suite.SSHConfig, []string{"brev"})

	suite.Equal(`Host brev
  Hostname 0.0.0.0
  IdentityFile /home/brev/.brev/brev.pem
  User brev
  Port 2222
`, newConfig)
}

func (suite *BrevSSHTestSuite) TestAppendBrevEntry() {
	_, err := makeSSHEntry("bar", "2222")
	suite.Nil(err)
}

func (suite *BrevSSHTestSuite) TestCreateBrevSSHConfigEntries() {
	configFile, err := CreateBrevSSHConfigEntries(suite.SSHConfig, []string{"foo", "bar", "baz"})
	suite.Nil(err)
	templateLen := len(strings.Split(workspaceSSHConfigTemplate, "\n"))
	actualLen := len(strings.Split(configFile, "\n"))
	suite.Greater(actualLen, (templateLen))
}

func (suite *BrevSSHTestSuite) TestConfigureSSH() {
	err := afero.WriteFile(MemAppFs, files.GetActiveOrgsPath(), []byte(`{"id":"ejmrvoj8m","name":"brev.dev"}`), 0644)
	if err != nil {
		panic(err)
	}
	workspaceGetter := new(MockWorkspaceGetter)
	sshConfigurer := NewDefaultSSHConfigurer(workspaceGetter, "lkjdflkj sld", func(fs afero.Fs) (*brev_api.Organization, error) {
		return &brev_api.Organization{}, nil
	})
	sshConfigurer.WithFS(MemAppFs)
	err = sshConfigurer.Config()
	suite.Nil(err)
}

func (suite *BrevSSHTestSuite) TestConfigureSSHWithActiveOrgs() {
	err := afero.WriteFile(MemAppFs, files.GetActiveOrgsPath(), []byte(`{"id":"ejmrvoj8m","name":"brev.dev"}`), 0644)
	if err != nil {
		panic(err)
	}
	workspaceGetter := new(MockWorkspaceGetterActiveOrgs)
	sshConfigurer := NewDefaultSSHConfigurer(workspaceGetter, "lkjdflkj sld", func(fs afero.Fs) (*brev_api.Organization, error) {
		return &brev_api.Organization{}, nil
	})
	sshConfigurer.WithFS(MemAppFs)
	err = sshConfigurer.Config()
	suite.Nil(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSSH(t *testing.T) {
	suite.Run(t, new(BrevSSHTestSuite))
}
