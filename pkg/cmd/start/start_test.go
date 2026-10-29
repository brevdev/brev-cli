package start

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/brevdev/brev-cli/pkg/terminal"
	resty "github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingCreateStartStore struct {
	StartStore
	createErr error
}

func (s failingCreateStartStore) GetAuthTokens() (*entity.AuthTokens, error) {
	return nil, nil
}

func (s failingCreateStartStore) GetCurrentUser() (*entity.User, error) {
	return &entity.User{ID: "user-1"}, nil
}

func (s failingCreateStartStore) GetActiveOrganizationOrDefault() (*entity.Organization, error) {
	return &entity.Organization{ID: "org-1"}, nil
}

func (s failingCreateStartStore) CreateWorkspace(_ string, _ *store.CreateWorkspacesOptions) (*entity.Workspace, error) {
	return nil, s.createErr
}

func startHTTPError(t *testing.T, status int, body string) error {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	response, err := resty.New().R().Post(server.URL)
	require.NoError(t, err)
	return fmt.Errorf("create failed: %w", store.NewHTTPResponseError(response))
}

func TestRunSingleStartDuplicateGuidanceUsesWorkspaceCode(t *testing.T) {
	cases := []struct {
		name         string
		status       int
		body         string
		wantGuidance bool
	}{
		{"coded duplicate", http.StatusBadRequest, `{"errors":[{"type":"BadRequestError","code":"WORKSPACE_ALREADY_EXISTS","message":"Bad Request"}]}`, true},
		{"generic bad request", http.StatusBadRequest, `{"errors":[{"type":"BadRequestError","message":"Bad Request"}]}`, false},
		{"unrelated error with duplicate words", http.StatusBadRequest, `{"errors":[{"type":"BadRequestError","code":"OTHER","message":"duplicate instance with name"}]}`, false},
		{"wrong status", http.StatusInternalServerError, `{"errors":[{"type":"InternalServerError","code":"WORKSPACE_ALREADY_EXISTS","message":"Internal Server Error"}]}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			createErr := startHTTPError(t, tc.status, tc.body)
			read, write, err := os.Pipe()
			require.NoError(t, err)
			original := os.Stdout
			os.Stdout = write
			t.Cleanup(func() {
				os.Stdout = original
				_ = read.Close()
				_ = write.Close()
			})
			term := terminal.New()
			runErr := runSingleStart(term, nil, "existing-name", "", "", "", "", "", true, false, failingCreateStartStore{createErr: createErr})
			require.Error(t, runErr)
			require.ErrorContains(t, runErr, "create failed")
			require.NoError(t, write.Close())
			var output bytes.Buffer
			_, err = output.ReadFrom(read)
			require.NoError(t, err)
			assert.Equal(t, tc.wantGuidance, strings.Contains(output.String(), "brev delete [name]"), output.String())
		})
	}
}

func TestMakeNewWorkspaceFromURL(t *testing.T) {
	gitTruth := "github.com:brevdev/brev-cli.git"
	nameTruth := "brev-cli"

	wksTruth := NewWorkspace{
		Name:    nameTruth,
		GitRepo: gitTruth,
	}

	naked := "https://github.com/brevdev/brev-cli"
	res := MakeNewWorkspaceFromURL(naked)
	if !assert.Equal(t, wksTruth, res) {
		return
	}

	http := "http://github.com/brevdev/brev-cli.git"
	res = MakeNewWorkspaceFromURL(http)
	if !assert.Equal(t, wksTruth, res) {
		return
	}

	https := "https://github.com/brevdev/brev-cli.git"
	res = MakeNewWorkspaceFromURL(https)
	if !assert.Equal(t, wksTruth, res) {
		return
	}

	ssh := "git@github.com:brevdev/brev-cli.git"
	res = MakeNewWorkspaceFromURL(ssh)
	if !assert.Equal(t, wksTruth, res) {
		return
	}
}

func Test_DisplayBC(t *testing.T) {
	term := terminal.New()
	displayConnectBreadCrumb(term, &entity.Workspace{
		ID:                "123456789",
		Name:              "my-name",
		OrganizationID:    "",
		WorkspaceClassID:  "",
		CreatedByUserID:   "",
		DNS:               "",
		Status:            "",
		Password:          "",
		GitRepo:           "",
		Version:           "",
		WorkspaceTemplate: entity.WorkspaceTemplate{},
		NetworkID:         "",
	})
}

func TestResolveWorkspaceUserOptions_APIKeyAuthUsesUserDefaultsWithoutUser(t *testing.T) {
	got := resolveWorkspaceUserOptions(&store.CreateWorkspacesOptions{}, nil, true)

	assert.Equal(t, store.UserWorkspaceTemplateID, got.WorkspaceTemplateID)
	assert.Equal(t, store.UserWorkspaceClassID, got.WorkspaceClassID)
}

func TestResolveWorkspaceUserOptions_AdminUserUsesDevDefaults(t *testing.T) {
	got := resolveWorkspaceUserOptions(&store.CreateWorkspacesOptions{}, &entity.User{GlobalUserType: entity.Admin}, false)

	assert.Equal(t, store.DevWorkspaceTemplateID, got.WorkspaceTemplateID)
	assert.Equal(t, store.DevWorkspaceClassID, got.WorkspaceClassID)
}
