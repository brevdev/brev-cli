package brevssh

// Basic imports
import (
	"fmt"
	"strings"
	"testing"

	"github.com/brevdev/brev-cli/pkg/brevapi"
	"github.com/brevdev/brev-cli/pkg/config"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/kevinburke/ssh_config"
	"github.com/spf13/afero"

	"github.com/stretchr/testify/suite"
)

var (
	noWorkspaces   = []brevapi.WorkspaceWithMeta{}
	someWorkspaces = []brevapi.WorkspaceWithMeta{
		{
			WorkspaceMetaData: brevapi.WorkspaceMetaData{},
			Workspace: brevapi.Workspace{
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
		},
		{
			WorkspaceMetaData: brevapi.WorkspaceMetaData{},
			Workspace: brevapi.Workspace{
				ID:               "bar",
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
		},
	}
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type BrevSSHTestSuite struct {
	suite.Suite
	SSHConfig ssh_config.Config
	store     SSHStore
}

var userConfigStr = `Host user-host
Hostname 172.0.0.0
`

func (suite *BrevSSHTestSuite) SetupTest() {
	s, err := makeMockSSHStore()
	if !suite.Nil(err) {
		return
	}
	suite.store = s

	r := strings.NewReader(fmt.Sprintf(`%[2]s
Host brev
	 Hostname 0.0.0.0
	 IdentityFile %[1]s
	 User brev
	 Port 2222
Host workspace-images
	 Hostname 0.0.0.0
	 IdentityFile %[1]s
	 User brev
	 Port 2223
Host brevdev/brev-deploy
	 Hostname 0.0.0.0
	 IdentityFile %[1]s
	 User brev
	 Port 2224`, s.GetPrivateKeyFilePath(), userConfigStr))
	SSHConfig, err := ssh_config.Decode(r)
	if err != nil {
		panic(err)
	}
	suite.SSHConfig = *SSHConfig
}

func (suite *BrevSSHTestSuite) TestGetBrevPorts() {
	ports, err := GetBrevPorts(suite.SSHConfig, []string{"brev", "workspace-images", "brevdev/brev-deploy"})
	suite.True(ports["2222"])
	suite.True(ports["2223"])
	suite.True(ports["2224"])
	suite.Nil(err)
}

func (suite *BrevSSHTestSuite) TestCheckIfBrevHost() {
	for _, host := range suite.SSHConfig.Hosts[2:] {
		if len(host.Nodes) > 0 {
			isBrevHost := checkIfBrevHost(*host, suite.store.GetPrivateKeyFilePath())
			suite.True(isBrevHost)
		}
	}
}

func (suite *BrevSSHTestSuite) TestPruneInactiveWorkspaces() {
	s, err := makeMockSSHStore()
	if !suite.Nil(err) {
		return
	}

	newConfig, err := DefaultSSHConfigurer{sshStore: s}.PruneInactiveWorkspaces(&suite.SSHConfig, []string{"brev"})
	if !suite.Nil(err) {
		return
	}

	configStr := newConfig.String()

	suite.Equal(fmt.Sprintf(`%s
Host brev
  Hostname 0.0.0.0
  IdentityFile %s
  User brev
  Port 2222
`, userConfigStr, s.GetPrivateKeyFilePath()), configStr)
}

func (suite *BrevSSHTestSuite) TestAppendBrevEntry() {
	s, err := makeMockSSHStore()
	if !suite.Nil(err) {
		return
	}

	_, err = DefaultSSHConfigurer{sshStore: s}.makeSSHEntry("bar", "2222")
	if !suite.Nil(err) {
		return
	}
}

func (suite *BrevSSHTestSuite) TestCreateBrevSSHConfigEntries() {
	s, err := makeMockSSHStore()
	if !suite.Nil(err) {
		return
	}

	configFile, err := DefaultSSHConfigurer{sshStore: s}.CreateBrevSSHConfigEntries(suite.SSHConfig, []string{"foo", "bar", "baz"})
	if !suite.Nil(err) {
		return
	}

	templateLen := len(strings.Split(workspaceSSHConfigTemplate, "\n"))
	actualLen := len(strings.Split(configFile.String(), "\n"))
	suite.Greater(actualLen, (templateLen))
}

// TODO abstract out setup
// TODO add in more meaningful assertions
func (suite *BrevSSHTestSuite) TestConfigureSSH() {
	s, err := makeMockSSHStore()
	if !suite.Nil(err) {
		return
	}
	sshConfigurer := NewDefaultSSHConfigurer(noWorkspaces, s, "lkjdflkj sld")
	err = sshConfigurer.Config()
	if !suite.Nil(err) {
		return
	}
}

func (suite *BrevSSHTestSuite) TestConfigureSSHWithActiveOrgs() {
	s, err := makeMockSSHStore()
	if !suite.Nil(err) {
		return
	}
	sshConfigurer := NewDefaultSSHConfigurer(someWorkspaces, s, "lkjdflkj sld")
	err = sshConfigurer.Config()
	if !suite.Nil(err) {
		return
	}
}

func makeMockSSHStore() (SSHStore, error) {
	mfs := afero.NewMemMapFs()
	fs := store.NewBasicStore(*config.NewConstants()).WithFileSystem(mfs)
	err := afero.WriteFile(mfs, files.GetActiveOrgsPath(), []byte(`{"id":"ejmrvoj8m","name":"brev.dev"}`), 0644)
	p, err := files.GetUserSSHConfigPath()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	err = afero.WriteFile(mfs, *p, []byte(``), 0644)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return fs, nil
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSSH(t *testing.T) {
	suite.Run(t, new(BrevSSHTestSuite))
}
