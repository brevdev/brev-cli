package ssh

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"text/template"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

var somePlainWorkspaces = []entity.Workspace{
	{
		ID:               "test-id-1",
		Name:             "testName1",
		OrganizationID:   "oi",
		WorkspaceClassID: "wci",
		CreatedByUserID:  "cui",
		DNS:              "test1-dns-org.brev.sh",
		Status:           entity.Running,
		Password:         "sdfal",
		GitRepo:          "gitrepo",
		SSHUser:          "ubuntu",
	},
	{
		ID:               "test-id-2",
		Name:             "testName2",
		OrganizationID:   "oi",
		WorkspaceClassID: "wci",
		CreatedByUserID:  "cui",
		DNS:              "test2-dns-org.brev.sh",
		Status:           entity.Running,
		Password:         "sdfal",
		GitRepo:          "gitrepo",
		SSHUser:          "ubuntu",
	},
}

type DummyStore struct{}

type DummySSHConfigurerV2Store struct{}

type noHomeSSHConfigurerV2Store struct {
	DummySSHConfigurerV2Store
}

func (noHomeSSHConfigurerV2Store) GetPrivateKeyPath() (string, error) {
	return "/custom/brev-home/brev.pem", nil
}

func (noHomeSSHConfigurerV2Store) UserHomeDir() (string, error) {
	return "", errors.New("UserHomeDir should not be needed to locate Brev certificate files")
}

func (d DummySSHConfigurerV2Store) GetWSLHostUserSSHConfigPath() (string, error) {
	return "", nil
}

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

func (d DummySSHConfigurerV2Store) GetWindowsDir() (string, error) {
	return "", nil
}

func (d DummySSHConfigurerV2Store) WriteBrevSSHConfigWSL(_ string) error {
	return nil
}

func (d DummySSHConfigurerV2Store) FileExists(_ string) (bool, error) {
	return true, nil
}

func (d DummySSHConfigurerV2Store) GetFileAsString(_ string) (string, error) {
	return "", nil
}

// cannot use (DummySSHConfigurerV2Store literal) (value of type DummySSHConfigurerV2Store) as SSHConfigurerV2Store value in argument to NewSSHConfigurerV2: DummySSHConfigurerV2Store does not implement SSHConfigurerV2Store (missing method GetWSLHostBrevSSHConfigPath)
func (d DummySSHConfigurerV2Store) GetWSLHostBrevSSHConfigPath() (string, error) {
	return "", nil
}

// cannot use (DummySSHConfigurerV2Store literal) (value of type DummySSHConfigurerV2Store) as SSHConfigurerV2Store value in argument to NewSSHConfigurerV2: DummySSHConfigurerV2Store does not implement SSHConfigurerV2Store (missing method GetWSLUserSSHConfig)
func (d DummySSHConfigurerV2Store) GetWSLUserSSHConfig() (string, error) {
	return "", nil
}

// cannot use (DummySSHConfigurerV2Store literal) (value of type DummySSHConfigurerV2Store) as SSHConfigurerV2Store value in argument to NewSSHConfigurerV2: DummySSHConfigurerV2Store does not implement SSHConfigurerV2Store (missing method WriteWSLUserSSHConfig)
func (d DummySSHConfigurerV2Store) WriteWSLUserSSHConfig(_ string) error {
	return nil
}

func (d DummySSHConfigurerV2Store) GetBrevCloudflaredBinaryPath() (string, error) {
	return "", nil
}

func (d DummySSHConfigurerV2Store) UserHomeDir() (string, error) {
	return "/home/test-user", nil
}

func TestCreateNewSSHConfig(t *testing.T) {
	c := NewSSHConfigurerV2(DummySSHConfigurerV2Store{})
	cStr, err := c.CreateNewSSHConfig(somePlainWorkspaces, nil)

	assert.Nil(t, err)
	// sometimes vs code is not happy with the formatting
	// so if the formatting is not correct then the test will fail
	// if you run into this test failing b/c of the formatting
	// this might be why and you can try to fix it by reverting to the original
	// version of the test before vscode autoformats the config
	correct := fmt.Sprintf(`# included in /my/user/config
Host %s
  Hostname test1-dns-org.brev.sh
  IdentityFile "/my/priv/key.pem"
  User ubuntu
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  ControlMaster auto
  ControlPath ~/.ssh/brev-control-%%C
  ControlPersist 10m
  Port 22

Host %s-host
  Hostname test1-dns-org.brev.sh
  IdentityFile "/my/priv/key.pem"
  User ubuntu
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  ControlMaster auto
  ControlPath ~/.ssh/brev-control-%%C
  ControlPersist 10m
  Port 22

Host %s
  Hostname test2-dns-org.brev.sh
  IdentityFile "/my/priv/key.pem"
  User ubuntu
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  ControlMaster auto
  ControlPath ~/.ssh/brev-control-%%C
  ControlPersist 10m
  Port 22

Host %s-host
  Hostname test2-dns-org.brev.sh
  IdentityFile "/my/priv/key.pem"
  User ubuntu
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  ControlMaster auto
  ControlPath ~/.ssh/brev-control-%%C
  ControlPersist 10m
  Port 22

`, somePlainWorkspaces[0].GetLocalIdentifier(), somePlainWorkspaces[0].GetLocalIdentifier(),
		somePlainWorkspaces[1].GetLocalIdentifier(), somePlainWorkspaces[1].GetLocalIdentifier(),
	)
	assert.Equal(t, correct, cStr)

	cStr, err = c.CreateNewSSHConfig([]entity.Workspace{}, nil)
	assert.Nil(t, err)
	correct = `# included in /my/user/config
`
	assert.Equal(t, correct, cStr)
}

func TestCreateNewSSHConfig_DerivesCertPathFromBrevDirectory(t *testing.T) {
	w := entity.Workspace{
		ID:              "env-cert",
		Name:            "cert-env",
		Status:          entity.Running,
		SSHUser:         "ubuntu",
		SSHPort:         22,
		SSHHostname:     "10.0.0.1",
		SSHCertEligible: true,
		PortID:          "port-1",
	}

	c := NewSSHConfigurerV2(noHomeSSHConfigurerV2Store{})
	got, err := c.CreateNewSSHConfig([]entity.Workspace{w}, nil)
	if err != nil {
		t.Fatalf("CreateNewSSHConfig should not need UserHomeDir: %v", err)
	}
	if !strings.Contains(got, "/custom/brev-home/ssh-certs/env-cert") {
		t.Fatalf("certificate path should be rooted in the Brev directory: %s", got)
	}
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
	assert.False(t, doesUserSSHConfigIncludeBrevConfig(userConf, bscp))

	userConf = `Include "/my/brev/config"
`
	assert.True(t, doesUserSSHConfigIncludeBrevConfig(userConf, bscp))

	userConf = `# blahdlkfadlfa
Include "/my/brev/config"
# baldfhaldjf`
	assert.True(t, doesUserSSHConfigIncludeBrevConfig(userConf, bscp))
}

func TestAddIncludeToUserConfig(t *testing.T) {
	c := NewSSHConfigurerV2(DummySSHConfigurerV2Store{})
	bscp, err := c.store.GetBrevSSHConfigPath()
	if !assert.Nil(t, err) {
		return
	}

	userConf := ``
	newConf, err := AddIncludeToUserConfig(userConf, bscp)
	if !assert.Nil(t, err) {
		return
	}
	correct := `Include "/my/brev/config"
`
	assert.Equal(t, correct, newConf)

	userConf = `b;kasdfa;dsl;afd;kl
blaksdf;asdf;
`
	newConf, err = AddIncludeToUserConfig(userConf, bscp)
	if !assert.Nil(t, err) {
		return
	}
	correct = `Include "/my/brev/config"
` + userConf
	assert.Equal(t, correct, newConf)
}

func Test_makeSSHConfigEntryV2(t *testing.T) { //nolint:funlen // test
	type args struct {
		workspace             entity.Workspace
		privateKeyPath        string
		cloudflaredBinaryPath string
		runRemoteCMD          bool
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "test separate workload and host endpoints",
			args: args{
				workspace: entity.Workspace{
					ID:               "test-id-2",
					Name:             "testName2",
					OrganizationID:   "oi",
					WorkspaceClassID: "wci",
					CreatedByUserID:  "cui",
					DNS:              "test2-dns-org.brev.sh",
					Status:           entity.Running,
					Password:         "sdfal",
					GitRepo:          "gitrepo",
					SSHPort:          20,
					SSHUser:          "ubuntu-wk",
					SSHHostname:      "skybridge.example.com",
					HostSSHPort:      2022,
					HostSSHUser:      "ubuntu-host",
					HostSSHHostname:  "203.0.113.10",
				},
				privateKeyPath: "/my/priv/key.pem",
				runRemoteCMD:   true,
			},
			want: `Host testName2
  Hostname skybridge.example.com
  IdentityFile "/my/priv/key.pem"
  User ubuntu-wk
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  ControlMaster auto
  ControlPath ~/.ssh/brev-control-%C
  ControlPersist 10m
  Port 20

Host testName2-host
  Hostname 203.0.113.10
  IdentityFile "/my/priv/key.pem"
  User ubuntu-host
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  ControlMaster auto
  ControlPath ~/.ssh/brev-control-%C
  ControlPersist 10m
  Port 2022

`,
		},
		{
			name: "test by default we use devplane user",
			args: args{
				workspace: entity.Workspace{
					ID:               "test-id-2",
					Name:             "testName2",
					OrganizationID:   "oi",
					WorkspaceClassID: "wci",
					CreatedByUserID:  "cui",
					DNS:              "test2-dns-org.brev.sh",
					Status:           entity.Running,
					Password:         "sdfal",
					GitRepo:          "gitrepo",
					SSHUser:          "ubuntu",
				},
				privateKeyPath: "/my/priv/key.pem",
				runRemoteCMD:   true,
			},
			want: `Host testName2
  Hostname test2-dns-org.brev.sh
  IdentityFile "/my/priv/key.pem"
  User ubuntu
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  ControlMaster auto
  ControlPath ~/.ssh/brev-control-%C
  ControlPersist 10m
  Port 22

Host testName2-host
  Hostname test2-dns-org.brev.sh
  IdentityFile "/my/priv/key.pem"
  User ubuntu
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  ControlMaster auto
  ControlPath ~/.ssh/brev-control-%C
  ControlPersist 10m
  Port 22

`,
		},
		{
			name: "test SSH port is 2022",
			args: args{
				workspace: entity.Workspace{
					ID:               "test-id-2",
					Name:             "testName2",
					OrganizationID:   "oi",
					WorkspaceClassID: "wci",
					CreatedByUserID:  "cui",
					DNS:              "test2-dns-org.brev.sh",
					Status:           entity.Running,
					Password:         "sdfal",
					GitRepo:          "gitrepo",
					SSHPort:          2022,
					SSHUser:          "ubuntu",
				},
				privateKeyPath: "/my/priv/key.pem",
				runRemoteCMD:   true,
			},
			want: `Host testName2
  Hostname test2-dns-org.brev.sh
  IdentityFile "/my/priv/key.pem"
  User ubuntu
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  ControlMaster auto
  ControlPath ~/.ssh/brev-control-%C
  ControlPersist 10m
  Port 2022

Host testName2-host
  Hostname test2-dns-org.brev.sh
  IdentityFile "/my/priv/key.pem"
  User ubuntu
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  ControlMaster auto
  ControlPath ~/.ssh/brev-control-%C
  ControlPersist 10m
  Port 22

`,
		},
		{
			name: "test default ssh proxy",
			args: args{
				workspace: entity.Workspace{
					ID:                   "test-id-2",
					Name:                 "testName2",
					OrganizationID:       "oi",
					WorkspaceClassID:     "wci",
					CreatedByUserID:      "cui",
					DNS:                  "test2-dns-org.brev.sh",
					Status:               entity.Running,
					Password:             "sdfal",
					GitRepo:              "gitrepo",
					SSHProxyHostname:     "test-verb-proxy.com",
					HostSSHProxyHostname: "test-host-proxy.com",
					SSHUser:              "ubuntu",
				},
				privateKeyPath:        "/my/priv/key.pem",
				cloudflaredBinaryPath: "/Users/tmontfort/.brev/cloudflared",
				runRemoteCMD:          true,
			},
			want: `Host testName2
  IdentityFile "/my/priv/key.pem"
  User ubuntu
  ProxyCommand /Users/tmontfort/.brev/cloudflared access ssh --hostname test-verb-proxy.com
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  ControlMaster auto
  ControlPath ~/.ssh/brev-control-%C
  ControlPersist 10m

Host testName2-host
  IdentityFile "/my/priv/key.pem"
  User ubuntu
  ProxyCommand /Users/tmontfort/.brev/cloudflared access ssh --hostname test-host-proxy.com
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  ControlMaster auto
  ControlPath ~/.ssh/brev-control-%C
  ControlPersist 10m

`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := makeSSHConfigEntryV2(tt.args.workspace, tt.args.privateKeyPath, tt.args.cloudflaredBinaryPath, true)
			if (err != nil) != tt.wantErr {
				t.Errorf("makeSSHConfigEntryV2() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			diff := cmp.Diff(tt.want, got)
			if diff != "" {
				t.Fatalf("%s", diff)
			}
		})
	}
}

func TestMakeJetbrainsConfigEntryUsesSSHHostname(t *testing.T) {
	entry := makeJetbrainsConfigEntry(entity.Workspace{
		DNS:         "legacy.example.com",
		SSHHostname: "skybridge.example.com",
		SSHUser:     "root",
		SSHPort:     41234,
	}, "/tmp/brev.pem")

	if entry.Host != "skybridge.example.com" {
		t.Fatalf("host = %q, want skybridge.example.com", entry.Host)
	}
}

func TestSanitizeNodeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"My GPU Box", "my-gpu-box"},
		{"my-ec2", "my-ec2"},
		{"already-clean", "already-clean"},
		{"UPPER CASE", "upper-case"},
		{"special!@#chars", "special-chars"},
		{"  leading/trailing  ", "leading-trailing"},
		{"multiple   spaces", "multiple-spaces"},
		{"", "node"},
		{"!!!!", "node"},
		{"a", "a"},
		{"node-with--double-dash", "node-with-double-dash"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizeNodeName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeNodeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMakeSSHConfigEntryForNode(t *testing.T) {
	entry := ExternalNodeSSHEntry{
		Alias:    "my-gpu-box",
		Hostname: "10.0.0.5",
		Port:     41920,
		User:     "ec2-user",
	}

	got, err := makeSSHConfigEntryForNode(entry, "/home/test/.brev/brev.pem")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := `Host my-gpu-box
  HostName 10.0.0.5
  User ec2-user
  Port 41920
  IdentityFile "/home/test/.brev/brev.pem"
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null
  ServerAliveInterval 30
  ForwardAgent yes

`
	if got != want {
		t.Errorf("makeSSHConfigEntryForNode() mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestCreateNewSSHConfig_WithNodes(t *testing.T) {
	c := NewSSHConfigurerV2(DummySSHConfigurerV2Store{})

	nodes := []ExternalNodeSSHEntry{
		{Alias: "gpu-box", Hostname: "10.0.0.5", Port: 41920, User: "ec2-user"},
	}

	cStr, err := c.CreateNewSSHConfig([]entity.Workspace{}, nodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := `# included in /my/user/config
Host gpu-box
  HostName 10.0.0.5
  User ec2-user
  Port 41920
  IdentityFile "/my/priv/key.pem"
  StrictHostKeyChecking no
  UserKnownHostsFile /dev/null
  ServerAliveInterval 30
  ForwardAgent yes

`
	if cStr != want {
		t.Errorf("CreateNewSSHConfig with nodes mismatch:\ngot:\n%s\nwant:\n%s", cStr, want)
	}
}

func TestCreateNewSSHConfig_WorkspacesAndNodes(t *testing.T) {
	c := NewSSHConfigurerV2(DummySSHConfigurerV2Store{})

	nodes := []ExternalNodeSSHEntry{
		{Alias: "my-node", Hostname: "192.168.1.100", Port: 33000, User: "ubuntu"},
	}

	cStr, err := c.CreateNewSSHConfig(somePlainWorkspaces[:1], nodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain both workspace entry and node entry
	if !assert.Contains(t, cStr, "Host testName1\n") {
		return
	}
	if !assert.Contains(t, cStr, "Host my-node\n") {
		return
	}
	if !assert.Contains(t, cStr, "Port 33000\n") {
		return
	}
}

func makeMockFS() SSHConfigurerV2Store {
	bs := store.NewBasicStore().WithEnvGetter(
		func(s string) string {
			return "test"
		},
	)
	fs := bs.WithFileSystem(afero.NewMemMapFs())

	return fs.WithUserHomeDirGetter(
		func() (string, error) {
			return "/home/test", nil
		},
	)
}

func makeMockWSLFS() SSHConfigurerV2Store {
	bs := store.NewBasicStore().WithEnvGetter(
		func(s string) string {
			return "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/usr/games:/usr/local/games:/usr/lib/wsl/lib:/mnt/c/WINDOWS/system32:/mnt/c/WINDOWS:/mnt/c/WINDOWS/System32/Wbem:/mnt/c/WINDOWS/System32/WindowsPowerShell/v1.0/:/mnt/c/WINDOWS/System32/OpenSSH/:/mnt/c/Users/15854/AppData/Local/Microsoft/WindowsApps:/mnt/c/Users/15854/AppData/Local/Programs/Microsoft VS Code/bin:/snap/bin"
		},
	)
	f := afero.NewMemMapFs()
	fs := bs.WithFileSystem(f)
	dir, err := fs.GetWindowsDir()
	if err != nil {
		panic(err)
	}
	err = f.MkdirAll(dir, 0o755)
	if err != nil {
		panic(err)
	}
	return fs.WithUserHomeDirGetter(
		func() (string, error) {
			return "/home/test", nil
		},
	)
}

func TestSSHConfigurerV2_Update(t *testing.T) { //nolint  // this is a test
	type fields struct {
		store        SSHConfigurerV2Store
		runRemoteCMD bool
	}
	type args struct {
		workspaces []entity.Workspace
	}
	tests := []struct {
		name                   string
		fields                 fields
		args                   args
		wantErr                bool
		linuxSSHConfig         string
		linuxBrevSSHConfig     string
		windowsSSHConfig       string
		windowsBrevSSHConfig   string
		windowsSSHConfigExists bool
		skip                   bool
	}{
		// TODO: Add test cases.
		{
			name: "test update",
			fields: fields{
				store:        makeMockFS(),
				runRemoteCMD: false,
			},
			args: args{
				workspaces: []entity.Workspace{
					{
						ID:               "test-id-1",
						Name:             "testName1",
						OrganizationID:   "oi",
						WorkspaceClassID: "wci",
						CreatedByUserID:  "cui",
						DNS:              "test1-dns-org.brev.sh",
						Status:           entity.Running,
						Password:         "sdfal",
						GitRepo:          "gitrepo",
						SSHUser:          "ubuntu",
					},
				},
			},
			wantErr:        false,
			linuxSSHConfig: "Include \"/home/test/.brev/ssh_config\"\n",
			linuxBrevSSHConfig: `# included in /home/test/.ssh/config
Host testName1
  Hostname test1-dns-org.brev.sh
  IdentityFile "/home/test/.brev/brev.pem"
  User ubuntu
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  ControlMaster auto
  ControlPath ~/.ssh/brev-control-%C
  ControlPersist 10m
  Port 22

Host testName1-host
  Hostname test1-dns-org.brev.sh
  IdentityFile "/home/test/.brev/brev.pem"
  User ubuntu
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  ControlMaster auto
  ControlPath ~/.ssh/brev-control-%C
  ControlPersist 10m
  Port 22

`,

			windowsSSHConfig:       ``,
			windowsBrevSSHConfig:   ``,
			windowsSSHConfigExists: false,
		},
		{
			name: "test update with windows",
			fields: fields{
				store:        makeMockWSLFS(),
				runRemoteCMD: false,
			},
			args: args{
				workspaces: []entity.Workspace{
					{
						ID:               "test-id-1",
						Name:             "testName1",
						OrganizationID:   "oi",
						WorkspaceClassID: "wci",
						CreatedByUserID:  "cui",
						DNS:              "test1-dns-org.brev.sh",
						Status:           entity.Running,
						Password:         "sdfal",
						GitRepo:          "gitrepo",
						SSHUser:          "ubuntu",
					},
				},
			},
			wantErr:        false,
			linuxSSHConfig: "Include \"/home/test/.brev/ssh_config\"\n",
			linuxBrevSSHConfig: `# included in /home/test/.ssh/config
Host testName1
  Hostname test1-dns-org.brev.sh
  IdentityFile "/home/test/.brev/brev.pem"
  User ubuntu
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  ControlMaster auto
  ControlPath ~/.ssh/brev-control-%C
  ControlPersist 10m
  Port 22

Host testName1-host
  Hostname test1-dns-org.brev.sh
  IdentityFile "/home/test/.brev/brev.pem"
  User ubuntu
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  ControlMaster auto
  ControlPath ~/.ssh/brev-control-%C
  ControlPersist 10m
  Port 22

`,
			windowsSSHConfig: "Include \"C:\\Users\\15854\\.brev\\ssh_config\"\n",
			windowsBrevSSHConfig: `# included in C:\Users\15854\.brev\ssh_config
Host testName1
  Hostname test1-dns-org.brev.sh
  IdentityFile "C:\Users\15854\.brev\brev.pem"
  User ubuntu
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  ControlMaster auto
  ControlPath ~/.ssh/brev-control-%C
  ControlPersist 10m
  Port 22

Host testName1-host
  Hostname test1-dns-org.brev.sh
  IdentityFile "C:\Users\15854\.brev\brev.pem"
  User ubuntu
  ServerAliveInterval 30
  UserKnownHostsFile /dev/null
  IdentitiesOnly yes
  StrictHostKeyChecking no
  PasswordAuthentication no
  AddKeysToAgent yes
  ForwardAgent yes
  RequestTTY yes
  ControlMaster auto
  ControlPath ~/.ssh/brev-control-%C
  ControlPersist 10m
  Port 22

`,
			windowsSSHConfigExists: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip()
			}
			s := SSHConfigurerV2{
				store: tt.fields.store,
			}
			if err := s.Update(tt.args.workspaces, nil); (err != nil) != tt.wantErr {
				t.Errorf("SSHConfigurerV2.Update() error = %v, wantErr %v", err, tt.wantErr)
			}
			// make sure the linux config is correct
			linuxConfig, err := s.store.GetFileAsString("/home/test/.ssh/config")
			if err != nil {
				t.Fatal(err)
			}
			diff := cmp.Diff(tt.linuxSSHConfig, linuxConfig)
			if diff != "" {
				t.Fatalf("%s", diff)
			}

			linuxBrevSSHConfig, err := s.store.GetFileAsString("/home/test/.brev/ssh_config")
			if err != nil {
				t.Fatal(err)
			}
			diff = cmp.Diff(tt.linuxBrevSSHConfig, linuxBrevSSHConfig)
			if diff != "" {
				t.Fatalf("%s", diff)
			}

			if tt.windowsSSHConfigExists {
				// make sure the windows config is correct
				windowsConfig, err := tt.fields.store.GetFileAsString("/mnt/c/Users/15854/.ssh/config")
				if err != nil {
					t.Fatal(err)
				}
				diff = cmp.Diff(tt.windowsSSHConfig, windowsConfig)
				if diff != "" {
					t.Fatalf("%s", diff)
				}

				windowsBrevSSHConfig, err := s.store.GetFileAsString("/mnt/c/Users/15854/.brev/ssh_config")
				if err != nil {
					t.Fatal(err)
				}
				diff = cmp.Diff(tt.windowsBrevSSHConfig, windowsBrevSSHConfig)
				if diff != "" {
					t.Fatalf("%s", diff)
				}
			}
		})
	}
}

func TestMakeCertMatchEntry_IneligibleWorkspace(t *testing.T) {
	// No SSHCertEligible flag -> no Match block.
	w := entity.Workspace{ID: "env-1", Name: "n", SSHUser: "u", PortID: "p"}
	if got := makeCertMatchEntry(w, "/home/u/.brev", true); got != "" {
		t.Errorf("ineligible workspace should produce no Match block, got: %s", got)
	}
	// Eligible but no PortID -> no Match block (can't mint without port_id).
	w2 := entity.Workspace{ID: "env-1", Name: "n", SSHUser: "u", SSHCertEligible: true}
	if got := makeCertMatchEntry(w2, "/home/u/.brev", true); got != "" {
		t.Errorf("eligible without PortID should produce no Match block, got: %s", got)
	}
	// supportsCertHook=false (WSL/Windows config) -> no Match block even when eligible.
	// The Windows config is consumed by Windows ssh, but the Match exec would
	// invoke the WSL-Linux brev binary that Windows can't run; certs are deferred
	// there and work via the POSIX config path inside WSL instead.
	w3 := entity.Workspace{ID: "env-1", Name: "n", SSHUser: "u", SSHCertEligible: true, PortID: "p"}
	if got := makeCertMatchEntry(w3, "/home/u/.brev", false); got != "" {
		t.Errorf("WSL/Windows config should produce no Match block, got: %s", got)
	}
}

func TestMakeSSHConfigEntryV2_EligibleWorkspaceIncludesCertMatch(t *testing.T) {
	w := entity.Workspace{
		ID:              "env-cert",
		Name:            "cert-env",
		Status:          entity.Running,
		SSHUser:         "ubuntu",
		SSHPort:         22,
		SSHHostname:     "10.0.0.1",
		SSHCertEligible: true,
		PortID:          "port-1",
	}
	got, err := makeSSHConfigEntryV2(w, "/home/u/.brev/brev.pem", "/tmp/cf", true)
	if err != nil {
		t.Fatalf("makeSSHConfigEntryV2: %v", err)
	}
	// The Match block must precede the Host block.
	matchIdx := strings.Index(got, "Match host cert-env exec")
	hostIdx := strings.Index(got, "Host cert-env")
	if matchIdx < 0 {
		t.Fatal("expected Match block for cert-eligible workspace")
	}
	if hostIdx < 0 {
		t.Fatal("expected Host block")
	}
	if matchIdx >= hostIdx {
		t.Errorf("Match block must precede Host block (match=%d host=%d)", matchIdx, hostIdx)
	}
	if !strings.Contains(got, "/home/u/.brev/ssh-certs/env-cert") {
		t.Error("missing cert key path in Match block")
	}
	if !strings.Contains(got, "/home/u/.brev/brev.pem") {
		t.Error("missing static key path in Host block")
	}
}

func TestMakeCertMatchEntry_UsesAbsoluteBrevPath(t *testing.T) {
	// The Match exec must invoke the absolute path to the running brev binary,
	// not a bare `brev` that could resolve to a stale PATH binary.
	w := entity.Workspace{
		ID: "env-abc", Name: "n", SSHUser: "ubuntu",
		SSHCertEligible: true, PortID: "port-1",
	}
	got := makeCertMatchEntry(w, "/home/u/.brev", true)
	exe, err := os.Executable()
	if err != nil {
		t.Skip("os.Executable unavailable; cannot assert path")
	}
	want := fmt.Sprintf("%s mint-cert --env env-abc", exe)
	if !strings.Contains(got, want) {
		t.Errorf("expected Match exec to use absolute brev path %q; got: %s", want, got)
	}
	if strings.Contains(got, " exec \"brev mint-cert") {
		t.Errorf("Match exec must not use bare `brev`: %s", got)
	}
}

func TestSSHCertRequiredTemplateV3_MergesCertIntoMatchBlock(t *testing.T) {
	rawExec := `'/path/to/brev' mint-cert --env env-1 --port p --linux-user ubuntu --out-key /home/u/.brev/ssh-certs/env-1`
	entry := SSHConfigEntryV2{
		Alias:        "my-env",
		IdentityFile: `"/home/u/.brev/ssh-certs/env-1"`,
		User:         "ubuntu",
		HostName:     "10.0.0.5",
		Port:         34828,
		ExecCommand:  strconv.Quote(rawExec),
	}
	tmpl, err := template.New("m").Parse(SSHCertRequiredTemplateV3)
	if err != nil {
		t.Fatal(err)
	}
	got, err := tmplAndValToString(tmpl, entry)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "Match host my-env exec ") {
		t.Errorf("expected Match block, got: %s", got)
	}
	// The exec value must be a single quoted token; OpenSSH rejects bare
	// whitespace-separated commands in Match exec.
	execLine := "Match host my-env exec " + strconv.Quote(rawExec)
	if !strings.Contains(got, execLine) {
		t.Errorf("Match exec must quote the full command:\nwant: %s\ngot:  %s", execLine, got)
	}
	if strings.Contains(got, "Host my-env\n") {
		t.Errorf("cert-required mode must not emit a Host block: %s", got)
	}
	if !strings.Contains(got, "10.0.0.5") || !strings.Contains(got, "34828") {
		t.Errorf("host settings must be in the Match block: %s", got)
	}
	if !strings.Contains(got, "/home/u/.brev/ssh-certs/env-1") {
		t.Errorf("cert IdentityFile must be present: %s", got)
	}
	if strings.Contains(got, "brev.pem") {
		t.Errorf("static key must be absent in cert-required mode: %s", got)
	}
}

type recordingWSLStore struct {
	DummySSHConfigurerV2Store
	wslWritten bool
}

func (r *recordingWSLStore) WriteBrevSSHConfigWSL(_ string) error {
	r.wslWritten = true
	return nil
}

func TestUpdate_SkipsWSLConfigWhenCertRequired(t *testing.T) {
	orig := isSSHCertRequired
	isSSHCertRequired = func() bool { return true }
	t.Cleanup(func() { isSSHCertRequired = orig })

	store := &recordingWSLStore{}
	s := SSHConfigurerV2{store: store}
	if err := s.Update(nil, nil); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if store.wslWritten {
		t.Error("WriteBrevSSHConfigWSL must not be called when certs are required")
	}
}

func TestUpdate_WritesWSLConfigWhenCertNotRequired(t *testing.T) {
	orig := isSSHCertRequired
	isSSHCertRequired = func() bool { return false }
	t.Cleanup(func() { isSSHCertRequired = orig })

	store := &recordingWSLStore{}
	s := SSHConfigurerV2{store: store}
	if err := s.Update(nil, nil); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !store.wslWritten {
		t.Error("WriteBrevSSHConfigWSL should be called when certs are not required")
	}
}
