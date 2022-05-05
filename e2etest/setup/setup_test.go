package setup

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

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
	Ports []string
}

func init() {
	fmt.Println("building binary")
	cmd := exec.Command("/usr/bin/make")
	cmd.Dir = "/home/brev/workspace/brev-cli" // TODO relative path
	_, err := cmd.CombinedOutput()
	if err != nil {
		panic(err)
	}
}

func NewWorkspaceTestClient(setupParams *store.SetupParamsV0, containerParams []ContainerParams) *WorkspaceTestClient {
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		panic("not ok")
	}
	details := runtime.FuncForPC(pc)
	testPrefix := strings.Split(details.Name(), ".")[2]

	binPath := "/home/brev/workspace/brev-cli/brev" // TODO how to relative path

	workspaces := []Workspace{}
	for _, p := range containerParams {
		containerName := fmt.Sprintf("%s-%s", testPrefix, p.Name)
		// [a-zA-Z0-9][a-zA-Z0-9_.-]
		workspace := *NewTestWorkspace(binPath, containerName, p.Image, p.Ports, setupParams)
		_ = workspace.Done()
		workspace.ShowOut = true
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
	Reset() error
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
	Ports          []string
	TestBrevBinary string // path to brev binary that should be tested
	ShowOut        bool
}

var _ Workspace = TestWorkspace{}

// image := "brevdev/ubuntu-proxy:0.3.2"
func NewTestWorkspace(testBrevBinaryPath string, containerName string, image string, ports []string, setupParams *store.SetupParamsV0) *TestWorkspace {
	return &TestWorkspace{SetupParams: setupParams, ContainerName: containerName, Ports: ports, Image: image, TestBrevBinary: testBrevBinaryPath}
}

func sendToOut(c *exec.Cmd) {
	c.Stderr = os.Stderr
	c.Stdout = os.Stdout
}

func (w TestWorkspace) getWorkspaceVolumeName() string {
	return fmt.Sprintf("%s-workspace", w.ContainerName)
}

func (w TestWorkspace) Setup() error {
	ports := []string{}
	for _, p := range w.Ports {
		ports = append(ports, "-p", p)
	}

	args := append([]string{
		"run", "-d",
		"--privileged=true",
		fmt.Sprintf("--name=%s", w.ContainerName),
		"--rm", "-it", w.Image,
		"-v", fmt.Sprintf("%s:/home/brev/workspace", w.getWorkspaceVolumeName()),
	}, ports...)
	args = append(args, "bash")

	cmdR := exec.Command("docker", args...) //nolint:gosec // for tests
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
	cmdM := exec.Command("docker", arg...)
	out, err := cmdM.CombinedOutput()
	if w.ShowOut {
		fmt.Print(string(out))
	}
	if err != nil {
		return out, breverrors.WrapAndTrace(err)
	}
	return out, nil
}

func (w TestWorkspace) Copy(src string, dest string) error {
	cmdC := exec.Command("docker", "cp", src, dest)
	if w.ShowOut {
		sendToOut(cmdC)
	}
	err := cmdC.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w TestWorkspace) Reset() error {
	err := w.KillContainer()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = w.Setup()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w TestWorkspace) KillContainer() error {
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

func (w TestWorkspace) RmVolume() error {
	cmd := exec.Command("docker", "volume", "rm", w.getWorkspaceVolumeName()) //nolint:gosec // for tests
	if w.ShowOut {
		sendToOut(cmd)
	}
	err := cmd.Run()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (w TestWorkspace) Done() error {
	err := w.KillContainer()
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	err = w.RmVolume()
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

var SupportedContainers = []ContainerParams{
	{
		Name:  "brevdev-ubuntu-proxy-0.3.2",
		Image: "brevdev/ubuntu-proxy:0.3.2",
		Ports: []string{"22778:22778", "22779:22779", "2222:22"},
	},
}

// TODO too many redudant assertions

func Test_UserBrevProjectBrev(t *testing.T) {
	keys, err := GetTestKeys()
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	client := NewWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace) {
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidUserBrevSetup(t, w, "user-dotbrev")
		AssertTestUserRepoSetupRan(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertValidUserBrevSetup(t, w, "user-dotbrev")
		AssertTestUserRepoSetupRan(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")
	})

	assert.Nil(t, err)
}

func Test_NoProjectBrev(t *testing.T) {
	keys, err := GetTestKeys()
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceProjectRepo = ""

	client := NewWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace) {
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidUserBrevSetup(t, w, "user-dotbrev")
		AssertValidBrevProjRepo(t, w, "name")

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidUserBrevSetup(t, w, "user-dotbrev")
		AssertValidBrevProjRepo(t, w, "name")
	})
	assert.Nil(t, err)
}

func Test_NoUserBrevNoProj(t *testing.T) {
	keys, err := GetTestKeys()
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceBaseRepo = ""
	params.WorkspaceProjectRepo = ""

	client := NewWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace) {
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertPathDoesNotExist(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "name")
	})
	assert.Nil(t, err)
}

func Test_NoUserBrevProj(t *testing.T) {
	keys, err := GetTestKeys()
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceBaseRepo = ""

	client := NewWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace) {
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertPathDoesNotExist(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertPathDoesNotExist(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")
	})
	assert.Nil(t, err)
}

func Test_ProjectRepoNoBrev(t *testing.T) {
	keys, err := GetTestKeys()
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceProjectRepo = "github.com:brevdev/test-repo-no-dotbrev.git"

	client := NewWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace) {
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, "test-repo-no-dotbrev")

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, "test-repo-no-dotbrev")
	})
	assert.Nil(t, err)
}

const ProvidedSetupScriptMsg = "provided setup script ran"

func Test_ProvidedSetupRanNoProj(t *testing.T) { //nolint:dupl // TODO should refactor since Test_ProvidedSetupRanProjNoBrev is really similar
	keys, err := GetTestKeys()
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceProjectRepo = ""
	setupScript := fmt.Sprintf("echo %s ", ProvidedSetupScriptMsg)
	base64SetupScript := base64.StdEncoding.EncodeToString([]byte(setupScript))
	params.SetupScript = &base64SetupScript

	client := NewWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace) {
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, "name")
		AssertFileContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", "name"), ProvidedSetupScriptMsg)

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, "name")
		AssertFileContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", "name"), ProvidedSetupScriptMsg)
	})
	assert.Nil(t, err)
}

func Test_ProvidedSetupRanProjNoBrev(t *testing.T) { //nolint:dupl // TODO should refactor since Test_ProvidedSetupRanProjNoBrev is really similar
	keys, err := GetTestKeys()
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceProjectRepo = "github.com:brevdev/test-repo-no-dotbrev.git"
	setupScript := fmt.Sprintf("echo %s ", ProvidedSetupScriptMsg)
	base64SetupScript := base64.StdEncoding.EncodeToString([]byte(setupScript))
	params.SetupScript = &base64SetupScript

	client := NewWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace) {
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, "test-repo-no-dotbrev")
		AssertFileContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", "test-repo-no-dotbrev"), ProvidedSetupScriptMsg)

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, "test-repo-no-dotbrev")
		AssertFileContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", "test-repo-no-dotbrev"), ProvidedSetupScriptMsg)
	})
	assert.Nil(t, err)
}

func Test_ProvidedSetupNotRunProjBrev(t *testing.T) {
	keys, err := GetTestKeys()
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)
	setupScript := fmt.Sprintf("echo %s ", ProvidedSetupScriptMsg)
	base64SetupScript := base64.StdEncoding.EncodeToString([]byte(setupScript))
	params.SetupScript = &base64SetupScript

	client := NewWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace) {
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidUserBrevSetup(t, w, "user-dotbrev")
		AssertTestUserRepoSetupRan(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")
		AssertFileNotContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", "test-repo-dotbrev"), ProvidedSetupScriptMsg)

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidUserBrevSetup(t, w, "user-dotbrev")
		AssertTestUserRepoSetupRan(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")
		AssertFileNotContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", "test-repo-dotbrev"), ProvidedSetupScriptMsg)
	})
	assert.Nil(t, err)
}

func Test_UnauthenticatedSSHKey(_ *testing.T) {
	// uses an ssh key that is not connected to github
}

func AssertWorkspaceSetup(t *testing.T, w Workspace, password string, host string) {
	t.Helper()
	AssertUser(t, w, "root")
	AssertCwd(t, w, "/home/brev/workspace")

	time.Sleep(1000 * time.Millisecond) // sometimes localhost:2278 returs bad error

	AssertInternalCurlOuputContains(t, w, "localhost:22778", "Found. Redirecting to ./login")
	AssertInternalCurlOuputContains(t, w, "localhost:22779/proxy", "Bad Request")
	AssertFileContainsString(t, w, "/home/brev/.config/code-server/config.yaml", password)
	AssertFileContainsString(t, w, "/home/brev/.config/code-server/config.yaml", host)
	AssertInternalSSHServerRunning(t, w, "/home/brev/.ssh/id_rsa", "brev", "ls")
}

func AssertValidBrevBaseRepoSetup(t *testing.T, w Workspace, repoPath string) {
	t.Helper()
	AssertPathExists(t, w, repoPath)
	AssertPathExists(t, w, fmt.Sprintf("%s/.git", repoPath))
	AssertPathExists(t, w, fmt.Sprintf("%s/.brev", repoPath))
	AssertPathExists(t, w, fmt.Sprintf("%s/.brev/setup.sh", repoPath))
	AssertPathExists(t, w, fmt.Sprintf("%s/.brev/logs", repoPath))
}

func AssertValidUserBrevSetup(t *testing.T, w Workspace, repoPath string) {
	t.Helper()
	AssertValidBrevBaseRepoSetup(t, w, repoPath)
	AssertPathNotExist(t, w, fmt.Sprintf("%s/.brev/ports.yaml", repoPath))
}

func AssertTestUserRepoSetupRan(t *testing.T, w Workspace, repoPath string) {
	t.Helper()
	AssertFileContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", repoPath), "user setup script ran")
}

func AssertValidBrevProjRepo(t *testing.T, w Workspace, repoPath string) {
	t.Helper()
	AssertValidBrevBaseRepoSetup(t, w, repoPath)

	AssertPathExists(t, w, fmt.Sprintf("%s/.brev/.gitignore", repoPath))
	AssertPathExists(t, w, fmt.Sprintf("%s/.brev/ports.yaml", repoPath))
}

func AssertTestRepoSetupRan(t *testing.T, w Workspace, repoPath string) {
	t.Helper()
	AssertFileContainsString(t, w, fmt.Sprintf("%s/.brev/logs/setup.log", repoPath), "repo setup script ran")
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
	o := string(out)
	assert.Contains(t, o, contains)
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

func AssertPathExists(t *testing.T, workspace Workspace, path string) bool {
	t.Helper()
	_, err := workspace.Exec("ls", path)
	return assert.Nil(t, err)
}

func AssertPathNotExist(t *testing.T, workspace Workspace, path string) bool {
	t.Helper()
	_, err := workspace.Exec("ls", path)
	return assert.NotNil(t, err)
}

func AssertPathDoesNotExist(t *testing.T, workspace Workspace, path string) bool {
	t.Helper()
	_, err := workspace.Exec("ls", path)
	return assert.Error(t, err)
}

func AssertFileContainsString(t *testing.T, w Workspace, filePath string, contains string) bool {
	t.Helper()

	res, err := w.Exec("grep", contains, filePath)
	assert.Contains(t, string(res), contains)
	return assert.Nil(t, err)
}

func AssertFileNotContainsString(t *testing.T, w Workspace, filePath string, contains string) bool {
	t.Helper()

	_, err := w.Exec("grep", contains, filePath)
	return assert.Error(t, err)
}
