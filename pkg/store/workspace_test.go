package store

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	nodev1connect "buf.build/gen/go/brevdev/devplane/connectrpc/go/devplaneapi/v1/devplaneapiv1connect"
	nodev1 "buf.build/gen/go/brevdev/devplane/protocolbuffers/go/devplaneapi/v1"
	"connectrpc.com/connect"
	"github.com/brevdev/brev-cli/pkg/entity"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeEnvironmentService struct {
	nodev1connect.UnimplementedEnvironmentServiceHandler
	listFn func(http.Header, *nodev1.ListEnvironmentRequest) (*nodev1.ListEnvironmentResponse, error)
}

func (f *fakeEnvironmentService) ListEnvironment(_ context.Context, req *connect.Request[nodev1.ListEnvironmentRequest]) (*connect.Response[nodev1.ListEnvironmentResponse], error) {
	resp, err := f.listFn(req.Header(), req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(resp), nil
}

func TestGetWorkspaces(t *testing.T) {
	var gotAuth string
	var gotOrgID string
	var gotSSHAccess bool
	svc := &fakeEnvironmentService{listFn: func(header http.Header, msg *nodev1.ListEnvironmentRequest) (*nodev1.ListEnvironmentResponse, error) {
		gotAuth = header.Get("Authorization")
		gotOrgID = msg.GetOptions().GetHasAllLabels()["organizationId"]
		gotSSHAccess = msg.GetOptions().GetAttachedDataOptions().GetSshAccess()
		return &nodev1.ListEnvironmentResponse{Items: []*nodev1.Environment{{
			EnvironmentId: "env-1",
			Name:          "name",
			Labels: map[string]string{
				"organizationId":   "o1",
				"userId":           "user-1",
				"workspaceGroupId": "group-1",
			},
			Status: nodev1.EnvironmentStatus_ENVIRONMENT_STATUS_RUNNING,
			Instance: &nodev1.Instance{
				InstanceType: "gpu-h100",
				PublicDns:    "env.example.com",
				SshUser:      "ubuntu",
				SshPort:      22,
				IsStoppable:  true,
			},
		}}}, nil
	}}
	_, handler := nodev1connect.NewEnvironmentServiceHandler(svc)
	server := httptest.NewServer(handler)
	defer server.Close()
	t.Setenv("BREV_PUBLIC_API_URL", server.URL)

	legacyRESTServer := httptest.NewServer(http.NotFoundHandler())
	defer legacyRESTServer.Close()
	token := "tok"
	fileStore, _, _ := newAuthTokenTestStore(t)
	s := fileStore.WithAuthHTTPClient(NewAuthHTTPClient(MockAuth{token: &token}, legacyRESTServer.URL))

	workspaces, err := s.GetWorkspaces("o1", nil)
	require.NoError(t, err)
	require.Len(t, workspaces, 1)
	assert.Equal(t, "Bearer tok", gotAuth)
	assert.Equal(t, "o1", gotOrgID)
	assert.True(t, gotSSHAccess)
	assert.Equal(t, entity.Workspace{
		ID:               "env-1",
		Name:             "name",
		WorkspaceGroupID: "group-1",
		OrganizationID:   "o1",
		CreatedByUserID:  "user-1",
		InstanceType:     "gpu-h100",
		DNS:              "env.example.com",
		Status:           entity.Running,
		SSHUser:          "ubuntu",
		SSHPort:          22,
		HostSSHUser:      "ubuntu",
		HostSSHPort:      22,
		IsStoppable:      true,
		HealthStatus:     entity.Unavailable,
		Version:          "v1",
	}, workspaces[0])
}

func TestMapEnvironmentToWorkspacePreservesRuntimeAccessData(t *testing.T) {
	older := timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	newer := timestamppb.New(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))

	workspace := mapEnvironmentToWorkspace(&nodev1.Environment{
		EnvironmentId: "env-1",
		Namespace:     "org-from-namespace",
		Labels:        map[string]string{"userId": "owner"},
		Status:        nodev1.EnvironmentStatus_ENVIRONMENT_STATUS_RUNNING,
		SysUsers: []*nodev1.SysUser{
			{UserType: nodev1.UserType_USER_TYPE_HOST, Username: "host-user", Port: 2222, SshProxyHostname: "host-proxy"},
			{UserType: nodev1.UserType_USER_TYPE_CONTAINER, Username: "container-user", Port: 2200, SshProxyHostname: "container-proxy"},
		},
		SshAccess: []*nodev1.SSHAccess{
			{UserId: "owner"},
			{UserId: "shared-user"},
			{UserId: "shared-user"},
		},
		Tasks: []*nodev1.Task{
			{Name: "environment-build", Status: "failed", CreateTime: newer},
			{Name: "environment-build", Status: "running", CreateTime: older},
		},
	})

	assert.Equal(t, "org-from-namespace", workspace.OrganizationID)
	assert.Equal(t, "container-user", workspace.SSHUser)
	assert.Equal(t, 2200, workspace.SSHPort)
	assert.Equal(t, "container-proxy", workspace.SSHProxyHostname)
	assert.Equal(t, "host-user", workspace.HostSSHUser)
	assert.Equal(t, 2222, workspace.HostSSHPort)
	assert.Equal(t, "host-proxy", workspace.HostSSHProxyHostname)
	assert.Equal(t, []string{"shared-user"}, workspace.AdditionalUsers)
	assert.Equal(t, entity.CreateFailed, workspace.VerbBuildStatus)
}

func TestCreateWorkspace(t *testing.T) {
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	orgID := "o1"
	expected := &entity.Workspace{
		ID:               "1",
		Name:             "name",
		WorkspaceGroupID: "wgi",
		OrganizationID:   orgID,
		WorkspaceClassID: "wci",
		CreatedByUserID:  "cbuid",
		DNS:              "dns",
		Status:           "s",
		Password:         "pw",
		GitRepo:          "g",
	}
	res, err := httpmock.NewJsonResponder(201, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", s.authHTTPClient.restyClient.BaseURL, fmt.Sprintf(workspaceOrgPathPattern, orgID))
	httpmock.RegisterResponder("POST", url, res)

	u, err := s.CreateWorkspace(orgID, NewCreateWorkspacesOptions("wgi", "name"))
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, u) {
		return
	}

	if !assert.Equal(t, expected, u) {
		return
	}
}

func TestGetWorkspacesWithName(t *testing.T) { //nolint:dupl // To refactor later, not fully duplicate code
	orgID := "o1"
	expected := []entity.Workspace{{
		ID:               "1",
		Name:             "name",
		WorkspaceGroupID: "wgi",
		OrganizationID:   orgID,
		CreatedByUserID:  "blas",
		Status:           entity.Running,
		SSHPort:          22,
		SSHUser:          entity.DefaultUser,
		HostSSHPort:      22,
		HostSSHUser:      entity.DefaultUser,
		HealthStatus:     entity.Unavailable,
		Version:          "v1",
	}}
	s := makeEnvironmentStore(t, []*nodev1.Environment{
		{EnvironmentId: "2", Name: "n2", Labels: map[string]string{"organizationId": orgID, "userId": "other", "workspaceGroupId": "wgi"}, Status: nodev1.EnvironmentStatus_ENVIRONMENT_STATUS_RUNNING},
		{EnvironmentId: "1", Name: "name", Labels: map[string]string{"organizationId": orgID, "userId": "blas", "workspaceGroupId": "wgi"}, Status: nodev1.EnvironmentStatus_ENVIRONMENT_STATUS_RUNNING},
	})

	w, err := s.GetWorkspaces(orgID, &GetWorkspacesOptions{
		Name: "name",
	})
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, w) {
		return
	}
	if !assert.Len(t, w, 1) {
		return
	}
	if !assert.Equal(t, expected, w) {
		return
	}
}

func TestGetWorkspacesWithUser(t *testing.T) { //nolint:dupl // To refactor later, not fully duplicate code
	orgID := "o1"
	expected := []entity.Workspace{{
		ID:               "1",
		Name:             "name",
		WorkspaceGroupID: "wgi",
		OrganizationID:   orgID,
		CreatedByUserID:  "me",
		Status:           entity.Running,
		SSHPort:          22,
		SSHUser:          entity.DefaultUser,
		HostSSHPort:      22,
		HostSSHUser:      entity.DefaultUser,
		HealthStatus:     entity.Unavailable,
		Version:          "v1",
	}}
	s := makeEnvironmentStore(t, []*nodev1.Environment{
		{EnvironmentId: "2", Name: "n2", Labels: map[string]string{"organizationId": orgID, "userId": "other", "workspaceGroupId": "wgi"}, Status: nodev1.EnvironmentStatus_ENVIRONMENT_STATUS_RUNNING},
		{EnvironmentId: "1", Name: "name", Labels: map[string]string{"organizationId": orgID, "userId": "me", "workspaceGroupId": "wgi"}, Status: nodev1.EnvironmentStatus_ENVIRONMENT_STATUS_RUNNING},
	})

	u, err := s.GetWorkspaces(orgID, &GetWorkspacesOptions{
		UserID: "me",
	})
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, u) {
		return
	}
	if !assert.Len(t, u, 1) {
		return
	}
	if !assert.Equal(t, expected, u) {
		return
	}
}

func makeEnvironmentStore(t *testing.T, environments []*nodev1.Environment) *AuthHTTPStore {
	t.Helper()
	svc := &fakeEnvironmentService{listFn: func(_ http.Header, _ *nodev1.ListEnvironmentRequest) (*nodev1.ListEnvironmentResponse, error) {
		return &nodev1.ListEnvironmentResponse{Items: environments}, nil
	}}
	_, handler := nodev1connect.NewEnvironmentServiceHandler(svc)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("BREV_PUBLIC_API_URL", server.URL)

	legacyRESTServer := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(legacyRESTServer.Close)
	token := "tok"
	fileStore, _, _ := newAuthTokenTestStore(t)
	return fileStore.WithAuthHTTPClient(NewAuthHTTPClient(MockAuth{token: &token}, legacyRESTServer.URL))
}

func TestGetWorkspaceMetaData(t *testing.T) {
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	workspaceID := "1"
	expected := &entity.WorkspaceMetaData{
		PodName:       "pn",
		NamespaceName: "nsn",
	}

	res, err := httpmock.NewJsonResponder(200, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", s.authHTTPClient.restyClient.BaseURL, fmt.Sprintf(workspaceMetadataPathPattern, workspaceID))
	httpmock.RegisterResponder("GET", url, res)

	u, err := s.GetWorkspaceMetaData(workspaceID)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, u) {
		return
	}
	if !assert.Equal(t, expected, u) {
		return
	}
}

func TestStopWorkspace(t *testing.T) { //nolint:dupl // ok to have this be duplicate
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	workspaceID := "1"
	expected := &entity.Workspace{
		ID:               workspaceID,
		Name:             "name",
		WorkspaceGroupID: "wgi",
		OrganizationID:   "oi",
		WorkspaceClassID: "wci",
		CreatedByUserID:  "cui",
		DNS:              "dns",
		Status:           "s",
		Password:         "p",
		GitRepo:          "g",
	}

	res, err := httpmock.NewJsonResponder(200, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", s.authHTTPClient.restyClient.BaseURL, fmt.Sprintf(workspaceStopPathPattern, workspaceID))
	httpmock.RegisterResponder("PUT", url, res)

	u, err := s.StopWorkspace(workspaceID)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, u) {
		return
	}
	if !assert.Equal(t, expected, u) {
		return
	}
}

func TestStartWorkspace(t *testing.T) { //nolint:dupl // ok to have this be duplicate
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	workspaceID := "1"
	expected := &entity.Workspace{
		ID:               workspaceID,
		Name:             "name",
		WorkspaceGroupID: "wgi",
		OrganizationID:   "oi",
		WorkspaceClassID: "wci",
		CreatedByUserID:  "cui",
		DNS:              "dns",
		Status:           "s",
		Password:         "p",
		GitRepo:          "g",
	}

	res, err := httpmock.NewJsonResponder(200, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", s.authHTTPClient.restyClient.BaseURL, fmt.Sprintf(workspaceStartPathPattern, workspaceID))
	httpmock.RegisterResponder("PUT", url, res)

	u, err := s.StartWorkspace(workspaceID)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, u) {
		return
	}
	if !assert.Equal(t, expected, u) {
		return
	}
}

func TestResetWorkspace(t *testing.T) { //nolint:dupl // ok to have this be duplicate
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	workspaceID := "1"
	expected := &entity.Workspace{
		ID:               workspaceID,
		Name:             "name",
		WorkspaceGroupID: "wgi",
		OrganizationID:   "oi",
		WorkspaceClassID: "wci",
		CreatedByUserID:  "cui",
		DNS:              "dns",
		Status:           "s",
		Password:         "p",
		GitRepo:          "g",
	}

	res, err := httpmock.NewJsonResponder(200, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", s.authHTTPClient.restyClient.BaseURL, fmt.Sprintf(workspaceResetPathPattern, workspaceID))
	httpmock.RegisterResponder("PUT", url, res)

	u, err := s.ResetWorkspace(workspaceID)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, u) {
		return
	}
	if !assert.Equal(t, expected, u) {
		return
	}
}

func TestGetWorkspace(t *testing.T) { //nolint:dupl // ok to have this be duplicate
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	workspaceID := "1"
	expected := &entity.Workspace{
		ID:               workspaceID,
		Name:             "name",
		WorkspaceGroupID: "wgi",
		OrganizationID:   "oi",
		WorkspaceClassID: "wci",
		CreatedByUserID:  "cui",
		DNS:              "dns",
		Status:           "s",
		Password:         "p",
		GitRepo:          "g",
	}

	res, err := httpmock.NewJsonResponder(200, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", s.authHTTPClient.restyClient.BaseURL, fmt.Sprintf(workspacePathPattern, workspaceID))
	httpmock.RegisterResponder("GET", url, res)

	u, err := s.GetWorkspace(workspaceID)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, u) {
		return
	}
	if !assert.Equal(t, expected, u) {
		return
	}
}

func TestDeleteWorkspace(t *testing.T) { //nolint:dupl // ok to have this be duplicate
	s := MakeMockAuthHTTPStore()
	httpmock.ActivateNonDefault(s.authHTTPClient.restyClient.GetClient())

	workspaceID := "1"
	expected := &entity.Workspace{
		ID:               workspaceID,
		Name:             "name",
		WorkspaceGroupID: "wgi",
		OrganizationID:   "oi",
		WorkspaceClassID: "wci",
		CreatedByUserID:  "cui",
		DNS:              "dns",
		Status:           "s",
		Password:         "p",
		GitRepo:          "g",
	}

	res, err := httpmock.NewJsonResponder(200, expected)
	if !assert.Nil(t, err) {
		return
	}
	url := fmt.Sprintf("%s/%s", s.authHTTPClient.restyClient.BaseURL, fmt.Sprintf(workspacePathPattern, workspaceID))
	httpmock.RegisterResponder("DELETE", url, res)

	u, err := s.DeleteWorkspace(workspaceID)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, u) {
		return
	}
	if !assert.Equal(t, expected, u) {
		return
	}
}

// func TestValidateOllamaModel(t *testing.T) {
// 	type args struct {
// 		model string
// 		tag   string
// 	}
// 	tests := []struct {
// 		name    string
// 		args    args
// 		want    bool
// 		wantErr bool
// 	}{
// 		{"empty", args{"", ""}, false, false},
// 		{"llama3", args{"llama3", ""}, true, false},
// 		{"llama3:80b", args{"llama3", "80b"}, false, false},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			got, err := ValidateOllamaModel(tt.args.model, tt.args.tag)
// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("ValidateOllamaModel() error = %v, wantErr %v", err, tt.wantErr)
// 				return
// 			}
// 			if got != tt.want {
// 				t.Errorf("ValidateOllamaModel() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }
