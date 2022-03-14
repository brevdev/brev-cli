package ssh

import (
	"fmt"
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/stretchr/testify/assert"
)

var somePlainWorkspaces = []entity.Workspace{
	{
		ID:               "test-id-1",
		Name:             "testName1",
		WorkspaceGroupID: "wgi",
		OrganizationID:   "oi",
		WorkspaceClassID: "wci",
		CreatedByUserID:  "cui",
		DNS:              "test1-dns-org.brev.sh",
		Status:           "RUNNING",
		Password:         "sdfal",
		GitRepo:          "gitrepo",
	},
	{
		ID:               "test-id-2",
		Name:             "testName2",
		WorkspaceGroupID: "wgi",
		OrganizationID:   "oi",
		WorkspaceClassID: "wci",
		CreatedByUserID:  "cui",
		DNS:              "test2-dns-org.brev.sh",
		Status:           "RUNNING",
		Password:         "sdfal",
		GitRepo:          "gitrepo",
	},
}

type DummyStore struct{}

func (d DummyStore) GetWorkspaces() ([]entity.Workspace, error) {
	return []entity.Workspace{}, nil
}

type DummySSHConfigurerV2Store struct{}

func (d DummySSHConfigurerV2Store) OverrideWriteSSHConfig(_ string) error {
	return nil
}

func (d DummySSHConfigurerV2Store) WriteBrevSSHConfig(_ string) error {
	return nil
}

func (d DummySSHConfigurerV2Store) GetUserSSHConfig() (string, error) {
	return "", nil
}

func (d DummySSHConfigurerV2Store) WriteUserSSHConfig(_ string) error {
	return nil
}

func (d DummySSHConfigurerV2Store) GetPrivateKeyPath() (string, error) {
	return "/my/priv/key.pem", nil
}

func (d DummySSHConfigurerV2Store) GetUserSSHConfigPath() (string, error) {
	return "/my/user/config", nil
}

func (d DummySSHConfigurerV2Store) GetBrevSSHConfigPath() (string, error) {
	return "/my/brev/config", nil
}

func TestCreateNewSSHConfig(t *testing.T) {
	c := NewSSHConfigurerV2(DummySSHConfigurerV2Store{})
	cStr, err := c.CreateNewSSHConfig(somePlainWorkspaces)
	assert.Nil(t, err)
	correct := fmt.Sprintf(`# included in /my/user/config
Host %s
  IdentityFile /my/priv/key.pem
  User brev
  ProxyCommand brev proxy test-id-1
  ServerAliveInterval 30

Host %s
  IdentityFile /my/priv/key.pem
  User brev
  ProxyCommand brev proxy test-id-2
  ServerAliveInterval 30

`, somePlainWorkspaces[0].GetLocalIdentifier(somePlainWorkspaces),
		somePlainWorkspaces[1].GetLocalIdentifier(somePlainWorkspaces))
	assert.Equal(t, correct, cStr)

	cStr, err = c.CreateNewSSHConfig([]entity.Workspace{})
	assert.Nil(t, err)
	correct = `# included in /my/user/config
`
	assert.Equal(t, correct, cStr)
}

func TestEnsureConfigHasInclude(t *testing.T) {
	c := NewSSHConfigurerV2(DummySSHConfigurerV2Store{})

	err := c.EnsureConfigHasInclude()
	assert.Nil(t, err)
	// test if ! then called
}

func TestDoesUserSSHConfigIncludeBrevConfig(t *testing.T) {
	c := NewSSHConfigurerV2(DummySSHConfigurerV2Store{})
	bscp, err := c.store.GetBrevSSHConfigPath()
	if !assert.Nil(t, err) {
		return
	}

	userConf := ``
	assert.False(t, c.doesUserSSHConfigIncludeBrevConfig(userConf, bscp))

	userConf = `Include /my/brev/config
`
	assert.True(t, c.doesUserSSHConfigIncludeBrevConfig(userConf, bscp))

	userConf = `# blahdlkfadlfa
Include /my/brev/config
# baldfhaldjf`
	assert.True(t, c.doesUserSSHConfigIncludeBrevConfig(userConf, bscp))
}

func TestAddIncludeToUserConfig(t *testing.T) {
	c := NewSSHConfigurerV2(DummySSHConfigurerV2Store{})
	bscp, err := c.store.GetBrevSSHConfigPath()
	if !assert.Nil(t, err) {
		return
	}

	userConf := ``
	newConf, err := c.AddIncludeToUserConfig(userConf, bscp)
	if !assert.Nil(t, err) {
		return
	}
	correct := `Include /my/brev/config
`
	assert.Equal(t, correct, newConf)

	userConf = `b;kasdfa;dsl;afd;kl
blaksdf;asdf;
`
	newConf, err = c.AddIncludeToUserConfig(userConf, bscp)
	if !assert.Nil(t, err) {
		return
	}
	correct = `Include /my/brev/config
` + userConf
	assert.Equal(t, correct, newConf)
}
