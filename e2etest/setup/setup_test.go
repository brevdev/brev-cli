package setup

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
)

type WorkspaceTestClient struct {
	SetupParams     *store.SetupParamsV0
	ContainerParams []ContainerParams
	TestWorkspaces  []Workspace
}

type ContainerParams struct {
	Name  string
	Image string
	Port  string
}

func init() {
}

func NewWorkspaceTestClient(setupParams *store.SetupParamsV0, containerParams []ContainerParams) *WorkspaceTestClient {
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		panic("not ok")
	}
	details := runtime.FuncForPC(pc)
	dbTestPrefix := strings.Split(details.Name(), ".")[2]

	binPath := "/home/brev/workspace/brev-cli/brev" // TODO how to auto build binary + relative path

	workspaces := []Workspace{}
	for _, p := range containerParams {
		containerName := fmt.Sprintf("%s-%s", dbTestPrefix, p.Name)
		// [a-zA-Z0-9][a-zA-Z0-9_.-]
		workspace := *NewTestWorkspace(binPath, containerName, p.Image, p.Port, setupParams)
		_ = workspace.Done()
		workspaces = append(workspaces, workspace)
	}

	return &WorkspaceTestClient{
		SetupParams:     setupParams,
		ContainerParams: containerParams,
		TestWorkspaces:  workspaces,
	}
}

func (w WorkspaceTestClient) Done() error {
	var allError error
	for _, w := range w.TestWorkspaces {
		err := w.Done()
		if err != nil {
			allError = multierror.Append(allError, err)
		}
	}
	if allError != nil {
		return breverrors.WrapAndTrace(allError)
	}
	return nil
}

type workspaceTest func(workspace Workspace)

func (w WorkspaceTestClient) Test(test workspaceTest) error {
	for _, w := range w.TestWorkspaces {
		err := w.Setup()
		if err != nil {
			return breverrors.WrapAndTrace(err)
		}
		test(w)
	}
	return nil
}

type Workspace interface {
	Setup() error
	Done() error
	Exec(arg ...string) ([]byte, error) // always returns []byte even if error since stdout/err is still useful
	Copy(src string, dest string) error
}

type ExecResult interface {
	CombinedOutput() ([]byte, error)
}

type TestWorkspace struct {
	SetupParams    *store.SetupParamsV0
	ContainerName  string
	Image          string
	Port           string
	TestBrevBinary string // path to brev binary that should be tested
	ShowOut        bool
}

var _ Workspace = TestWorkspace{}

// image := "brevdev/ubuntu-proxy:0.3.2"
func NewTestWorkspace(testBrevBinaryPath string, containerName string, image string, port string, setupParams *store.SetupParamsV0) *TestWorkspace {
	return &TestWorkspace{SetupParams: setupParams, ContainerName: containerName, Port: port, Image: image, TestBrevBinary: testBrevBinaryPath}
}

func sendToOut(c *exec.Cmd) {
	c.Stderr = os.Stderr
	c.Stdout = os.Stdout
}

func (w TestWorkspace) Setup() error {
	cmdR := exec.Command("docker", "run", "-d", "--privileged=true", fmt.Sprintf("--name=%s", w.ContainerName), "--rm", "-it", w.Image, "-p", w.Port, "bash") //nolint:gosec // for tests
	if w.ShowOut {
		sendToOut(cmdR)
	}
	err := cmdR.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	_, err = w.Exec("mkdir", "-p", "/etc/meta")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	tmpSetupFile := fmt.Sprintf("/tmp/%s_setup_v0.json", w.ContainerName)
	err = files.OverwriteJSON(files.AppFs, tmpSetupFile, w.SetupParams)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	setupPath := fmt.Sprintf("%s:/etc/meta/setup_v0.json", w.ContainerName)
	err = w.Copy(tmpSetupFile, setupPath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	binPath := fmt.Sprintf("%s:/usr/local/bin/brev", w.ContainerName)
	err = w.Copy(w.TestBrevBinary, binPath)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	_, err = w.Exec("/usr/local/bin/brev", "setupworkspace")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w TestWorkspace) Exec(arg ...string) ([]byte, error) {
	arg = append([]string{"exec", w.ContainerName}, arg...)
	cmdM := exec.Command("docker", arg...) //nolint:gosec // for tests
	out, err := cmdM.CombinedOutput()
	if w.ShowOut {
		fmt.Print(out)
	}
	if err != nil {
		return out, breverrors.WrapAndTrace(err)
	}
	return out, nil
}

func (w TestWorkspace) Copy(src string, dest string) error {
	cmdC := exec.Command("docker", "cp", src, dest) //nolint:gosec // for tests
	if w.ShowOut {
		sendToOut(cmdC)
	}
	err := cmdC.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w TestWorkspace) Done() error {
	cmd := exec.Command("docker", "kill", w.ContainerName) //nolint:gosec // for tests
	if w.ShowOut {
		sendToOut(cmd)
	}
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func NewTestSetupParams(keyPair *store.KeyPair) *store.SetupParamsV0 {
	return &store.SetupParamsV0{
		WorkspaceHost:                    "name-rand-org.x.y",
		WorkspacePort:                    22778,
		WorkspaceBaseRepo:                "github.com:brevdev/test-repo-user-config.git",
		WorkspaceProjectRepo:             "github.com:brevdev/test-repo-dotbrev.git",
		WorkspaceApplicationStartScripts: []string{},
		WorkspaceUsername:                "brevtest",
		WorkspaceEmail:                   "test+3333@gmail.com",
		WorkspacePassword:                "12345",
		WorkspaceKeyPair:                 keyPair,
		SetupScript:                      nil,
	}
}

func GetTestKeys() (*store.KeyPair, error) {
	kp := store.KeyPair{}
	err := files.ReadJSON(files.AppFs, "/home/brev/workspace/brev-cli/assets/test_keypair.json", &kp) // TODO relative path
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &kp, nil
}

func Test_UserBrevProjectBrev(t *testing.T) {
	keys, err := GetTestKeys()
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)
	client := NewWorkspaceTestClient(params, []ContainerParams{
		{
			Name:  "brevdev-ubuntu-proxy-0.3.2",
			Image: "brevdev/ubuntu-proxy:0.3.2",
			Port:  "22778",
		},
	})

	err = client.Test(func(w Workspace) {
		AssertUser(t, w, "root")
		AssertCwd(t, w, "/home/brev/workspace")
		AssertInternalCurlOuputContains(t, w, "localhost:22778", "Found. Redirecting to ./login")
		AssertInternalSSHServerRunning(t, w, "/home/brev/.ssh/id_rsa", "brev", "ls")
		AssertValidBrevRepoSetup(t, w, "test-repo-dotbrev")
		AssertValidBrevRepoSetup(t, w, "user-dotbrev")
	})
	assert.Nil(t, err)
}

func AssertValidBrevRepoSetup(t *testing.T, w Workspace, repoPath string) {
	t.Helper()
	AssertPathExists(t, w, repoPath)
	AssertPathExists(t, w, fmt.Sprintf("%s/.git", repoPath))
	AssertPathExists(t, w, fmt.Sprintf("%s/.brev", repoPath))
	AssertPathExists(t, w, fmt.Sprintf("%s/.brev/setup.sh", repoPath))
	AssertPathExists(t, w, fmt.Sprintf("%s/.brev/logs", repoPath))
}

func AssertCwd(t *testing.T, w Workspace, expectedCwd string) {
	t.Helper()
	out, err := w.Exec("pwd")
	assert.Nil(t, err)
	assert.Equal(t, expectedCwd, strings.TrimSpace(string(out)))
}

func AssertUser(t *testing.T, w Workspace, expectedUser string) {
	t.Helper()
	out, err := w.Exec("whoami")
	assert.Nil(t, err)
	assert.Equal(t, expectedUser, strings.TrimSpace(string(out)))
}

func AssertInternalCurlOuputContains(t *testing.T, w Workspace, url string, contains string) {
	t.Helper()
	out, err := w.Exec("curl", "-s", url)
	assert.Nil(t, err)
	assert.Contains(t, string(out), contains)
}

// func AssertCodeServerPasswordWorks(t *testing.T) {
// }

func AssertInternalSSHServerRunning(t *testing.T, w Workspace, privKeyPath string, user string, healthCheck string) {
	t.Helper()

	if !AssertPathExists(t, w, privKeyPath) {
		return
	}

	_, err := w.Exec("ssh", "-i", privKeyPath, "-o", "StrictHostKeyChecking no", fmt.Sprintf("%s@localhost", user), healthCheck)
	assert.Nil(t, err, "can connect with private key")

	_, err = w.Exec("ssh", "-o", "StrictHostKeyChecking no", fmt.Sprintf("%s@localhost", user), healthCheck)
	assert.Error(t, err, "can not connect without private key")
}

// func AssertInternalSSHServerNoPass(t *testing.T, w Workspace, user string, pass string, healthCheck string) {
// 	t.Helper()
// 	// TODO assert can't connect with password
// 	// TODO how to pass password in one line - use sshpass?
// }

func AssertUserGitExists(t *testing.T) {
	// validate cloned, logs
}

func AssertPathExists(t *testing.T, workspace Workspace, path string) bool {
	t.Helper()
	_, err := workspace.Exec("ls", path)
	return assert.Nil(t, err)
}

func AssertGitProjectExistsWithoutBrev(t *testing.T) {
	// validate cloned, logs
}

func AssertStringInLogs(t *testing.T, logsPath string, value string) {
}

func AssertNoGitProject(t *testing.T) {
}
