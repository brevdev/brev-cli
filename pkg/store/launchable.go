package store

import (
	"fmt"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
)

const (
	createEnvironmentLaunchablePathPattern = "api/organizations/%s/v2/launchables"
	createEnvironmentLaunchablePath        = fmt.Sprintf(createEnvironmentLaunchablePathPattern, fmt.Sprintf("{%s}", orgIDParamName))
	validateDockerComposePath              = "api/file-validation/docker-compose"
)

type CreateLaunchableResponse struct {
	ID string `json:"id"`
}

type CreateFirewallRule struct {
	Port       string   `json:"port"`
	AllowedIPs string   `json:"allowedIPs"`
	ClientIPs  []string `json:"clientIPs,omitempty"`
}

type CreateEnvironmentLaunchableWorkspaceRequest struct {
	WorkspaceGroupID string               `json:"workspaceGroupId"`
	InstanceType     string               `json:"instanceType"`
	Location         string               `json:"location,omitempty"`
	Region           string               `json:"region,omitempty"`
	SubLocation      string               `json:"subLocation,omitempty"`
	Storage          string               `json:"storage,omitempty"`
	FirewallRules    []CreateFirewallRule `json:"firewallRules,omitempty"`
	ImageID          string               `json:"imageId,omitempty"`
}

type CreateEnvironmentLaunchableBuildRequest struct {
	VMBuild        *VMBuild         `json:"vmBuild,omitempty"`
	ContainerBuild *CustomContainer `json:"containerBuild,omitempty"`
	DockerCompose  *DockerCompose   `json:"dockerCompose,omitempty"`
	Ports          []LaunchablePort `json:"ports,omitempty"`
}

type CreateEnvironmentLaunchableRequest struct {
	Name                   string                                      `json:"name"`
	Description            string                                      `json:"description"`
	ViewAccess             string                                      `json:"viewAccess"`
	CreateWorkspaceRequest CreateEnvironmentLaunchableWorkspaceRequest `json:"createWorkspaceRequest"`
	BuildRequest           CreateEnvironmentLaunchableBuildRequest     `json:"buildRequest"`
	File                   *LaunchableFile                             `json:"file,omitempty"`
}

type ValidateDockerComposeRequest struct {
	FileURL    string `json:"fileUrl,omitempty"`
	YamlString string `json:"yamlString,omitempty"`
}

type ValidateDockerComposeResponse map[string]interface{}

func (s AuthHTTPStore) CreateEnvironmentLaunchable(organizationID string, req CreateEnvironmentLaunchableRequest) (*CreateLaunchableResponse, error) {
	var result CreateLaunchableResponse
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam(orgIDParamName, organizationID).
		SetBody(req).
		SetResult(&result).
		Post(createEnvironmentLaunchablePath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return &result, nil
}

func (s AuthHTTPStore) ValidateDockerCompose(req ValidateDockerComposeRequest) (*ValidateDockerComposeResponse, error) {
	result := ValidateDockerComposeResponse{}
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetBody(req).
		SetResult(&result).
		Post(validateDockerComposePath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return &result, nil
}
