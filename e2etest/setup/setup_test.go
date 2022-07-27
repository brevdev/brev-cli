package setup

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/stretchr/testify/assert"
)

var testCMDDir = os.Getenv("BREV_SETUP_TEST_CMD_DIR")

func init() {
	cmd := exec.Command("/usr/bin/make", "fast-build")
	if testCMDDir == "" {
		testCMDDir = "/home/brev/workspace/brev-cli"
	}
	cmd.Dir = testCMDDir

	fmt.Printf("building binary in %s\n", testCMDDir)
	_, err := cmd.CombinedOutput()
	if err != nil {
		panic(err)
	}
}

const testConfig = "test-config"

func NewStdWorkspaceTestClient(setupParams *store.SetupParamsV0, containerParams []ContainerParams, options ...WorkspaceTestClientOption) *WorkspaceTestClient {
	pc, _, _, ok := runtime.Caller(1)
	if !ok {
		panic("not ok")
	}
	details := runtime.FuncForPC(pc)
	testNamePrefix := strings.Split(details.Name(), ".")[2]
	binPath := filepath.Join(testCMDDir, "bin")

	return NewWorkspaceTestClient(setupParams, containerParams, append([]WorkspaceTestClientOption{
		BrevBinaryPath{BinaryPath: binPath}, // TODO relativ path
		TestNamePrefix{Name: testNamePrefix},
	}, options...)...)
}

var SupportedContainers = []ContainerParams{
	{
		Name:  "brevdev-ubuntu-proxy-0.3.20",
		Image: "brevdev/ubuntu-proxy:0.3.20",
		Ports: []string{},
	},
}

func Test_UserBrevProjectBrevV0(t *testing.T) {
	keys, err := GetTestKeys(testCMDDir) // todo add path param with test dir env
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)
	params.ReposV0 = entity.ReposV0{
		testConfig: entity.RepoV0{
			Repository: "github.com:brevdev/test-repo-dotbrev.git",
			Directory:  testConfig,
		},
	}

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidUserBrevSetup(t, w, "user-dotbrev")
		AssertTestUserRepoSetupRan(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")

		AssertValidBrevProjRepo(t, w, testConfig)
		AssertTestRepoSetupRan(t, w, testConfig)

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertValidUserBrevSetup(t, w, "user-dotbrev")
		AssertTestUserRepoSetupRan(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")

		AssertValidBrevProjRepo(t, w, testConfig)
		AssertTestRepoSetupRan(t, w, testConfig)
	})

	assert.Nil(t, err)
}

func Test_UserBrevProjectBrevV1Minimal(t *testing.T) {
	keys, err := GetTestKeys(testCMDDir)
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)
	params.WorkspaceProjectRepo = ""
	params.ReposV1 = entity.ReposV1{
		"test-repo-dotbrev": entity.RepoV1{
			Type: entity.GitRepoType,
			GitRepo: entity.GitRepo{
				Repository:     "github.com:brevdev/test-repo-dotbrev.git",
				GitRepoOptions: entity.GitRepoOptions{},
			},
		},
	}

	execWorkDir := "test-repo-dotbrev"
	params.ExecsV1 = entity.ExecsV1{
		"test-config-setup": entity.ExecV1{
			Type: entity.PathExecType,
			ExecOptions: entity.ExecOptions{
				ExecWorkDir: &execWorkDir,
			},
			PathExec: entity.PathExec{
				ExecPath: "test-repo-dotbrev/.brev/setup.sh",
			},
		},
	}

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
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

func Test_UserBrevProjectBrevV1All(t *testing.T) {
	keys, err := GetTestKeys(testCMDDir)
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)
	dir := testConfig
	params.ReposV1 = entity.ReposV1{
		testConfig: entity.RepoV1{
			Type: entity.GitRepoType,
			GitRepo: entity.GitRepo{
				Repository: "github.com:brevdev/test-repo-dotbrev.git",
				GitRepoOptions: entity.GitRepoOptions{
					GitDirectory: &dir,
				},
			},
		},
	}

	execWorkDir := testConfig
	log := "test-config/.brev/logs"
	logA := "test-config/.brev/logs/archive"
	stage := entity.StartStage
	params.ExecsV1 = entity.ExecsV1{
		"test-config-setup": entity.ExecV1{
			Type:  entity.PathExecType,
			Stage: &stage,
			ExecOptions: entity.ExecOptions{
				ExecWorkDir:    &execWorkDir,
				LogPath:        &log,
				LogArchivePath: &logA,
			},
			PathExec: entity.PathExec{
				ExecPath: "test-config/.brev/setup.sh",
			},
		},
	}

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidUserBrevSetup(t, w, "user-dotbrev")
		AssertTestUserRepoSetupRan(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")

		AssertValidBrevProjRepo(t, w, testConfig)
		AssertTestRepoSetupRan(t, w, testConfig)

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertValidUserBrevSetup(t, w, "user-dotbrev")
		AssertTestUserRepoSetupRan(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")

		AssertValidBrevProjRepo(t, w, testConfig)
		AssertTestRepoSetupRan(t, w, testConfig)
	})

	assert.Nil(t, err)
}

func Test_NoProjectBrev(t *testing.T) {
	keys, err := GetTestKeys(testCMDDir)
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceProjectRepo = ""

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
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
	keys, err := GetTestKeys(testCMDDir)
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceBaseRepo = ""
	params.WorkspaceProjectRepo = ""

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertPathDoesNotExist(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "name")
	})
	assert.Nil(t, err)
}

func Test_NoUserBrevProj(t *testing.T) {
	keys, err := GetTestKeys(testCMDDir)
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceBaseRepo = ""

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
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

const testRepoNoDotBrev = "github.com:brevdev/test-repo-no-dotbrev.git"

func Test_ProjectRepoNoBrev(t *testing.T) {
	keys, err := GetTestKeys(testCMDDir)
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceProjectRepo = testRepoNoDotBrev

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
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

func Test_ProvidedSetupRanNoProj(t *testing.T) {
	keys, err := GetTestKeys(testCMDDir)
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceProjectRepo = ""
	setupScript := fmt.Sprintf("echo %s ", ProvidedSetupScriptMsg)
	base64SetupScript := base64.StdEncoding.EncodeToString([]byte(setupScript))
	params.ProjectSetupScript = &base64SetupScript

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, "name")
		AssertFileContainsString(t, w, ".brev/logs/setup.log", ProvidedSetupScriptMsg)

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, "name")
		AssertFileContainsString(t, w, ".brev/logs/setup.log", ProvidedSetupScriptMsg)
	})
	assert.Nil(t, err)
}

func Test_ProvidedSetupFileChange(t *testing.T) {
	keys, err := GetTestKeys(testCMDDir)
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceProjectRepo = testRepoNoDotBrev
	setupScript := fmt.Sprintf("echo %s ", ProvidedSetupScriptMsg)
	base64SetupScript := base64.StdEncoding.EncodeToString([]byte(setupScript))
	params.ProjectSetupScript = &base64SetupScript

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, "test-repo-no-dotbrev")
		AssertFileContainsString(t, w, ".brev/logs/setup.log", ProvidedSetupScriptMsg)

		// AssertRepoHasNumFiles(t, w, "/home/brev/workspace/.brev/logs/archive", 3)

		resetMessage := "reset run"
		err = UpdateFile(w, ".brev/setup.sh", fmt.Sprintf(" echo %s\n", resetMessage))
		assert.Nil(t, err)

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, "test-repo-no-dotbrev")
		AssertFileNotContainsString(t, w, ".brev/logs/setup.log", resetMessage)
		// AssertRepoHasNumFiles(t, w, "/home/brev/workspace/.brev/logs/archive", 4)
	})
	assert.Nil(t, err)
}

func Test_ProvidedSetupUpdated(t *testing.T) {
	keys, err := GetTestKeys(testCMDDir)
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)
	setupScript := fmt.Sprintf("echo %s ", ProvidedSetupScriptMsg)
	base64SetupScript := base64.StdEncoding.EncodeToString([]byte(setupScript))
	params.ProjectSetupScript = &base64SetupScript

	resetMsg := "updated setup script"

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidUserBrevSetup(t, w, "user-dotbrev")
		AssertTestUserRepoSetupRan(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")
		AssertFileContainsString(t, w, ".brev/logs/setup.log", ProvidedSetupScriptMsg)

		ss := fmt.Sprintf("echo %s ", resetMsg)
		params.ProjectSetupScript = &ss

		w.UpdateParams(params)

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidUserBrevSetup(t, w, "user-dotbrev")
		AssertTestUserRepoSetupRan(t, w, "user-dotbrev")

		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")
		AssertFileContainsString(t, w, ".brev/logs/setup.log", resetMsg)
	})
	assert.Nil(t, err)
}

func Test_UnauthenticatedSSHKey(t *testing.T) {
	noauthKeys, err := GetUnauthedTestKeys(testCMDDir)
	if !assert.Nil(t, err) {
		return
	}

	workskeys, err := GetTestKeys(testCMDDir)
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(noauthKeys)

	params.WorkspaceBaseRepo = ""
	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
		params.WorkspaceKeyPair = workskeys
		w.UpdateParams(params)
		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))
		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")
	})
	assert.Nil(t, err)
}

func Test_httpGit(t *testing.T) {
	noauthKeys, err := GetUnauthedTestKeys(testCMDDir)
	if !assert.Nil(t, err) {
		return
	}

	params := NewTestSetupParams(noauthKeys)
	params.WorkspaceBaseRepo = ""
	params.WorkspaceProjectRepo = "https://github.com/brevdev/test-repo-dotbrev.git"
	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		// AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))
		AssertValidBrevProjRepo(t, w, "test-repo-dotbrev")
		AssertTestRepoSetupRan(t, w, "test-repo-dotbrev")
	})
	assert.Nil(t, err)
}

func Test_VscodeExtension(t *testing.T) {
	keys, err := GetTestKeys(testCMDDir)
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)

	params.WorkspaceProjectRepo = testRepoNoDotBrev
	params.IDEConfigs = store.IDEConfigs{
		"vscode": {
			ExtensionIDs: []string{"golang.go@0.33.1"},
		},
	}

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, "test-repo-no-dotbrev")
		_, err = w.Exec("ls", "/home/brev/.vscode-server/extensions/golang.go-0.33.1")
		assert.Nil(t, err)

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, "test-repo-no-dotbrev")

		_, err = w.Exec("ls", "/home/brev/.vscode-server/extensions/golang.go-0.33.1")
		assert.Nil(t, err)
	})
	assert.Nil(t, err)
}

func Test_CustomBranchCustomSetupCustomFolder(t *testing.T) {
	keys, err := GetTestKeys(testCMDDir)
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)
	params.ProjectFolderName = "custom-folder"
	params.WorkspaceProjectRepoBranch = "other-branch"
	params.ProjectSetupExecPath = "scripts/my_custom_script.sh"
	params.WorkspaceBaseRepo = ""

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, params.ProjectFolderName)
		AssertCustomTestRepoSetupRan(t, w, params.ProjectFolderName, "custom setup script", "brev", filepath.Join("/home/brev/workspace", params.ProjectFolderName), "my_custom_script.log")

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, params.ProjectFolderName)
		AssertCustomTestRepoSetupRan(t, w, params.ProjectFolderName, "custom setup script", "brev", filepath.Join("/home/brev/workspace", params.ProjectFolderName), "my_custom_script.log")
	})
	assert.Nil(t, err)
}

func Test_ChangePwd(t *testing.T) {
	keys, err := GetTestKeys(testCMDDir)
	if !assert.Nil(t, err) {
		return
	}
	params := NewTestSetupParams(keys)
	params.WorkspaceProjectRepo = ""
	params.WorkspaceBaseRepo = ""

	params.ExecsV0 = entity.ExecsV0{
		"exec-name": entity.ExecV0{
			Exec:        "echo 'my exec ran'",
			ExecWorkDir: "test-repo-dotbrev",
		},
	}
	params.ReposV0 = entity.ReposV0{
		"repo-name": entity.RepoV0{
			Repository:    "github.com:brevdev/test-repo-dotbrev.git",
			Branch:        "",
			Directory:     "",
			BrevPath:      "",
			SetupExecPath: "",
			ExecWorkDir:   ".brev",
		},
	}
	folderName := "test-repo-dotbrev"

	client := NewStdWorkspaceTestClient(params, SupportedContainers)

	err = client.Test(func(w Workspace, err error) {
		assert.Nil(t, err)
		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, folderName)
		AssertCustomTestRepoSetupRan(t, w, folderName, "repo setup script ran", "brev", filepath.Join("/home/brev/workspace", folderName, ".brev"), "setup.log")
		AssertCustomTestRepoSetupRan(t, w, "/home/brev/workspace", "my exec ran", "brev", filepath.Join("/home/brev/workspace", folderName), "exec-name.log")

		err1 := w.Reset()
		if !assert.Nil(t, err1) {
			return
		}

		AssertWorkspaceSetup(t, w, params.WorkspacePassword, string(params.WorkspaceHost))

		AssertValidBrevProjRepo(t, w, folderName)
		AssertCustomTestRepoSetupRan(t, w, folderName, "repo setup script ran", "brev", filepath.Join("/home/brev/workspace", folderName, ".brev"), "setup.log")
		AssertCustomTestRepoSetupRan(t, w, "/home/brev/workspace", "my exec ran", "brev", filepath.Join("/home/brev/workspace", folderName), "exec-name.log")
	})
	assert.Nil(t, err)
}

func Test_CanClonePublicRepoWithoutAuthorizeddKeysAllFormats(t *testing.T) {
	noauthKeys, err := GetUnauthedTestKeys(testCMDDir)
	if !assert.Nil(t, err) {
		return
	}

	params := NewTestSetupParams(noauthKeys)
	dir := testConfig

	execWorkDir := testConfig
	log := "test-config/.brev/logs"
	logA := "test-config/.brev/logs/archive"
	stage := entity.StartStage
	params.ExecsV1 = entity.ExecsV1{
		"test-config-setup": entity.ExecV1{
			Type:  entity.PathExecType,
			Stage: &stage,
			ExecOptions: entity.ExecOptions{
				ExecWorkDir:    &execWorkDir,
				LogPath:        &log,
				LogArchivePath: &logA,
			},
			PathExec: entity.PathExec{
				ExecPath: "test-config/.brev/setup.sh",
			},
		},
	}

	// table tests
	type args struct {
		repo     string
		SSHURL   string
		HTTPURL  string
		HTTPSURL string
	}

	githubsshURL := "github.com:brevdev/test-repo-dotbrev.git"
	githubhttpURL := "http://github.com/brevdev/test-repo-dotbrev"
	githubhttpsURL := "https://github.com/brevdev/test-repo-dotbrev"

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "github",
			args: args{
				repo:     "github.com:brevdev/test-repo-dotbrev.git",
				SSHURL:   githubsshURL,
				HTTPURL:  githubhttpURL,
				HTTPSURL: githubhttpsURL,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params.ReposV1 = entity.ReposV1{
				testConfig: entity.RepoV1{
					Type: entity.GitRepoType,
					GitRepo: entity.GitRepo{
						Repository: tt.args.repo,
						SSHURL:     tt.args.SSHURL,
						HTTPURL:    tt.args.HTTPURL,
						HTTPSURL:   tt.args.HTTPSURL,
						GitRepoOptions: entity.GitRepoOptions{
							GitDirectory: &dir,
						},
					},
				},
			}
			fmt.Println("args: ", tt.args)
			client := NewStdWorkspaceTestClient(params, SupportedContainers)
			err := client.Test(func(w Workspace, err error) {
				AssertValidBrevProjRepo(t, w, testConfig)
			})
			if !tt.wantErr {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
			}
		})
	}
}
