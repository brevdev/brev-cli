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
			DNS:              "test-dns-org.brev.sh",
			Status:           "RUNNING",
			Password:         "sdfal",
			GitRepo:          "gitrepo",
		},
	},
}

const (
	userConfigStr = `Host user-host
  Hostname 172.0.0.0
`
	WorkingRSAPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----`
	CorruptRSAPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----`
)

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
Host %[3]s
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

`, store.GetPrivateKeyFilePath(), userConfigStr, someWorkspaces[0].GetLocalIdentifier())
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

	value := "Host test-ident\n  Hostname 0.0.0.0\n  IdentityFile /Users/alecfong/.brev/brev.pem\n  User brev\n  Port 2222\n\n"
	res = hostnameFromString(value)
	if !assert.Equal(t, "test-ident", res) {
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
	store, err := makeMockSSHStore()
	assert.Nil(t, err)
	reader, err := makeTestSSHConfig(store)
	assert.Nil(t, err)
	writer := reader

	sshConfigurer := NewSSHConfigurer(someWorkspaces, reader, writer, []Writer{writer}, WorkingRSAPrivateKey)
	assert.NotNil(t, sshConfigurer)
}

func TestGetActiveWorkspaceIdentifiers(t *testing.T) {
	store, err := makeMockSSHStore()
	assert.Nil(t, err)
	reader, err := makeTestSSHConfig(store)
	assert.Nil(t, err)
	writer := reader
	sshConfigurer := NewSSHConfigurer(someWorkspaces, reader, writer, []Writer{writer}, WorkingRSAPrivateKey)
	activeWorkspaces := sshConfigurer.GetActiveWorkspaceIdentifiers()
	assert.Equal(t, activeWorkspaces, []string{someWorkspaces[0].GetLocalIdentifier()})
}

func TestSyncSSHConfigurer(t *testing.T) {
	mockStore, err := makeMockSSHStore()
	assert.Nil(t, err)
	sshConfig, err := makeTestSSHConfig(mockStore)
	assert.Nil(t, err)
	sshConfigurer := NewSSHConfigurer(someWorkspaces, sshConfig, sshConfig, []Writer{sshConfig}, WorkingRSAPrivateKey)

	err = sshConfigurer.Sync()
	assert.Nil(t, err)
	// reread sshConfig
	sshConfig, err = NewSSHConfig(mockStore)
	assert.Equal(t, err, nil)

	assert.Equal(t, fmt.Sprintf(`Host user-host
  Hostname 172.0.0.0

Host %s
  Hostname 0.0.0.0
  IdentityFile %s
  User brev
  Port 2222
`, someWorkspaces[0].GetLocalIdentifier(), sshConfig.privateKey), sshConfig.sshConfig.String())
	assert.Equal(t, 3, len(sshConfig.sshConfig.Hosts))
	privateKeyExists, err := mockStore.FileExists(sshConfig.privateKey)
	assert.Nil(t, err)
	assert.True(t, privateKeyExists)
	privateKeyFilePath := mockStore.GetPrivateKeyFilePath()
	pk, err := mockStore.GetOrCreateFile(privateKeyFilePath)
	if !assert.Nil(t, err) {
		return
	}
	data, err := afero.ReadAll(pk)
	if !assert.Nil(t, err) {
		return
	}
	assert.Nil(t, store.VerifyPrivateKey(data))
}

func TestSSHConfigurerGetConfiguredWorkspacePortSSHConfig(t *testing.T) {
	store, err := makeMockSSHStore()
	assert.Nil(t, err)
	sshConfig, err := makeTestSSHConfig(store)
	assert.Nil(t, err)
	sshConfigurer := NewSSHConfigurer(someWorkspaces, sshConfig, sshConfig, []Writer{sshConfig}, WorkingRSAPrivateKey)
	err = sshConfigurer.Sync()
	assert.Nil(t, err)
	port, err := sshConfigurer.GetConfiguredWorkspacePort(someWorkspaces[0].Workspace.GetLocalIdentifier())
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
	activeWorkspace := someWorkspaces[0].GetLocalIdentifier()
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
	assert.Equal(t, brevhosts, []string{someWorkspaces[0].GetLocalIdentifier(), "workspace-images", "brevdev/brev-deploy"})
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
	expectedValueSet[someWorkspaces[0].GetLocalIdentifier()] = true
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
	identityPortMap[someWorkspaces[0].GetLocalIdentifier()] = "2222"
	err = sshConfig.Sync(identityPortMap)
	assert.Equal(t, err, nil)
	// reread sshConfig
	sshConfig, err = NewSSHConfig(store)
	assert.Equal(t, err, nil)
	// assert.Equal(t, 4, len(sshConfig.sshConfig.Hosts))
	assert.Equal(t, fmt.Sprintf(`Host user-host
  Hostname 172.0.0.0

Host %s
  Hostname 0.0.0.0
  IdentityFile %s
  User brev
  Port 2222
`, someWorkspaces[0].GetLocalIdentifier(), sshConfig.privateKey), sshConfig.sshConfig.String())
}

func TestGetConfigurerWorkspacePortSSHConfig(t *testing.T) {
	store, err := makeMockSSHStore()
	assert.Nil(t, err)
	sshConfig, err := makeTestSSHConfig(store)
	assert.Nil(t, err)
	sshConfigurer := NewSSHConfigurer(someWorkspaces, sshConfig, sshConfig, []Writer{sshConfig}, WorkingRSAPrivateKey)
	err = sshConfigurer.Sync()
	assert.Nil(t, err)
	port, err := sshConfigurer.GetConfiguredWorkspacePort(someWorkspaces[0].Workspace.GetLocalIdentifier())
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
	jetBrainsGatewayConfig, err := NewJetBrainsGatewayConfig(mockJetbrainsGatewayStore)

	assert.Nil(t, err)
	assert.NotNil(t, jetBrainsGatewayConfig)
}

func TestSyncJetBrainsGateWayConfig(t *testing.T) {
	mockJetbrainsGatewayStore := makeMockJetBrainsGateWayStore()
	err := mockJetbrainsGatewayStore.WriteJetBrainsConfig(`<application>
  <component name="SshConfigs">
    <configs>
      <sshConfig host="foo" id="f72d6499-1376-47df-b274-94de782a7dd2" keyPath="bar" port="2225" customName="test-manual-install" nameFormat="CUSTOM" username="sfdfls" useOpenSSHConfig="true">
        <option name="customName" value="test-manual-install" />
      </sshConfig>
    </configs>
  </component>
</application>
`)
	assert.Nil(t, err)
	jetBrainsGatewayConfig, err := NewJetBrainsGatewayConfig(mockJetbrainsGatewayStore)
	assert.Nil(t, err)
	assert.NotNil(t, jetBrainsGatewayConfig)

	identityPortMap := make(IdentityPortMap)
	identityPortMap[someWorkspaces[0].GetLocalIdentifier()] = "2222"
	err = jetBrainsGatewayConfig.Sync(identityPortMap)
	assert.Nil(t, err)
	privatekeypath := mockJetbrainsGatewayStore.GetPrivateKeyFilePath()
	assert.Nil(t, err)
	config, err := mockJetbrainsGatewayStore.GetJetBrainsConfig()
	assert.Nil(t, err)
	assert.Equal(t, config, fmt.Sprintf(`<application>
  <component name="SshConfigs">
    <configs>
      <sshConfig id="f72d6499-1376-47df-b274-94de782a7dd2" customName="test-manual-install" nameFormat="CUSTOM" useOpenSSHConfig="true" host="foo" port="2225" keyPath="bar" username="sfdfls">
        <option name="customName" value="test-manual-install"></option>
      </sshConfig>
      <sshConfig customName="%s" nameFormat="CUSTOM" host="localhost" port="2222" keyPath="%s" username="brev">
        <option name="CustomName" value="%s"></option>
      </sshConfig>
    </configs>
  </component>
</application>`, someWorkspaces[0].GetLocalIdentifier(), privatekeypath, someWorkspaces[0].GetLocalIdentifier()))
}

func TestGetBrevPortsJetBrainsGateWayConfig(t *testing.T) {
	mockJetbrainsGatewayStore := makeMockJetBrainsGateWayStore()
	err := mockJetbrainsGatewayStore.WriteJetBrainsConfig(fmt.Sprintf(`<application>
  <component name="SshConfigs">
    <configs>
      <sshConfig host="%s" id="f72d6499-1376-47df-b274-94de782a7dd2" keyPath="$USER_HOME$/.brev/brev.pem" port="2222" customName="test-manual-install" nameFormat="CUSTOM" username="brev" useOpenSSHConfig="true">
        <option name="customName" value="test-manual-install" />
      </sshConfig>
    </configs>
  </component>
</application>
`, someWorkspaces[0].GetLocalIdentifier()))
	assert.Nil(t, err)
	jetBrainsGatewayConfig, err := NewJetBrainsGatewayConfig(mockJetbrainsGatewayStore)
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
      <sshConfig host="%s" id="f72d6499-1376-47df-b274-94de782a7dd2" keyPath="%s" port="2222" customName="test-manual-install" nameFormat="CUSTOM" username="brev" useOpenSSHConfig="true">
        <option name="customName" value="test-manual-install" />
      </sshConfig>
    </configs>
  </component>
</application>
`, someWorkspaces[0].GetLocalIdentifier(), mockJetbrainsGatewayStore.GetPrivateKeyFilePath()))
	assert.Nil(t, err)
	jetBrainsGatewayConfig, err := NewJetBrainsGatewayConfig(mockJetbrainsGatewayStore)
	assert.Nil(t, err)
	assert.NotNil(t, jetBrainsGatewayConfig)
	hostValues := jetBrainsGatewayConfig.GetBrevHostValueSet()
	assert.True(t, hostValues[someWorkspaces[0].GetLocalIdentifier()])
	assert.Equal(t, len(hostValues), 1)
}

func TestGetConfiguredWorkspacePortJetBrainsGatewayConfig(t *testing.T) {
	mockJetbrainsGatewayStore := makeMockJetBrainsGateWayStore()
	err := mockJetbrainsGatewayStore.WriteJetBrainsConfig(fmt.Sprintf(`<application>
  <component name="SshConfigs">
    <configs>
      <sshConfig host="%s" id="f72d6499-1376-47df-b274-94de782a7dd2" keyPath="%s" port="2222" customName="test-manual-install" nameFormat="CUSTOM" username="brev" useOpenSSHConfig="true">
        <option name="customName" value="test-manual-install" />
      </sshConfig>
    </configs>
  </component>
</application>
`, someWorkspaces[0].GetLocalIdentifier(), mockJetbrainsGatewayStore.GetPrivateKeyFilePath()))
	assert.Nil(t, err)
	jetBrainsGatewayConfig, err := NewJetBrainsGatewayConfig(mockJetbrainsGatewayStore)
	assert.Nil(t, err)
	assert.NotNil(t, jetBrainsGatewayConfig)
	port, err := jetBrainsGatewayConfig.GetConfiguredWorkspacePort(someWorkspaces[0].GetLocalIdentifier())
	assert.Nil(t, err)
	assert.Equal(t, port, "2222")
}

func TestParseJetbrainsGatewayXml(t *testing.T) {
	mockJetbrainsGatewayStore := makeMockJetBrainsGateWayStore()
	xml, err := parseJetbrainsGatewayXML(fmt.Sprintf(`<application>
  <component name="SshConfigs">
    <configs>
      <sshConfig host="%s" id="f72d6499-1376-47df-b274-94de782a7dd2" keyPath="%s" port="2222" customName="test-manual-install" nameFormat="CUSTOM" username="brev" useOpenSSHConfig="true">
        <option name="customName" value="test-manual-install" />
      </sshConfig>
    </configs>
  </component>
</application>
`, someWorkspaces[0].GetLocalIdentifier(), mockJetbrainsGatewayStore.GetPrivateKeyFilePath()))
	assert.Nil(t, err)
	assert.Equal(t, len(xml.Component.Configs.SSHConfigs), 1)
	assert.Equal(t, xml.Component.Configs.SSHConfigs[0].Host, someWorkspaces[0].GetLocalIdentifier())
	assert.Equal(t, xml.Component.Configs.SSHConfigs[0].Port, "2222")
}
