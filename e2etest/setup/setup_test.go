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
	Exec(arg ...string) error
	Copy(src string, dest string) error
}

type TestWorkspace struct {
	SetupParams    *store.SetupParamsV0
	ContainerName  string
	Image          string
	Port           string
	TestBrevBinary string // path to brev binary that should be tested
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
	sendToOut(cmdR)
	err := cmdR.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = w.Exec("mkdir", "-p", "/etc/meta")
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

	err = w.Exec("/usr/local/bin/brev", "setupworkspace")
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w TestWorkspace) Exec(arg ...string) error {
	arg = append([]string{"exec", w.ContainerName}, arg...)
	cmdM := exec.Command("docker", arg...) //nolint:gosec // for tests
	sendToOut(cmdM)
	err := cmdM.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w TestWorkspace) Copy(src string, dest string) error {
	cmdC := exec.Command("docker", "cp", src, dest) //nolint:gosec // for tests
	sendToOut(cmdC)
	err := cmdC.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w TestWorkspace) Done() error {
	cmd := exec.Command("docker", "kill", w.ContainerName) //nolint:gosec // for tests
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

	err = client.Test(func(workspace Workspace) {
		AssertGitProjectExistsWithBrev(t, workspace, "test-repo-dotbrev")
	})
	assert.Nil(t, err)
}

func AssertCodeServerRunningAuth(t *testing.T) {
}

func AssertCodeServerPasswordWorks(t *testing.T) {
}

func AssertSSHServerRunning(t *testing.T, w Workspace) {
	// test if connect no auth fails
	// test if connect pk works
}

func AssertsshServerPasswordAuthFail(t *testing.T) {
}

func AssertUserGitExists(t *testing.T) {
	// validate cloned, logs
}

func AssertGitProjectExistsWithBrev(t *testing.T, workspace Workspace, projectName string) {
	err := workspace.Exec("ls", projectName)
	assert.Nil(t, err)
	// validate cloned, logs
}

func AssertGitProjectExistsWithoutBrev(t *testing.T) {
	// validate cloned, logs
}

func AssertStringInLogs(t *testing.T, logsPath string, value string) {
}

func AssertNoGitProject(t *testing.T) {
}
