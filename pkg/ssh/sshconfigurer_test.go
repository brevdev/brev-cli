package ssh

import (
	"fmt"
	"strings"
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			got, err := makeSSHConfigEntryV2(tt.args.workspace, tt.args.privateKeyPath, tt.args.cloudflaredBinaryPath)
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

func TestSSHConfigurerV2UpdateRenameReplacesManagedAliasesAndPreservesConnectionFields(t *testing.T) {
	tests := []struct {
		name                   string
		workspace              entity.Workspace
		expectedConnectionLine []string
	}{
		{
			name: "direct endpoints",
			workspace: entity.Workspace{
				ID:               "environment-1",
				Name:             "old-instance-name",
				Status:           entity.Running,
				SSHUser:          "workload-user",
				SSHHostname:      "198.51.100.10",
				SSHPort:          2200,
				HostSSHUser:      "host-user",
				HostSSHHostname:  "198.51.100.11",
				HostSSHPort:      2201,
				WorkspaceClassID: "2x8",
				OrganizationID:   "org-1",
				CreatedByUserID:  "user-1",
			},
			expectedConnectionLine: []string{
				"Hostname 198.51.100.10", "User workload-user", "Port 2200",
				"Hostname 198.51.100.11", "User host-user", "Port 2201",
			},
		},
		{
			name: "proxied endpoints",
			workspace: entity.Workspace{
				ID:                   "environment-1",
				Name:                 "old-instance-name",
				Status:               entity.Running,
				SSHUser:              "workload-user",
				SSHProxyHostname:     "workload.example.com",
				HostSSHUser:          "host-user",
				HostSSHProxyHostname: "host.example.com",
				WorkspaceClassID:     "2x8",
				OrganizationID:       "org-1",
				CreatedByUserID:      "user-1",
			},
			expectedConnectionLine: []string{
				"ProxyCommand /home/test/.brev/cloudflared access ssh --hostname workload.example.com", "User workload-user",
				"ProxyCommand /home/test/.brev/cloudflared access ssh --hostname host.example.com", "User host-user",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := makeMockFS()
			userConfig := "Host personal-box\n  Hostname personal.example\n"
			assert.NoError(t, fs.WriteUserSSHConfig(userConfig))
			configurer := NewSSHConfigurerV2(fs)
			assert.NoError(t, configurer.Update([]entity.Workspace{tt.workspace}, nil))

			before, err := fs.GetFileAsString("/home/test/.brev/ssh_config")
			if !assert.NoError(t, err) {
				return
			}
			assert.Contains(t, before, "Host old-instance-name\n")
			assert.Contains(t, before, "Host old-instance-name-host\n")
			for _, expectedLine := range tt.expectedConnectionLine {
				assert.Contains(t, before, expectedLine)
			}

			tt.workspace.Name = "new-instance-name"
			assert.NoError(t, configurer.Update([]entity.Workspace{tt.workspace}, nil))

			after, err := fs.GetFileAsString("/home/test/.brev/ssh_config")
			if !assert.NoError(t, err) {
				return
			}
			assert.NotContains(t, after, "Host old-instance-name\n")
			assert.NotContains(t, after, "Host old-instance-name-host\n")
			assert.Contains(t, after, "Host new-instance-name\n")
			assert.Contains(t, after, "Host new-instance-name-host\n")
			assert.Equal(t, normalizeSSHConfigHostHeaders(before), normalizeSSHConfigHostHeaders(after))

			updatedUserConfig, err := fs.GetFileAsString("/home/test/.ssh/config")
			if !assert.NoError(t, err) {
				return
			}
			assert.Contains(t, updatedUserConfig, userConfig)
			assert.Equal(t, 1, strings.Count(updatedUserConfig, "Include \"/home/test/.brev/ssh_config\""))
		})
	}
}

func normalizeSSHConfigHostHeaders(config string) string {
	lines := strings.Split(config, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "Host ") {
			lines[i] = "Host <alias>"
		}
	}
	return strings.Join(lines, "\n")
}

type renameConfigUpdaterStore struct {
	workspaces []entity.Workspace
	privateKey string
}

func (s *renameConfigUpdaterStore) CopyBin(string) error                { return nil }
func (s *renameConfigUpdaterStore) WriteString(string, string) error    { return nil }
func (s *renameConfigUpdaterStore) GetOSUser() string                   { return "test" }
func (s *renameConfigUpdaterStore) UserHomeDir() (string, error)        { return "/home/test", nil }
func (s *renameConfigUpdaterStore) Remove(string) error                 { return nil }
func (s *renameConfigUpdaterStore) FileExists(string) (bool, error)     { return false, nil }
func (s *renameConfigUpdaterStore) DownloadBinary(string, string) error { return nil }
func (s *renameConfigUpdaterStore) GetContextWorkspaces() ([]entity.Workspace, error) {
	return s.workspaces, nil
}
func (s *renameConfigUpdaterStore) WritePrivateKey(key string) error {
	s.privateKey = key
	return nil
}

// Exercise the real updater and renderer together: a stopped rename must
// remove the old alias, and restart must publish only the new one.
func TestConfigUpdaterRefreshesAliasesAcrossStoppedRename(t *testing.T) {
	store := &renameConfigUpdaterStore{workspaces: []entity.Workspace{{
		ID: "environment-1", Name: "old-instance-name", Status: entity.Running,
		SSHUser: "ubuntu", SSHHostname: "192.0.2.10", SSHPort: 22,
	}}}
	fs := makeMockFS()
	updater := NewConfigUpdater(store, []Config{NewSSHConfigurerV2(fs)}, "private-key")
	for _, step := range []struct {
		name      string
		status    string
		wantAlias string
	}{
		{"old-instance-name", entity.Running, "old-instance-name"},
		{"old-instance-name", entity.Stopped, ""},
		{"new-instance-name", entity.Stopped, ""},
		{"new-instance-name", entity.Running, "new-instance-name"},
	} {
		store.workspaces[0].Name = step.name
		store.workspaces[0].Status = step.status
		require.NoError(t, updater.Run())
		config, err := fs.GetFileAsString("/home/test/.brev/ssh_config")
		require.NoError(t, err)
		for _, alias := range []string{"old-instance-name", "new-instance-name"} {
			if alias == step.wantAlias {
				assert.Contains(t, config, "Host "+alias+"\n")
				assert.Contains(t, config, "Host "+alias+"-host\n")
			} else {
				assert.NotContains(t, config, "Host "+alias+"\n")
				assert.NotContains(t, config, "Host "+alias+"-host\n")
			}
		}
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
