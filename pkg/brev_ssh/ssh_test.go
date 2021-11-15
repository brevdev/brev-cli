package brev_ssh

// Basic imports
import (
	"strings"
	"testing"

	"github.com/brevdev/brev-cli/pkg/brev_api"
	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/kevinburke/ssh_config"
	"github.com/spf13/afero"

	"github.com/stretchr/testify/suite"
)

var (
	noWorkspaces   = []brev_api.WorkspaceWithMeta{}
	someWorkspaces = []brev_api.WorkspaceWithMeta{
		{
			WorkspaceMetaData: brev_api.WorkspaceMetaData{},
			Workspace: brev_api.Workspace{
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
			WorkspaceMetaData: brev_api.WorkspaceMetaData{},
			Workspace: brev_api.Workspace{
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
			isBrevHost := checkIfBrevHost(*host, "/home/brev/.brev/brev.pem")
			suite.True(isBrevHost)
		}
	}
}

func (suite *BrevSSHTestSuite) TestPruneInactiveWorkspaces() {
	newConfig, err := PruneInactiveWorkspaces(&suite.SSHConfig, []string{"brev"})
	if !suite.Nil(err) {
		return
	}

	suite.Equal(`Host brev
  Hostname 0.0.0.0
  IdentityFile /home/brev/.brev/brev.pem
  User brev
  Port 2222
`, newConfig.String())
}

func (suite *BrevSSHTestSuite) TestAppendBrevEntry() {
	_, err := makeSSHEntry("bar", "2222")
	suite.Nil(err)
}

func (suite *BrevSSHTestSuite) TestCreateBrevSSHConfigEntries() {
	configFile, err := CreateBrevSSHConfigEntries(suite.SSHConfig, []string{"foo", "bar", "baz"})
	suite.Nil(err)
	templateLen := len(strings.Split(workspaceSSHConfigTemplate, "\n"))
	actualLen := len(strings.Split(configFile.String(), "\n"))
	suite.Greater(actualLen, (templateLen))
}

// TODO abstract out setup
// TODO add in more meaningful assertions
func (suite *BrevSSHTestSuite) TestConfigureSSH() {
	mfs := afero.NewMemMapFs()
	fs := store.NewBasicStore(*config.NewConstants()).WithFileSystem(mfs)
	err := afero.WriteFile(mfs, files.GetActiveOrgsPath(), []byte(`{"id":"ejmrvoj8m","name":"brev.dev"}`), 0644)
	p, err := files.GetUserSSHConfigPath()
	if !suite.Nil(err) {
		return
	}
	err = afero.WriteFile(mfs, *p, []byte(``), 0644)
	if !suite.Nil(err) {
		return
	}
	sshConfigurer := NewDefaultSSHConfigurer(noWorkspaces, fs, "lkjdflkj sld")
	err = sshConfigurer.Config()
	if !suite.Nil(err) {
		return
	}
}

func (suite *BrevSSHTestSuite) TestConfigureSSHWithActiveOrgs() {
	mfs := afero.NewMemMapFs()
	fs := store.NewBasicStore(*config.NewConstants()).WithFileSystem(mfs)
	err := afero.WriteFile(mfs, files.GetActiveOrgsPath(), []byte(`{"id":"ejmrvoj8m","name":"brev.dev"}`), 0644)
	p, err := files.GetUserSSHConfigPath()
	if !suite.Nil(err) {
		return
	}
	err = afero.WriteFile(mfs, *p, []byte(``), 0644)
	if !suite.Nil(err) {
		return
	}
	sshConfigurer := NewDefaultSSHConfigurer(someWorkspaces, fs, "lkjdflkj sld")
	err = sshConfigurer.Config()
	if !suite.Nil(err) {
		return
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSSH(t *testing.T) {
	suite.Run(t, new(BrevSSHTestSuite))
}
