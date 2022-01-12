package ssh

import (
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

func TestCreateNewSSHConfig(t *testing.T) {
	c := NewSSHConfigurerV2(DummySSHConfigurerV2Store{})
	cStr, err := c.CreateNewSSHConfig(somePlainWorkspaces)
	assert.Nil(t, err)
	correct := `# included in /my/user/config
Host test1-dns
  IdentityFile /my/priv/key.pem
  User brev
  ProxyCommand huproxyclient wss://ssh-test1-dns-org.brev.sh/proxy/localhost/22

Host test2-dns
  IdentityFile /my/priv/key.pem
  User brev
  ProxyCommand huproxyclient wss://ssh-test2-dns-org.brev.sh/proxy/localhost/22

`
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

	userConf := ``
	assert.False(t, c.doesUserSSHConfigIncludeBrevConfig(userConf))

	userConf = `Include /my/brev/config
`
	assert.True(t, c.doesUserSSHConfigIncludeBrevConfig(userConf))

	userConf = `# blahdlkfadlfa
Include /my/brev/config
# baldfhaldjf`
	assert.True(t, c.doesUserSSHConfigIncludeBrevConfig(userConf))
}

func TestAddIncludeToUserConfig(t *testing.T) {
	c := NewSSHConfigurerV2(DummySSHConfigurerV2Store{})

	userConf := ``
	newConf := c.AddIncludeToUserConfig(userConf)
	correct := `Include /my/brev/config
`
	assert.Equal(t, correct, newConf)

	userConf = `b;kasdfa;dsl;afd;kl
blaksdf;asdf;
`
	newConf = c.AddIncludeToUserConfig(userConf)
	correct = `Include /my/brev/config
` + userConf
	assert.Equal(t, correct, newConf)
}
