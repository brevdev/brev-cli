package setupworkspace

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

func NewWorkspaceTestClient(setupParams *store.SetupParamsV0, containerParams []ContainerParams) *WorkspaceTestClient {
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		panic("not ok")
	}
	details := runtime.FuncForPC(pc)
	dbTestPrefix := strings.Split(details.Name(), ".")[2]

	// docker run -d --privileged=true --name brev-e2e-test --rm -i -t  brevdev/ubuntu-proxy:0.3.2 bash

	binPath, err := os.Executable()
	if err != nil {
		panic(err)
	}

	// [a-zA-Z0-9][a-zA-Z0-9_.-]
	workspaces := []Workspace{}
	for _, p := range containerParams {
		containerName := fmt.Sprintf("%s-%s", dbTestPrefix, p.Name)
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

var testKeyPair store.KeyPair = store.KeyPair{
	PublicKeyData:  `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQDx9bJSOoTJMSq08hcc69sjPfzfdXOn89e2zPO/xklffQz/b1QYpQAnMSXXI3dUT3/2xbGjJJfMZKb5JVXuOv0qUQp08dfPEidiC9fwbGMlXO6oiah95wqYOLaeBTjwgrsbIN4hNJSA8jolseM8LxJzb7rBWo2IsjMFYAZ0gRndPF7vUZLDF8wziPFZMsjg4AfQil1flhwMHygLxI3l5tF7sWF9Qj6PT8fgkWb3HbEslL1xm43tlHy5F7qfYuq++iSPE+OIn8HGOoc2XyFbTeABZy8rZekv5uyL+cC7qxmA33b86l+yDZecvbaTfHPH6fzDDF8468oSA4inEcph+akIgw23bj9O1Xh/dVt2qf3gBA13pX2N4qkAW+GNizw401zhU0elKQG0YJaXFASwKkNJr0MBQgx1uV/c9YpLwp2o7EKGk9y66DQfjtjPk0MsQKwbvTvdrBLK+xqTcSjgc+FjNrwdgsGVB8er+RjccwWT2bWm/J3EXpYAHjYzrmwyUzlEMeWiy1Snu//r5BLgx2sbCq20yF0jqBXWNJU6VCl+wfFbHVWRiZs4Pouc/OJY9S6BdqK1fSuf0co6pADyZ3RnvnOkGYmwTBxV7TF8xjRc/FE9arLaOKYMhEh2jnO8Hi9MzQZ2MdX9YBooK9jsAuZeQU8Xc6NcXI5quI5oiGNl8w== test+3333@gmail.com`,
	PrivateKeyData: `-----BEGIN RSA PRIVATE KEY-----\r\nMIIJKQIBAAKCAgEA8fWyUjqEyTEqtPIXHOvbIz3833Vzp\/PXtszzv8ZJX30M\/29U\r\nGKUAJzEl1yN3VE9\/9sWxoySXzGSm+SVV7jr9KlEKdPHXzxInYgvX8GxjJVzuqImo\r\nfecKmDi2ngU48IK7GyDeITSUgPI6JbHjPC8Sc2+6wVqNiLIzBWAGdIEZ3Txe71GS\r\nwxfMM4jxWTLI4OAH0IpdX5YcDB8oC8SN5ebRe7FhfUI+j0\/H4JFm9x2xLJS9cZuN\r\n7ZR8uRe6n2LqvvokjxPjiJ\/BxjqHNl8hW03gAWcvK2XpL+bsi\/nAu6sZgN92\/Opf\r\nsg2XnL22k3xzx+n8wwxfOOvKEgOIpxHKYfmpCIMNt24\/TtV4f3Vbdqn94AQNd6V9\r\njeKpAFvhjYs8ONNc4VNHpSkBtGCWlxQEsCpDSa9DAUIMdblf3PWKS8KdqOxChpPc\r\nuug0H47Yz5NDLECsG7073awSyvsak3Eo4HPhYza8HYLBlQfHq\/kY3HMFk9m1pvyd\r\nxF6WAB42M65sMlM5RDHlostUp7v\/6+QS4MdrGwqttMhdI6gV1jSVOlQpfsHxWx1V\r\nkYmbOD6LnPziWPUugXaitX0rn9HKOqQA8md0Z75zpBmJsEwcVe0xfMY0XPxRPWqy\r\n2jimDIRIdo5zvB4vTM0GdjHV\/WAaKCvY7ALmXkFPF3OjXFyOariOaIhjZfMCAwEA\r\nAQKCAgBJ1O1LBixKsEQV3uGKo7XEtT+aeF6IW3Hxh+zBEiKFjsUOlMwWkRLQ4sBn\r\nO51IDtI\/XOftnlbrubLxx6DHBx0FcqE8OifeOe3mjzKfXJOMbSUuLINLl9q1xGiF\r\nI5bSXTH2\/zqI62B9UGzJ39Q1IzAAJZHZOmaB1c6Xz0to9ZQM3EUjxcKA4ZwgBaOP\r\n0l1VsUbxLad8aXO5hzBTFvEtvBckJWZYMISienfFYNkIgvjzX5fHOO5AFtVQLZt2\r\n01pKWE4bkrgVkpowgN+Nic3F7Kd0BiZwmbJkOZToyZc0LOulVYsbdfEphdhDregu\r\nbQVvdUj5w6ay2dBZWtayFE2rQ+F97YvTl+Q1AJ8OabydZPEvgIS54P5vDpjg6wWo\r\nnUqn7S2dNVmsuynqShFDkO7\/gMI2dJTC8\/nSHO7i9BWuoPTeeWqnih5gpnoLHgZ+\r\nZDUqbrrE8TZEiIUnfZk+s+8iyWt5KRg9Wp8psj1bXfxxZBbTSVn4K\/iB4MZA1OiC\r\nC\/LcMp0Vi6SKbob6jhJqGoGHrOzs3S+5NP7eLPc9dfGzWbopvbwXhp1p0rdmaHGc\r\nhxYTpDGDWbbKNKqXpMZM1Lf6TPzerIvvzn+pLXN0XAMI7UmANdmuYsQIOA93c3lz\r\nHUcj05AumHtIkKdleUbUKGLYHdAgk9FLqkpGic56LOvYx6\/K0QKCAQEA+I8LvUfI\r\nelpwGbvOKltnsyirmHNwbDX9ujoHguPNHiW0hTn3OJ\/VKxJPsGU+yRLYMpa0vCEU\r\nzS0g4V+CzDR2RDd7kn30bPZRQe8KfedO8hnmMM4AQaOqin\/rl+aBV5WlLxOKZS+X\r\nLjIbcjnVymK8pbjHlhqcvHLl2G6mfh1yXny39KroogG61HRVFrEXNQGmqYX1yg9f\r\nvVLla4TelB6Dv6uHuBGag03JCZaomEMKJxG+wzNE4wVBKJYl6ZuJjNn9kTxAIIUo\r\nD4gvhGLb\/xveVuiy7WoUkM4ROj7ShGviBsojRplDYr1Ji5jao6b\/BN0rf5WlrRy\/\r\nus07xKizJQuwdwKCAQEA+TQTaS4L\/Z26Cj7dtNRP\/ulS2hytIY1PcL8BednTeigD\r\nazdcAARnajZmi4EfteloAP9B1mDeHAlcxgurNEXswf1b5kilFbFa\/FI\/okfv\/4VI\r\nuaC3AHgWZN5la6gh6c3l9m+Gw5ik6g8NweM1CzQD4JWL\/PG79Urk4FnBaX38E9pS\r\nUR8Z4C9Zf0DuBLa7\/yn2hBHTQurspyCDMPhGgeLXrbw4siNQ6d4z1iZ7sZNrPwE+\r\nXwWZkr2TE9a9NJzGXiJX07pEKzSeMHx4WCxY1JUvhFTMYnkY3N7f\/B4W9bLwj7oi\r\nlAx1ZuuQQnBIzjsVxM+jgtib\/9nIsVvzfFVDQgAxZQKCAQEAnyagoqrS4B0GSEPr\r\nZ02toZa6ANxxsKgFdXdwlcuc69\/CrceG13fn+zM3WUAKqp7pVcMPqKIZ+qIZupT4\r\nYB57V4SbGBqUJiy1rN0NP76a2wPgU4GjwmO0cAgmZtXOHbGQ2grOA6osSAUHc+U6\r\nUeNU3VvqV99kWnnLWADJlFjwgTWkaAIDALDQ2vY+AVCVBnivKT7AOYgMimIIygaC\r\nqh67xz9ioGaNI+PrhLs16oCKgKepGL28LwyPQxiY3\/KaaVivNo54lRoNo5xUqJTQ\r\nPpGulMFcyA2za2C2wS+2hdm6GRTW7351GkUPUVYnMMBd69Rd5MyCD80nqsl8qphG\r\nVMMeUwKCAQB6\/NB3oFoamLUwSUZx8DZqwAw7yNtJK8yBAENiN7a\/GvBVAcVN3N6M\r\n9Lw3LUrRJJhHpbKAct4rSBOZSjj8W2Y1dyzbwg53XkhhLtZo6Mfxe34g3shyWtHy\r\nhi\/XqerS0OMldHU2IyeAvF01y0RqewlO1X95HnR84rGCZ8mknqDBy4XEs2y5z6SD\r\nwS+289hkXfljxMhWxkp1UP5uNJnXkHSRMctpXzSXtyouDmANi4vqVFrL2p+oZBcq\r\nO1i1lonv+1MNE2iBSj6n\/0YFfh15DQeeb5tPHiS\/HN++NbtvFxjSVjKqjluCp89S\r\nesfzwAVGVJOGCBE1e+4oWhEY05uV\/zJhAoIBAQDcCH9tLfjSrbdJwsGRRu5eeGy+\r\ndI4oBrD+YmQ48EH01JHunQT9W9XdR6xqomTRf02Ga3oanQQoZgpGXc0yijIaMhIQ\r\nu+Edi9VJCm0rj\/FK6swypksySKhLMQ7DtUvfaP0OlyrJNsCuEEB+kOmkxFkCdjOL\r\ndfRbsZWJh9OcXTQjIvt7Yqs2YHKCG8bqJGlAxp0kOredB3Y2jy7sdgEQKGA3fCQ5\r\nT+aiiljGAP0gBs0Q0r2HToLZowakYd3cjOo\/LCT+0pgdqmxsrBEQKkmVbbERWGIh\r\nOMjWggsIs8S+IyKC8DRFC9JAA45LAmhNedCOnvoQx+wBJajyMZIadr01VYd+\r\n-----END RSA PRIVATE KEY-----\r\n`,
}

var testSetupParamsV0 store.SetupParamsV0 = store.SetupParamsV0{
	WorkspaceHost:                    "name-rand-org.x.y",
	WorkspacePort:                    22778,
	WorkspaceBaseRepo:                "github.com:brevdev/test-repo-user-config.git",
	WorkspaceProjectRepo:             "github.com:brevdev/test-repo-dotbrev.git",
	WorkspaceApplicationStartScripts: []string{},
	WorkspaceUsername:                "brevtest",
	WorkspaceEmail:                   "test+3333@gmail.com",
	WorkspacePassword:                "12345",
	WorkspaceKeyPair:                 &testKeyPair,
	SetupScript:                      nil,
}

func Test_UserBrevProjectBrev(t *testing.T) {
	client := NewWorkspaceTestClient(&testSetupParamsV0, []ContainerParams{
		{
			Name:  "brevdev-ubuntu-proxy-0.3.2",
			Image: "brevdev/ubuntu-proxy:0.3.2",
			Port:  "22778",
		},
	})

	err := client.Test(func(workspace Workspace) {
		err := workspace.Exec("ls", "test-repo-dotbrev")
		assert.Nil(t, err)
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

func AssertGitProjectExistsWithBrev(t *testing.T) {
	// validate cloned, logs
}

func AssertGitProjectExistsWithoutBrev(t *testing.T) {
	// validate cloned, logs
}

func AssertStringInLogs(t *testing.T, logsPath string, value string) {
}

func AssertNoGitProject(t *testing.T) {
}
