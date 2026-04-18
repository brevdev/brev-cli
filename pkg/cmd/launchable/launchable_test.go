package launchable

import (
	"testing"

	"github.com/brevdev/brev-cli/pkg/store"
	"github.com/stretchr/testify/assert"
)

// validSpec returns a request that passes validateRequest, so tests can mutate
// one field at a time to isolate what fails.
func validSpec() *store.CreateLaunchableRequest {
	return &store.CreateLaunchableRequest{
		Name: "test",
		CreateWorkspaceRequest: store.CreateLaunchableWorkspaceRequest{
			InstanceType:     "g2-standard-4:nvidia-l4:1",
			WorkspaceGroupID: "GCP",
		},
		BuildRequest: store.LaunchableBuildRequest{
			DockerCompose: &store.DockerCompose{FileURL: "https://example.com/x.yml"},
		},
	}
}

func TestValidateRequest(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*store.CreateLaunchableRequest)
		wantErr string // substring of expected error; "" means no error
	}{
		{
			name:    "valid",
			mutate:  func(_ *store.CreateLaunchableRequest) {},
			wantErr: "",
		},
		{
			name:    "missing name",
			mutate:  func(r *store.CreateLaunchableRequest) { r.Name = "" },
			wantErr: "name is required",
		},
		{
			name: "missing instanceType",
			mutate: func(r *store.CreateLaunchableRequest) {
				r.CreateWorkspaceRequest.InstanceType = ""
			},
			wantErr: "instanceType is required",
		},
		{
			name: "missing workspaceGroupId",
			mutate: func(r *store.CreateLaunchableRequest) {
				r.CreateWorkspaceRequest.WorkspaceGroupID = ""
			},
			wantErr: "workspaceGroupId is required",
		},
		{
			name: "no build kind set",
			mutate: func(r *store.CreateLaunchableRequest) {
				r.BuildRequest = store.LaunchableBuildRequest{}
			},
			wantErr: "dockerCompose, containerBuild, or vmBuild",
		},
		{
			name: "custom container build accepted",
			mutate: func(r *store.CreateLaunchableRequest) {
				r.BuildRequest = store.LaunchableBuildRequest{
					CustomContainer: &store.CustomContainer{},
				}
			},
			wantErr: "",
		},
		{
			name: "vm build accepted",
			mutate: func(r *store.CreateLaunchableRequest) {
				r.BuildRequest = store.LaunchableBuildRequest{
					VMBuild: &store.VMBuild{},
				}
			},
			wantErr: "",
		},
		{
			name:    "viewAccess public ok",
			mutate:  func(r *store.CreateLaunchableRequest) { r.ViewAccess = "public" },
			wantErr: "",
		},
		{
			name:    "viewAccess private ok",
			mutate:  func(r *store.CreateLaunchableRequest) { r.ViewAccess = "private" },
			wantErr: "",
		},
		{
			name:    "viewAccess mixed case normalized",
			mutate:  func(r *store.CreateLaunchableRequest) { r.ViewAccess = "Public" },
			wantErr: "",
		},
		{
			name:    "viewAccess invalid",
			mutate:  func(r *store.CreateLaunchableRequest) { r.ViewAccess = "shared" },
			wantErr: `viewAccess must be "public" or "private"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validSpec()
			tc.mutate(req)
			err := validateRequest(req)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

func TestValidateRequestNormalizesViewAccess(t *testing.T) {
	req := validSpec()
	req.ViewAccess = "PUBLIC"
	assert.NoError(t, validateRequest(req))
	assert.Equal(t, "public", req.ViewAccess, "validateRequest should lowercase viewAccess in place")
}

func TestApplyOverrides(t *testing.T) {
	tests := []struct {
		name            string
		specName        string
		specDescription string
		specViewAccess  string
		positional      string
		nameFlag        string
		description     string
		viewAccess      string
		wantName        string
		wantDescription string
		wantViewAccess  string
	}{
		{
			name:     "positional overrides spec and --name",
			specName: "spec-name",
			positional: "positional-name",
			nameFlag: "flag-name",
			wantName: "positional-name",
		},
		{
			name:     "--name overrides spec when no positional",
			specName: "spec-name",
			nameFlag: "flag-name",
			wantName: "flag-name",
		},
		{
			name:     "spec name kept when no positional and no --name",
			specName: "spec-name",
			wantName: "spec-name",
		},
		{
			name:            "description override replaces spec",
			specDescription: "spec-desc",
			description:     "flag-desc",
			wantDescription: "flag-desc",
		},
		{
			name:            "empty --description keeps spec",
			specDescription: "spec-desc",
			wantDescription: "spec-desc",
		},
		{
			name:           "view-access override replaces spec",
			specViewAccess: "private",
			viewAccess:     "public",
			wantViewAccess: "public",
		},
		{
			name:           "empty --view-access keeps spec",
			specViewAccess: "private",
			wantViewAccess: "private",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &store.CreateLaunchableRequest{
				Name:        tc.specName,
				Description: tc.specDescription,
				ViewAccess:  tc.specViewAccess,
			}
			applyOverrides(req, tc.positional, tc.nameFlag, tc.description, tc.viewAccess)
			if tc.wantName != "" {
				assert.Equal(t, tc.wantName, req.Name)
			}
			if tc.wantDescription != "" {
				assert.Equal(t, tc.wantDescription, req.Description)
			}
			if tc.wantViewAccess != "" {
				assert.Equal(t, tc.wantViewAccess, req.ViewAccess)
			}
		})
	}
}
