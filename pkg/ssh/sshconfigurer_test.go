package ssh

import (
	"fmt"
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/google/go-cmp/cmp"
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
		Status:           entity.Running,
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
		Status:           entity.Running,
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

func (d DummySSHConfigurerV2Store) GetJetBrainsConfigPath() (string, error) {
	return "", nil
}

func (d DummySSHConfigurerV2Store) GetJetBrainsConfig() (string, error) {
	return "", nil
}

func (d DummySSHConfigurerV2Store) WriteJetBrainsConfig(_ string) error {
	return nil
}

func (d DummySSHConfigurerV2Store) DoesJetbrainsFilePathExist() (bool, error) {
	return true, nil
}

func TestCreateNewSSHConfig(t *testing.T) {
	c := NewSSHConfigurerV2(DummySSHConfigurerV2Store{}, true)
	cStr, err := c.CreateNewSSHConfig(somePlainWorkspaces)
	assert.Nil(t, err)
	// sometimes vs code is not happy with the formatting
	// so if the formatting is not correct then the test will fail
	// if you run into this test failing b/c of the formatting
	// this might be why and you can try to fix it by reverting to the original
	// version of the test before vscode autoformats the config
	correct := fmt.Sprintf(`# included in /my/user/config
Host %s
  Hostname test1-dns-org.brev.sh
  IdentityFile /my/priv/key.pem
  User ubuntu
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  StrictHostKeyChecking no
  PasswordAuthentication no
  RequestTTY yes

  RemoteCommand cd /home/ubuntu/gitrepo; $SHELL

Host %s
  Hostname test2-dns-org.brev.sh
  IdentityFile /my/priv/key.pem
  User ubuntu
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  StrictHostKeyChecking no
  PasswordAuthentication no
  RequestTTY yes

  RemoteCommand cd /home/ubuntu/gitrepo; $SHELL

`, somePlainWorkspaces[0].Name,
		somePlainWorkspaces[1].Name)
	assert.Equal(t, correct, cStr)

	cStr, err = c.CreateNewSSHConfig([]entity.Workspace{})
	assert.Nil(t, err)
	correct = `# included in /my/user/config
`
	assert.Equal(t, correct, cStr)
}

func TestEnsureConfigHasInclude(t *testing.T) {
	c := NewSSHConfigurerV2(DummySSHConfigurerV2Store{}, true)

	err := c.EnsureConfigHasInclude()
	assert.Nil(t, err)
	// test if ! then called
}

func TestDoesUserSSHConfigIncludeBrevConfig(t *testing.T) {
	c := NewSSHConfigurerV2(DummySSHConfigurerV2Store{}, true)
	bscp, err := c.store.GetBrevSSHConfigPath()
	if !assert.Nil(t, err) {
		return
	}

	userConf := ``
	assert.False(t, doesUserSSHConfigIncludeBrevConfig(userConf, bscp))

	userConf = `Include /my/brev/config
`
	assert.True(t, doesUserSSHConfigIncludeBrevConfig(userConf, bscp))

	userConf = `# blahdlkfadlfa
Include /my/brev/config
# baldfhaldjf`
	assert.True(t, doesUserSSHConfigIncludeBrevConfig(userConf, bscp))
}

func TestAddIncludeToUserConfig(t *testing.T) {
	c := NewSSHConfigurerV2(DummySSHConfigurerV2Store{}, true)
	bscp, err := c.store.GetBrevSSHConfigPath()
	if !assert.Nil(t, err) {
		return
	}

	userConf := ``
	newConf, err := AddIncludeToUserConfig(userConf, bscp)
	if !assert.Nil(t, err) {
		return
	}
	correct := `Include /my/brev/config
`
	assert.Equal(t, correct, newConf)

	userConf = `b;kasdfa;dsl;afd;kl
blaksdf;asdf;
`
	newConf, err = AddIncludeToUserConfig(userConf, bscp)
	if !assert.Nil(t, err) {
		return
	}
	correct = `Include /my/brev/config
` + userConf
	assert.Equal(t, correct, newConf)
}

func Test_makeSSHConfigEntryV2(t *testing.T) { //nolint:funlen // test
	type args struct {
		workspace      entity.Workspace
		privateKeyPath string
		runRemoteCMD   bool
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "test devplane uses ubuntu",
			args: args{
				workspace: entity.Workspace{
					ID:               "test-id-2",
					Name:             "testName2",
					WorkspaceGroupID: entity.WorkspaceGroupDevPlane,
					OrganizationID:   "oi",
					WorkspaceClassID: "wci",
					CreatedByUserID:  "cui",
					DNS:              "test2-dns-org.brev.sh",
					Status:           entity.Running,
					Password:         "sdfal",
					GitRepo:          "gitrepo",
				},
				privateKeyPath: "/my/priv/key.pem",
				runRemoteCMD:   true,
			},
			want: `Host testname2-id-2
  Hostname test2-dns-org.brev.sh
  IdentityFile /my/priv/key.pem
  User ubuntu
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  StrictHostKeyChecking no
  PasswordAuthentication no
  RequestTTY yes

  RemoteCommand cd /home/ubuntu/gitrepo; $SHELL

`,
		},
		{
			name: "test by default we use devplane user",
			args: args{
				workspace: entity.Workspace{
					ID:               "test-id-2",
					Name:             "testName2",
					WorkspaceGroupID: "test-id-2",
					OrganizationID:   "oi",
					WorkspaceClassID: "wci",
					CreatedByUserID:  "cui",
					DNS:              "test2-dns-org.brev.sh",
					Status:           entity.Running,
					Password:         "sdfal",
					GitRepo:          "gitrepo",
				},
				privateKeyPath: "/my/priv/key.pem",
				runRemoteCMD:   true,
			},
			want: `Host testname2-id-2
  Hostname test2-dns-org.brev.sh
  IdentityFile /my/priv/key.pem
  User ubuntu
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  StrictHostKeyChecking no
  PasswordAuthentication no
  RequestTTY yes

  RemoteCommand cd /home/ubuntu/gitrepo; $SHELL

`,
		},
		{
			name: "test legacy workspace uses brev user 1",
			args: args{
				workspace: entity.Workspace{
					ID:               "test-id-2",
					Name:             "testName2",
					WorkspaceGroupID: "k8s.brevstack.com", // a legacy wsg
					OrganizationID:   "oi",
					WorkspaceClassID: "wci",
					CreatedByUserID:  "cui",
					DNS:              "test2-dns-org.brev.sh",
					Status:           entity.Running,
					Password:         "sdfal",
					GitRepo:          "gitrepo",
				},
				privateKeyPath: "/my/priv/key.pem",
				runRemoteCMD:   true,
			},
			want: `Host testname2-id-2
  IdentityFile /my/priv/key.pem
  User brev
  ProxyCommand brev proxy test-id-2
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  StrictHostKeyChecking no
  PasswordAuthentication no
  RequestTTY yes

  RemoteCommand cd /home/brev/workspace/gitrepo; $SHELL

`,
		},
		{
			name: "test legacy workspace uses brev user 1",
			args: args{
				workspace: entity.Workspace{
					ID:               "test-id-2",
					Name:             "testName2",
					WorkspaceGroupID: "brev-test-brevtenant-cluster", // a legacy wsg
					OrganizationID:   "oi",
					WorkspaceClassID: "wci",
					CreatedByUserID:  "cui",
					DNS:              "test2-dns-org.brev.sh",
					Status:           entity.Running,
					Password:         "sdfal",
					GitRepo:          "gitrepo",
				},
				privateKeyPath: "/my/priv/key.pem",
				runRemoteCMD:   true,
			},
			want: `Host testname2-id-2
  IdentityFile /my/priv/key.pem
  User brev
  ProxyCommand brev proxy test-id-2
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  StrictHostKeyChecking no
  PasswordAuthentication no
  RequestTTY yes

  RemoteCommand cd /home/brev/workspace/gitrepo; $SHELL

`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := makeSSHConfigEntryV2(tt.args.workspace, tt.args.privateKeyPath, tt.args.runRemoteCMD)
			if (err != nil) != tt.wantErr {
				t.Errorf("makeSSHConfigEntryV2() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			diff := cmp.Diff(tt.want, got)
			if diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}
