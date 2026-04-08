package refresh

import (
	"io"
	"io/fs"
	"strings"
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
)

type previewStore struct{}

func (previewStore) GetContextWorkspaces() ([]entity.Workspace, error) {
	return []entity.Workspace{
		{
			Name:    "testName1",
			DNS:     "test1-dns-org.brev.sh",
			Status:  entity.Running,
			SSHUser: "ubuntu",
		},
		{
			Name:    "ignored-stopped",
			DNS:     "ignored-dns-org.brev.sh",
			Status:  entity.Stopped,
			SSHUser: "ubuntu",
		},
	}, nil
}

func (previewStore) WritePrivateKey(string) error { return nil }
func (previewStore) GetCurrentUser() (*entity.User, error) {
	return &entity.User{ID: "user-1"}, nil
}
func (previewStore) GetCurrentUserKeys() (*entity.UserKeys, error) {
	return &entity.UserKeys{PrivateKey: "test-key"}, nil
}
func (previewStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return &entity.Organization{ID: "org-1"}, nil
}
func (previewStore) GetAccessToken() (string, error)    { return "", nil }
func (previewStore) Chmod(string, fs.FileMode) error    { return nil }
func (previewStore) MkdirAll(string, fs.FileMode) error { return nil }
func (previewStore) GetBrevCloudflaredBinaryPath() (string, error) {
	return "/home/test/.brev/bin/cloudflared", nil
}
func (previewStore) Create(string) (io.WriteCloser, error)        { return nil, nil }
func (previewStore) WriteBrevSSHConfig(string) error              { return nil }
func (previewStore) GetUserSSHConfig() (string, error)            { return "", nil }
func (previewStore) WriteUserSSHConfig(string) error              { return nil }
func (previewStore) GetPrivateKeyPath() (string, error)           { return "/home/test/.brev/brev.pem", nil }
func (previewStore) GetUserSSHConfigPath() (string, error)        { return "/home/test/.ssh/config", nil }
func (previewStore) GetBrevSSHConfigPath() (string, error)        { return "/home/test/.brev/ssh/config", nil }
func (previewStore) GetJetBrainsConfigPath() (string, error)      { return "", nil }
func (previewStore) GetJetBrainsConfig() (string, error)          { return "", nil }
func (previewStore) WriteJetBrainsConfig(string) error            { return nil }
func (previewStore) DoesJetbrainsFilePathExist() (bool, error)    { return false, nil }
func (previewStore) GetWSLHostUserSSHConfigPath() (string, error) { return "", nil }
func (previewStore) GetWindowsDir() (string, error)               { return "", nil }
func (previewStore) WriteBrevSSHConfigWSL(string) error           { return nil }
func (previewStore) GetFileAsString(string) (string, error)       { return "", nil }
func (previewStore) FileExists(string) (bool, error)              { return false, nil }
func (previewStore) GetWSLHostBrevSSHConfigPath() (string, error) { return "", nil }
func (previewStore) GetWSLUserSSHConfig() (string, error)         { return "", nil }
func (previewStore) WriteWSLUserSSHConfig(string) error           { return nil }
func (previewStore) CopyBin(string) error                         { return nil }
func (previewStore) WriteString(string, string) error             { return nil }
func (previewStore) GetOSUser() string                            { return "test-user" }
func (previewStore) UserHomeDir() (string, error)                 { return "/home/test", nil }
func (previewStore) Remove(string) error                          { return nil }
func (previewStore) DownloadBinary(string, string) error          { return nil }

func TestBuildSSHConfigPreview(t *testing.T) {
	preview, err := BuildSSHConfigPreview(previewStore{})
	if err != nil {
		t.Fatalf("BuildSSHConfigPreview returned error: %v", err)
	}

	if preview.IncludeDirective != `Include "/home/test/.brev/ssh/config"` {
		t.Fatalf("unexpected include directive: %q", preview.IncludeDirective)
	}

	if preview.BrevConfigPath != "/home/test/.brev/ssh/config" {
		t.Fatalf("unexpected brev config path: %q", preview.BrevConfigPath)
	}

	if preview.BrevConfig == "" {
		t.Fatal("expected non-empty brev config")
	}

	if !strings.Contains(preview.BrevConfig, "Host testName1") {
		t.Fatalf("expected running workspace config, got:\n%s", preview.BrevConfig)
	}

	if strings.Contains(preview.BrevConfig, "ignored-stopped") {
		t.Fatalf("did not expect stopped workspace in config, got:\n%s", preview.BrevConfig)
	}
}
