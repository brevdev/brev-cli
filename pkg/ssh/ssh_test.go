package ssh

import (
	"fmt"
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

var someWorkspaces = []entity.WorkspaceWithMeta{
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

func makeMockJetBrainsGateWayStore() *store.FileStore {
	mfs := afero.NewMemMapFs()
	fs := store.NewBasicStore().WithFileSystem(mfs)
	return fs
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

func makeTestSSHConfig(store SSHStore) (*SSHConfig, error) {
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

func TestNewSShConfigurer(t *testing.T) {
	reader := BrevTestReader{}
	writer := BrevTestWriter{}
	sshConfigurer := NewSSHConfigurer(someWorkspaces, reader, writer, []Writer{writer})
	assert.NotNil(t, sshConfigurer)
}

func TestGetActiveWorkspaceIdentifiers(t *testing.T) {
	reader := BrevTestReader{}
	writer := BrevTestWriter{}
	sshConfigurer := NewSSHConfigurer(someWorkspaces, reader, writer, []Writer{writer})
	activeWorkspaces := sshConfigurer.GetActiveWorkspaceIdentifiers()
	assert.Equal(t, activeWorkspaces, []string{"test-dns.brev.sh"})
}

func TestSyncSSHConfigurer(t *testing.T) {
	store, err := makeMockSSHStore()
	assert.Nil(t, err)
	sshConfig, err := makeTestSSHConfig(store)
	assert.Nil(t, err)
	sshConfigurer := NewSSHConfigurer(someWorkspaces, sshConfig, sshConfig, []Writer{sshConfig})

	err = sshConfigurer.Sync()
	assert.Nil(t, err)
	// reread sshConfig
	sshConfig, err = NewSSHConfig(store)
	assert.Equal(t, err, nil)

	assert.Equal(t, fmt.Sprintf(`Host user-host
  Hostname 172.0.0.0

Host test-dns.brev.sh
  Hostname 0.0.0.0
  IdentityFile %s
  User brev
  Port 2222
`, sshConfig.privateKey), sshConfig.sshConfig.String())
	assert.Equal(t, 3, len(sshConfig.sshConfig.Hosts))
}

func TestSSHConfigurerGetConfiguredWorkspacePortSSHConfig(t *testing.T) {
	store, err := makeMockSSHStore()
	assert.Nil(t, err)
	sshConfig, err := makeTestSSHConfig(store)
	assert.Nil(t, err)
	sshConfigurer := NewSSHConfigurer(someWorkspaces, sshConfig, sshConfig, []Writer{sshConfig})
	err = sshConfigurer.Sync()
	assert.Nil(t, err)
	port, err := sshConfigurer.GetConfiguredWorkspacePort(someWorkspaces[0].Workspace.DNS)
	assert.Nil(t, err)
	assert.Equal(t, "2222", port)
}

func TestNewSSHConfg(t *testing.T) {
	store, err := makeMockSSHStore()
	assert.Nil(t, err)
	sshConfig, err := makeTestSSHConfig(store)
	assert.Nil(t, err)
	userSSHConfigStr, err := makeTestUserSSHConfigString()
	assert.Nil(t, err)
	assert.NotNil(t, sshConfig)
	assert.Equal(t, len(sshConfig.sshConfig.Hosts), 5)
	assert.Equal(t, sshConfig.sshConfig.String(), userSSHConfigStr)
}

func TestPruneInactiveWorkspaces(t *testing.T) {
	activeWorkspace := "test-dns.brev.sh"
	store, err := makeMockSSHStore()
	assert.Nil(t, err)
	sshConfig, err := makeTestSSHConfig(store)
	assert.Equal(t, err, nil)
	identityPortMap := make(IdentityPortMap)
	identityPortMap[activeWorkspace] = "2222"
	err = sshConfig.PruneInactiveWorkspaces(identityPortMap)
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
	store, err := makeMockSSHStore()
	assert.Nil(t, err)
	sshConfig, err := makeTestSSHConfig(store)
	assert.Equal(t, err, nil)
	brevhosts := sshConfig.GetBrevHostValues()
	assert.Equal(t, brevhosts, []string{"test-dns.brev.sh", "workspace-images", "brevdev/brev-deploy"})
}

func TestGetBrevPorts(t *testing.T) {
	store, err := makeMockSSHStore()
	assert.Nil(t, err)
	sshConfig, err := makeTestSSHConfig(store)
	assert.Equal(t, err, nil)
	brevports, err := sshConfig.GetBrevPorts()
	assert.Nil(t, err)
	expectedBrevPorts := make(BrevPorts)
	expectedBrevPorts["2222"] = true
	expectedBrevPorts["2223"] = true
	expectedBrevPorts["2224"] = true
	assert.Equal(t, brevports, expectedBrevPorts)
}

func TestGetBrevHostValueSet(t *testing.T) {
	store, err := makeMockSSHStore()
	assert.Nil(t, err)
	sshConfig, err := makeTestSSHConfig(store)
	assert.Equal(t, err, nil)
	brevhosts := sshConfig.GetBrevHostValueSet()
	expectedValueSet := make(BrevHostValuesSet)
	expectedValueSet["test-dns.brev.sh"] = true
	expectedValueSet["workspace-images"] = true
	expectedValueSet["brevdev/brev-deploy"] = true
	assert.Equal(t, brevhosts, expectedValueSet)
}

func TestSyncSSHConfig(t *testing.T) {
	store, err := makeMockSSHStore()
	assert.Nil(t, err)
	sshConfig, err := makeTestSSHConfig(store)
	assert.Equal(t, err, nil)

	identityPortMap := make(IdentityPortMap)
	identityPortMap["test-dns.brev.sh"] = "2222"
	err = sshConfig.Sync(identityPortMap)
	assert.Equal(t, err, nil)
	// reread sshConfig
	sshConfig, err = NewSSHConfig(store)
	assert.Equal(t, err, nil)
	// assert.Equal(t, 4, len(sshConfig.sshConfig.Hosts))
	assert.Equal(t, fmt.Sprintf(`Host user-host
  Hostname 172.0.0.0

Host test-dns.brev.sh
  Hostname 0.0.0.0
  IdentityFile %s
  User brev
  Port 2222
`, sshConfig.privateKey), sshConfig.sshConfig.String())
}

func TestGetConfigurerWorkspacePortSSHConfig(t *testing.T) {
	store, err := makeMockSSHStore()
	assert.Nil(t, err)
	sshConfig, err := makeTestSSHConfig(store)
	assert.Nil(t, err)
	sshConfigurer := NewSSHConfigurer(someWorkspaces, sshConfig, sshConfig, []Writer{sshConfig})
	err = sshConfigurer.Sync()
	assert.Nil(t, err)
	port, err := sshConfigurer.GetConfiguredWorkspacePort(someWorkspaces[0].Workspace.DNS)
	assert.Nil(t, err)
	assert.Equal(t, "2222", port)
}

func TestNewJetBrainsGateWayConfig(t *testing.T) {
	mockJetbrainsGatewayStore := makeMockJetBrainsGateWayStore()
	err := mockJetbrainsGatewayStore.WriteJetBrainsConfig(`<application>
  <component name="SshConfigs">
    <configs>
      <sshConfig host="localhost" id="f72d6499-1376-47df-b274-94de782a7dd2" keyPath="$USER_HOME$/.brev/brev.pem" port="2225" customName="test-manual-install" nameFormat="CUSTOM" username="brev" useOpenSSHConfig="true">
        <option name="customName" value="test-manual-install" />
      </sshConfig>
    </configs>
  </component>
</application>
`)
	assert.Nil(t, err)
	jetBrainsGatewayConfig, err := NewJetBrainsGatewayConfig(BrevTestWriter{}, mockJetbrainsGatewayStore)

	assert.Nil(t, err)
	assert.NotNil(t, jetBrainsGatewayConfig)
}

func TestSyncJetBrainsGateWayConfig(t *testing.T) {
	mockJetbrainsGatewayStore := makeMockJetBrainsGateWayStore()
	err := mockJetbrainsGatewayStore.WriteJetBrainsConfig(`<application>
  <component name="SshConfigs">
    <configs>
      <sshConfig host="localhost" id="f72d6499-1376-47df-b274-94de782a7dd2" keyPath="$USER_HOME$/.brev/brev.pem" port="2225" customName="test-manual-install" nameFormat="CUSTOM" username="brev" useOpenSSHConfig="true">
        <option name="customName" value="test-manual-install" />
      </sshConfig>
    </configs>
  </component>
</application>
`)
	assert.Nil(t, err)
	jetBrainsGatewayConfig, err := NewJetBrainsGatewayConfig(BrevTestWriter{}, mockJetbrainsGatewayStore)
	assert.Nil(t, err)
	assert.NotNil(t, jetBrainsGatewayConfig)

	identityPortMap := make(IdentityPortMap)
	identityPortMap["test-dns.brev.sh"] = "2222"
	err = jetBrainsGatewayConfig.Sync(identityPortMap)
	assert.Nil(t, err)
}

func TestGetBrevPortsJetBrainsGateWayConfig(t *testing.T) {
	mockJetbrainsGatewayStore := makeMockJetBrainsGateWayStore()
	err := mockJetbrainsGatewayStore.WriteJetBrainsConfig(`<application>
  <component name="SshConfigs">
    <configs>
      <sshConfig host="test-dns.brev.sh" id="f72d6499-1376-47df-b274-94de782a7dd2" keyPath="$USER_HOME$/.brev/brev.pem" port="2222" customName="test-manual-install" nameFormat="CUSTOM" username="brev" useOpenSSHConfig="true">
        <option name="customName" value="test-manual-install" />
      </sshConfig>
    </configs>
  </component>
</application>
`)
	assert.Nil(t, err)
	jetBrainsGatewayConfig, err := NewJetBrainsGatewayConfig(BrevTestWriter{}, mockJetbrainsGatewayStore)
	assert.Nil(t, err)
	assert.NotNil(t, jetBrainsGatewayConfig)
	ports, err := jetBrainsGatewayConfig.GetBrevPorts()
	assert.Nil(t, err)
	assert.True(t, ports["2222"])
	assert.Equal(t, len(ports), 1)
}

func TestGetBrevHostValueSetJetBrainsGateWayConfig(t *testing.T) {
	mockJetbrainsGatewayStore := makeMockJetBrainsGateWayStore()
	err := mockJetbrainsGatewayStore.WriteJetBrainsConfig(fmt.Sprintf(`<application>
  <component name="SshConfigs">
    <configs>
      <sshConfig host="test-dns.brev.sh" id="f72d6499-1376-47df-b274-94de782a7dd2" keyPath="%s" port="2222" customName="test-manual-install" nameFormat="CUSTOM" username="brev" useOpenSSHConfig="true">
        <option name="customName" value="test-manual-install" />
      </sshConfig>
    </configs>
  </component>
</application>
`, mockJetbrainsGatewayStore.GetPrivateKeyFilePath()))
	assert.Nil(t, err)
	jetBrainsGatewayConfig, err := NewJetBrainsGatewayConfig(BrevTestWriter{}, mockJetbrainsGatewayStore)
	assert.Nil(t, err)
	assert.NotNil(t, jetBrainsGatewayConfig)
	hostValues := jetBrainsGatewayConfig.GetBrevHostValueSet()
	assert.True(t, hostValues["test-dns.brev.sh"])
	assert.Equal(t, len(hostValues), 1)
}

func TestGetConfiguredWorkspacePortJetBrainsGatewayConfig(t *testing.T) {
	mockJetbrainsGatewayStore := makeMockJetBrainsGateWayStore()
	err := mockJetbrainsGatewayStore.WriteJetBrainsConfig(fmt.Sprintf(`<application>
  <component name="SshConfigs">
    <configs>
      <sshConfig host="test-dns.brev.sh" id="f72d6499-1376-47df-b274-94de782a7dd2" keyPath="%s" port="2222" customName="test-manual-install" nameFormat="CUSTOM" username="brev" useOpenSSHConfig="true">
        <option name="customName" value="test-manual-install" />
      </sshConfig>
    </configs>
  </component>
</application>
`, mockJetbrainsGatewayStore.GetPrivateKeyFilePath()))
	assert.Nil(t, err)
	jetBrainsGatewayConfig, err := NewJetBrainsGatewayConfig(BrevTestWriter{}, mockJetbrainsGatewayStore)
	assert.Nil(t, err)
	assert.NotNil(t, jetBrainsGatewayConfig)
	port, err := jetBrainsGatewayConfig.GetConfiguredWorkspacePort("test-dns.brev.sh")
	assert.Nil(t, err)
	assert.Equal(t, port, "2222")
}

func TestParseJetbrainsGatewayXml(t *testing.T) {
	xml, err := parseJetbrainsGatewayXml(`<application>
  <component name="SshConfigs">
    <configs>
      <sshConfig host="test-dns.brev.sh" id="f72d6499-1376-47df-b274-94de782a7dd2" keyPath="%s" port="2222" customName="test-manual-install" nameFormat="CUSTOM" username="brev" useOpenSSHConfig="true">
        <option name="customName" value="test-manual-install" />
      </sshConfig>
    </configs>
  </component>
</application>
`)
	assert.Nil(t, err)
	assert.Equal(t, len(xml.SSHConfigs), 1)
	assert.Equal(t, xml.SSHConfigs[0].Host, "test-dns.brev.sh")
	assert.Equal(t, xml.SSHConfigs[0].Port, "2222")
}
