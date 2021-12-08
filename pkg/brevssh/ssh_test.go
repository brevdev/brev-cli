package brevssh

import (
	"fmt"
	"strings"
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

var (
	noWorkspaces   = []entity.WorkspaceWithMeta{}
	someWorkspaces = []entity.WorkspaceWithMeta{
		{
			WorkspaceMetaData: entity.WorkspaceMetaData{},
			Workspace: entity.Workspace{
				ID:               "test-id",
				Name:             "testName",
				WorkspaceGroupID: "wgi",
				OrganizationID:   "oi",
				WorkspaceClassID: "wci",
				CreatedByUserID:  "cui",
				DNS:              "test-dns.brev.sh",
				Status:           "RUNNING",
				Password:         "sdfal",
				GitRepo:          "gitrepo",
			},
		},
	}
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type BrevSSHTestSuite struct {
	suite.Suite
	store      store.FileStore
	Configurer *DefaultSSHConfigurer
}

type BrevTestReader struct {
	Reader
}

func (btr BrevTestReader) GetBrevPorts() (BrevPorts, error) {
	return make(BrevPorts), nil
}

func (btr BrevTestReader) GetBrevHostValueSet() BrevHostValuesSet {
	return make(BrevHostValuesSet)
}

type BrevTestWriter struct {
	Writer
}

var userConfigStr = `Host user-host
Hostname 172.0.0.0
`

func makeMockSSHStore() (*store.FileStore, error) {
	mfs := afero.NewMemMapFs()
	fs := store.NewBasicStore().WithFileSystem(mfs)
	err := afero.WriteFile(mfs, files.GetActiveOrgsPath(), []byte(`{"id":"ejmrvoj8m","name":"brev.dev"}`), 0o644)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	p, err := files.GetUserSSHConfigPath()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	err = afero.WriteFile(mfs, *p, []byte(``), 0o644)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return fs, nil
}

func makeTestUserSSHConfigString() (string, error) {
	store, err := makeMockSSHStore()
	if err != nil {
		return "", err
	}
	userSSHConfigStr := fmt.Sprintf(`%[2]s
Host test-dns.brev.sh
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
  Port 2224

`, store.GetPrivateKeyFilePath(), userConfigStr)
	return userSSHConfigStr, err
}

func makeTestSSHConfig() (*SSHConfig, error) {
	store, err := makeMockSSHStore()
	if err != nil {
		return nil, err
	}
	userSSHConfigStr, err := makeTestUserSSHConfigString()
	if err != nil {
		return nil, err
	}
	err = store.WriteSSHConfig(userSSHConfigStr)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	sshConfig, err := NewSSHConfig(store)
	if err != nil {
		return nil, err
	}
	return sshConfig, err
}

func (suite *BrevSSHTestSuite) SetupTest() {
	s, err := makeMockSSHStore()
	if !suite.Nil(err) {
		return
	}
	suite.store = *s

	userSSHConfigStr := fmt.Sprintf(`%[2]s
Host test-dns.brev.sh
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
	 Port 2224`, s.GetPrivateKeyFilePath(), userConfigStr)
	err = s.WriteSSHConfig(userSSHConfigStr)
	if !suite.Nil(err) {
		return
	}
	suite.Configurer, err = NewDefaultSSHConfigurer(someWorkspaces, *s, s.GetPrivateKeyFilePath())
	suite.Nil(err)
	if !suite.Nil(err) {
		return
	}
}

func (suite *BrevSSHTestSuite) TestGetBrevPorts() {
	ports, err := suite.Configurer.GetBrevPorts([]string{"test-dns.brev.sh", "workspace-images", "brevdev/brev-deploy"})
	if !suite.Nil(err) {
		return
	}
	suite.True(ports["2222"])
	suite.True(ports["2223"])
	suite.True(ports["2224"])
}

func (suite *BrevSSHTestSuite) TestCheckIfBrevHost() {
	for _, host := range suite.Configurer.sshConfig.Hosts[2:] {
		if len(host.Nodes) > 0 {
			isBrevHost := checkIfBrevHost(*host, suite.store.GetPrivateKeyFilePath())
			suite.True(isBrevHost)
		}
	}
}

func (suite *BrevSSHTestSuite) TestPruneInactiveWorkspaces() {
	userSSHConfigStr := fmt.Sprintf(`%[2]s
Host test-dns.brev.sh
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
  Port 2224
`, suite.store.GetPrivateKeyFilePath(), userConfigStr)
	suite.Equal(userSSHConfigStr, suite.Configurer.sshConfig.String())
	err := suite.Configurer.PruneInactiveWorkspaces()
	if !suite.Nil(err) {
		return
	}
	suite.Equal(fmt.Sprintf(`%s
Host test-dns.brev.sh
  Hostname 0.0.0.0
  IdentityFile %s
  User brev
  Port 2222
`, userConfigStr, suite.store.GetPrivateKeyFilePath()), suite.Configurer.sshConfig.String())
}

func (suite *BrevSSHTestSuite) TestAppendBrevEntry() {
	s, err := makeMockSSHStore()
	if !suite.Nil(err) {
		return
	}

	_, err = DefaultSSHConfigurer{sshStore: *s}.makeSSHEntry("bar", "2222")
	if !suite.Nil(err) {
		return
	}
}

func (suite *BrevSSHTestSuite) TestCreateBrevSSHConfigEntries() {
	err := suite.Configurer.CreateBrevSSHConfigEntries()
	if !suite.Nil(err) {
		return
	}
	templateLen := len(strings.Split(workspaceSSHConfigTemplate, "\n"))
	actualLen := len(strings.Split(suite.Configurer.sshConfig.String(), "\n"))
	suite.Greater(actualLen, (templateLen))
}

// TODO abstract out setup
// TODO add in more meaningful assertions
func (suite *BrevSSHTestSuite) TestConfigureSSHNoWorkspaces() {
	s, err := makeMockSSHStore()
	if !suite.Nil(err) {
		return
	}
	sshFileOrig, err := s.GetSSHConfig()
	if !suite.Nil(err) {
		return
	}
	sshConfigurer, err := NewDefaultSSHConfigurer(noWorkspaces, *s, "mock-priv-key")
	if !suite.Nil(err) {
		return
	}
	err = sshConfigurer.Config()
	if !suite.Nil(err) {
		return
	}
	sshFileAfter, err := s.GetSSHConfig()
	if !suite.Nil(err) {
		return
	}

	// should be totally empty except for user values
	if !suite.Equal(sshFileOrig, sshFileAfter) {
		return
	}
}

func (suite *BrevSSHTestSuite) TestConfigureSSHWithWorkspaces() {
	s, err := makeMockSSHStore()
	if !suite.Nil(err) {
		return
	}
	sshFileOrig, err := s.GetSSHConfig()
	if !suite.Nil(err) {
		return
	}
	sshConfigurer, err := NewDefaultSSHConfigurer(someWorkspaces, *s, "mock-priv-key")
	if !suite.Nil(err) {
		return
	}
	err = sshConfigurer.Config()
	if !suite.Nil(err) {
		return
	}
	sshFileAfter, err := s.GetSSHConfig()
	if !suite.Nil(err) {
		return
	}
	if !suite.NotEqual(sshFileOrig, sshFileAfter) {
		return
	}
}

func (suite *BrevSSHTestSuite) TestGetConfiguredWorkspacePort() {
	err := suite.Configurer.Config()
	if !suite.Nil(err) {
		return
	}

	port, err := suite.Configurer.GetConfiguredWorkspacePort(someWorkspaces[0].Workspace)
	if !suite.Nil(err) {
		return
	}
	if !suite.NotEmpty(port) {
		return
	}
}

func TestHostnameFromString(t *testing.T) {
	res := hostnameFromString("")
	if !assert.Equal(t, "", res) {
		return
	}
	res = hostnameFromString("\n")
	if !assert.Equal(t, "", res) {
		return
	}
	res = hostnameFromString("\n\n")
	if !assert.Equal(t, "", res) {
		return
	}

	value := "Host testtime-1bxl-brevdev.brev.sh\n  Hostname 0.0.0.0\n  IdentityFile /Users/alecfong/.brev/brev.pem\n  User brev\n  Port 2222\n\n"
	res = hostnameFromString(value)
	if !assert.Equal(t, "testtime-1bxl-brevdev.brev.sh", res) {
		return
	}
}

func TestCheckIfHostIsActive(t *testing.T) {
	hostIsActive := checkIfHostIsActive(
		"Host workspace-images\n  Hostname 0.0.0.0\n  IdentityFile /home/brev/.brev/brev.pem\n  User brev\n  Port 2223",
		[]string{"brev"},
	)
	assert.False(t, hostIsActive, "assert workspace-images is not an active host")

	hostIsActive = checkIfHostIsActive(
		"Host brev\n  Hostname 0.0.0.0\n  IdentityFile /home/brev/.brev/brev.pem\n  User brev\n  Port 2223",
		[]string{"brev"},
	)
	assert.True(t, hostIsActive, "assert brev is an active host")
}

func TestCreateConfigEntry(t *testing.T) {
	assert.Equal(t, createConfigEntry("foo", true, true), "foo")
	assert.Equal(t, createConfigEntry("foo", true, false), "")
	assert.Equal(t, createConfigEntry("foo", false, true), "foo")
	assert.Equal(t, createConfigEntry("foo", false, false), "foo")
}

func TestSSHConfigFromString(t *testing.T) {
	sshConfig, err := sshConfigFromString("Host user-host\nHostname 172.0.0.0\n\nHost brev\n  Hostname 0.0.0.0\n  IdentityFile /home/brev/.brev/brev.pem\n  User brev\n  Port 2222\n")
	assert.Equal(t, err, nil)
	assert.Equal(t, len(sshConfig.Hosts), 3)
}

func (suite *BrevSSHTestSuite) TestMakeSSHEntry() {
	s, err := makeMockSSHStore()
	if !suite.Nil(err) {
		return
	}
	privateKeyFilePath := s.GetPrivateKeyFilePath()

	entry, err := MakeSSHEntry("bar", "2222", privateKeyFilePath)

	if !suite.Nil(err) {
		return
	}
	suite.Equal(entry,
		`Host bar
  Hostname 0.0.0.0
  IdentityFile /home/brev/.brev/brev.pem
  User brev
  Port 2222

`)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestSSH(t *testing.T) {
	suite.Run(t, new(BrevSSHTestSuite))
}

func TestNewSShConfgiurer(t *testing.T) {
	reader := BrevTestReader{}
	writer := BrevTestWriter{}
	sshConfigurer := NewSSHConfigurer(someWorkspaces, reader, writer, []Writer{writer})
	assert.NotNil(t, sshConfigurer)
}

func TestNewSSHConfg(t *testing.T) {
	sshConfig, err := makeTestSSHConfig()
	assert.Nil(t, err)
	userSSHConfigStr, err := makeTestUserSSHConfigString()
	assert.Nil(t, err)
	assert.NotNil(t, sshConfig)
	assert.Equal(t, len(sshConfig.sshConfig.Hosts), 5)
	assert.Equal(t, sshConfig.sshConfig.String(), userSSHConfigStr)
}

func TestPruneInactiveWorkspaces(t *testing.T) {
	activeWorkspace := "test-dns.brev.sh"
	sshConfig, err := makeTestSSHConfig()
	assert.Equal(t, err, nil)
	err = sshConfig.PruneInactiveWorkspaces([]string{activeWorkspace})
	if !assert.Nil(t, err) {
		return
	}
	assert.Equal(t, fmt.Sprintf(`%s
Host %s
  Hostname 0.0.0.0
  IdentityFile %s
  User brev
  Port 2222
`, userConfigStr, activeWorkspace, sshConfig.store.GetPrivateKeyFilePath()), sshConfig.sshConfig.String())
}

func TestGetBrevHostValues(t *testing.T) {
	sshConfig, err := makeTestSSHConfig()
	assert.Equal(t, err, nil)
	brevhosts := sshConfig.GetBrevHostValues()
	assert.Equal(t, brevhosts, []string{"test-dns.brev.sh", "workspace-images", "brevdev/brev-deploy"})
}

func TestGetBrevPorts(t *testing.T) {
	sshConfig, err := makeTestSSHConfig()
	assert.Equal(t, err, nil)
	brevhosts := sshConfig.GetBrevHostValues()
	assert.Equal(t, brevhosts, []string{"test-dns.brev.sh", "workspace-images", "brevdev/brev-deploy"})
}

func TestGetBrevHostValueSet(t *testing.T) {
	sshConfig, err := makeTestSSHConfig()
	assert.Equal(t, err, nil)
	brevhosts := sshConfig.GetBrevHostValueSet()
	expectedValueSet := make(BrevHostValuesSet)
	expectedValueSet["test-dns.brev.sh"] = true
	expectedValueSet["workspace-images"] = true
	expectedValueSet["brevdev/brev-deploy"] = true
	assert.Equal(t, brevhosts, expectedValueSet)
}
