package store

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/uri"
	resty "github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/spf13/afero"
)

var (
	orgIDParamName          = "organizationID"
	workspaceOrgPathPattern = "api/organizations/%s/workspaces"
	workspaceOrgPath        = fmt.Sprintf(workspaceOrgPathPattern, fmt.Sprintf("{%s}", orgIDParamName))
)

type ModifyWorkspaceRequest struct {
	WorkspaceClassID  string            `json:"workspaceClassId,omitempty"`
	IsStoppable       *bool             `json:"isStoppable,omitempty"`
	StartupScriptPath string            `json:"startupScriptPath,omitempty"`
	Name              string            `json:"name,omitempty"`
	IDEConfig         *entity.IDEConfig `json:"ideConfig,omitempty"`
	Repos             entity.ReposV0    `json:"repos,omitempty"`
	Execs             entity.ExecsV0    `json:"execs,omitempty"`
	ReposV1           *entity.ReposV1   `json:"reposV1,omitempty"`
	ExecsV1           *entity.ExecsV1   `json:"execsV1,omitempty"`
	InstanceType      string            `json:"instanceType,omitempty"`
}

type CreateWorkspacesOptions struct {
	Name                 string               `json:"name"`
	WorkspaceGroupID     string               `json:"workspaceGroupId"`
	WorkspaceClassID     string               `json:"workspaceClassId"`
	WorkspaceTemplateID  string               `json:"workspaceTemplateId"`
	Description          string               `json:"description"`
	IsStoppable          *bool                `json:"isStoppable,omitempty"`
	PrimaryApplicationID string               `json:"primaryApplicationId"`
	Applications         []entity.Application `json:"applications"`
	StartupScript        string               `json:"startupScript"`
	GitRepo              string               `json:"gitRepo"`
	InitBranch           string               `json:"initBranch"`
	StartupScriptPath    string               `json:"startupScriptPath"`
	DotBrevPath          string               `json:"dotBrevPath"`
	IDEConfig            *entity.IDEConfig    `json:"ideConfig"`
	Repos                entity.ReposV0       `json:"repos"`
	Execs                entity.ExecsV0       `json:"execs"`
	ReposV1              *entity.ReposV1      `json:"reposV1"`
	ExecsV1              *entity.ExecsV1      `json:"execsV1"`
	InstanceType         string               `json:"instanceType"`
	DiskStorage          string               `json:"diskStorage"`
	BaseImage            string               `json:"baseImage"`
	VMOnlyMode           bool                 `json:"vmOnlyMode"`
	PortMappings         map[string]string    `json:"portMappings"`
	Files                interface{}          `json:"files"`
	Labels               interface{}          `json:"labels"`
	WorkspaceVersion     string               `json:"workspaceVersion"`
	LaunchJupyterOnStart bool                 `json:"launchJupyterOnStart"`
}

var (
	DefaultWorkspaceClassID = config.GlobalConfig.GetDefaultWorkspaceClass()
	UserWorkspaceClassID    = "2x8"
	DevWorkspaceClassID     = "4x16"

	DefaultWorkspaceTemplateID = config.GlobalConfig.GetDefaultWorkspaceTemplate()
	UserWorkspaceTemplateID    = "4nbb4lg2s"
	DevWorkspaceTemplateID     = "v7nd45zsc"
	DefaultDiskStorage         = "120Gi"
)

var (
	DefaultApplicationID = "92f59a4yf"
	DefaultApplication   = entity.Application{
		ID:           DefaultApplicationID,
		Name:         "VSCode",
		Port:         22778,
		StartCommand: "",
		Version:      "1.57.1",
	}
)
var DefaultApplicationList = []entity.Application{DefaultApplication}

func NewCreateWorkspacesOptions(clusterID, name string) *CreateWorkspacesOptions {
	return &CreateWorkspacesOptions{
		BaseImage:            "",
		Description:          "",
		DiskStorage:          DefaultDiskStorage,
		ExecsV1:              &entity.ExecsV1{},
		Files:                nil,
		InstanceType:         "",
		IsStoppable:          nil,
		Labels:               nil,
		LaunchJupyterOnStart: false,
		Name:                 name,
		PortMappings:         nil,
		ReposV1:              nil,
		VMOnlyMode:           true,
		WorkspaceGroupID:     "GCP",
		WorkspaceTemplateID:  DefaultWorkspaceTemplateID,
		WorkspaceVersion:     "v1",
	}
}

func (c *CreateWorkspacesOptions) WithCustomSetupRepo(gitRepo string, path string) *CreateWorkspacesOptions {
	c.Repos = entity.ReposV0{
		"configRepo": entity.RepoV0{
			Repository:    gitRepo,
			SetupExecPath: path,
		},
	}
	return c
}

func (c *CreateWorkspacesOptions) WithGitRepo(gitRepo string) *CreateWorkspacesOptions {
	c.GitRepo = gitRepo
	return c
}

func (c *CreateWorkspacesOptions) WithInstanceType(instanceType string) *CreateWorkspacesOptions {
	c.InstanceType = instanceType
	return c
}

func (c *CreateWorkspacesOptions) WithClassID(classID string) *CreateWorkspacesOptions {
	c.WorkspaceClassID = classID
	return c
}

func (c *CreateWorkspacesOptions) WithStartupScript(startupScript string) *CreateWorkspacesOptions {
	c.StartupScript = startupScript
	return c
}

func (c *CreateWorkspacesOptions) WithWorkspaceClassID(workspaceClassID string) *CreateWorkspacesOptions {
	c.WorkspaceClassID = workspaceClassID
	return c
}

func (s AuthHTTPStore) CreateWorkspace(organizationID string, options *CreateWorkspacesOptions) (*entity.Workspace, error) {
	if options == nil {
		return nil, fmt.Errorf("options can not be nil")
	}

	var result entity.Workspace
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam(orgIDParamName, organizationID).
		SetBody(options).
		SetResult(&result).
		Post(workspaceOrgPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}

	return &result, nil
}

type GetWorkspacesOptions struct {
	UserID string
	Name   string
}

func (s AuthHTTPStore) GetWorkspaces(organizationID string, options *GetWorkspacesOptions) ([]entity.Workspace, error) {
	workspaces, err := s.getWorkspaces(organizationID)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	if options == nil {
		return workspaces, nil
	}

	if options.UserID != "" {
		userWorkspaces := []entity.Workspace{}
		for _, w := range workspaces {
			if w.CanShow(options.UserID) {
				userWorkspaces = append(userWorkspaces, w)
			}
		}
		workspaces = userWorkspaces
	}

	if options.Name != "" {
		nameWorkspaces := []entity.Workspace{}
		for _, w := range workspaces {
			if w.Name == options.Name {
				nameWorkspaces = append(nameWorkspaces, w)
			} else if w.ID == options.Name {
				nameWorkspaces = append(nameWorkspaces, w)
			}
		}
		workspaces = nameWorkspaces
	}

	return workspaces, nil
}

func FilterForUserWorkspaces(workspaces []entity.Workspace, userID string) []entity.Workspace {
	filteredWorkspaces := []entity.Workspace{}
	for _, w := range workspaces {
		if w.CanShow(userID) {
			filteredWorkspaces = append(filteredWorkspaces, w)
		}
	}
	return filteredWorkspaces
}

func FilterNonFailedWorkspaces(workspaces []entity.Workspace) []entity.Workspace {
	filteredWorkspaces := []entity.Workspace{}
	for _, w := range workspaces {
		if w.Status != entity.Failure {
			filteredWorkspaces = append(filteredWorkspaces, w)
		}
	}
	return filteredWorkspaces
}

func (s AuthHTTPStore) GetWorkspaceByNameOrID(orgID string, nameOrID string) ([]entity.Workspace, error) {
	// pretty srue we always want to filter for workspaces owned by the user
	user, err := s.GetCurrentUser()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	workspaces, err := s.GetWorkspaces(orgID, &GetWorkspacesOptions{
		Name:   nameOrID,
		UserID: user.ID,
	})
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	return workspaces, nil
}

// get user workspaces in org, like brev ls
func (s AuthHTTPStore) GetContextWorkspaces() ([]entity.Workspace, error) {
	org, err := s.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	user, err := s.GetCurrentUser()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	workspaces, err := s.GetWorkspaces(org.ID, &GetWorkspacesOptions{UserID: user.ID})
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return workspaces, nil
}

func (s AuthHTTPStore) GetAllWorkspaces(options *GetWorkspacesOptions) ([]entity.Workspace, error) {
	orgs, err := s.GetOrganizations(nil)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	allWorkspaces := []entity.Workspace{}
	for _, o := range orgs {
		workspaces, err := s.GetWorkspaces(o.ID, options)
		if err != nil {
			return nil, breverrors.WrapAndTrace(err)
		}
		allWorkspaces = append(allWorkspaces, workspaces...)
	}

	return allWorkspaces, nil
}

func (s AuthHTTPStore) getWorkspaces(organizationID string) ([]entity.Workspace, error) {
	var result []entity.Workspace
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam(orgIDParamName, organizationID).
		SetResult(&result).
		Get(workspaceOrgPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}
	return result, nil
}

var (
	workspaceIDParamName = "workspaceID"
	workspacePathPattern = "api/workspaces/%s"
	workspacePath        = fmt.Sprintf(workspacePathPattern, fmt.Sprintf("{%s}", workspaceIDParamName))
)

func (s AuthHTTPStore) GetWorkspace(workspaceID string) (*entity.Workspace, error) {
	var result entity.Workspace
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam(workspaceIDParamName, workspaceID).
		SetResult(&result).
		Get(workspacePath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}
	return &result, nil
}

func (s AuthHTTPStore) ModifyWorkspace(workspaceID string, options *ModifyWorkspaceRequest) (*entity.Workspace, error) {
	if options == nil {
		return nil, fmt.Errorf("options can not be nil")
	}

	var result entity.Workspace
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam(workspaceIDParamName, workspaceID).
		SetResult(&result).
		SetBody(options).
		Put(workspacePath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}
	// fmt.Printf("name %s\n", result.Name)
	// fmt.Printf("template %s %s\n", result.WorkspaceTemplate.ID, result.WorkspaceTemplate.Name)
	// fmt.Printf("resource class %s\n", result.WorkspaceClassID)
	// fmt.Printf("instance %s\n", result.InstanceType)
	// fmt.Printf("workspace group %s\n", result.WorkspaceGroupID)
	return &result, nil
}

func (s AuthHTTPStore) DeleteWorkspace(workspaceID string) (*entity.Workspace, error) {
	var result entity.Workspace
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam(workspaceIDParamName, workspaceID).
		SetResult(&result).
		Delete(workspacePath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}
	return &result, nil
}

func (s AuthHTTPStore) BanUser(userID string) error {
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		Put(fmt.Sprintf("/api/users/%s/block", userID))
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return NewHTTPResponseError(res)
	}
	return nil
}

func (s AuthHTTPStore) GetAllOrgsAsAdmin(userID string) ([]entity.Organization, error) {
	var result []entity.Organization
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetQueryParam("user_id", userID).
		SetResult(&result).
		Get("/api/organizations")
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}
	return result, nil
}

var (
	workspaceMetadataPathPattern = fmt.Sprintf("%s/metadata", workspacePathPattern)
	workspaceMetadataPath        = fmt.Sprintf(workspaceMetadataPathPattern, fmt.Sprintf("{%s}", workspaceIDParamName))
)

func (s AuthHTTPStore) GetWorkspaceMetaData(workspaceID string) (*entity.WorkspaceMetaData, error) {
	var result entity.WorkspaceMetaData
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam(workspaceIDParamName, workspaceID).
		SetResult(&result).
		Get(workspaceMetadataPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}
	return &result, nil
}

var (
	workspaceStopPathPattern = fmt.Sprintf("%s/stop", workspacePathPattern)
	workspaceStopPath        = fmt.Sprintf(workspaceStopPathPattern, fmt.Sprintf("{%s}", workspaceIDParamName))
)

func (s AuthHTTPStore) StopWorkspace(workspaceID string) (*entity.Workspace, error) {
	var result entity.Workspace
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam(workspaceIDParamName, workspaceID).
		SetResult(&result).
		Put(workspaceStopPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}
	return &result, nil
}

func (s AuthHTTPStore) AutoStopWorkspace(workspaceID string) (*entity.Workspace, error) {
	var result entity.Workspace
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetQueryParam("autoStop", "true").
		SetPathParam(workspaceIDParamName, workspaceID).
		SetResult(&result).
		Put(workspaceStopPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}
	return &result, nil
}

var (
	workspaceStartPathPattern = fmt.Sprintf("%s/start", workspacePathPattern)
	workspaceStartPath        = fmt.Sprintf(workspaceStartPathPattern, fmt.Sprintf("{%s}", workspaceIDParamName))
)

func (s AuthHTTPStore) StartWorkspace(workspaceID string) (*entity.Workspace, error) {
	var result entity.Workspace
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam(workspaceIDParamName, workspaceID).
		SetResult(&result).
		Put(workspaceStartPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}
	return &result, nil
}

var (
	workspaceResetPathPattern = fmt.Sprintf("%s/reset", workspacePathPattern)
	workspaceResetPath        = fmt.Sprintf(workspaceResetPathPattern, fmt.Sprintf("{%s}", workspaceIDParamName))
)

func (s AuthHTTPStore) ResetWorkspace(workspaceID string) (*entity.Workspace, error) {
	var result entity.Workspace
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam(workspaceIDParamName, workspaceID).
		SetResult(&result).
		Put(workspaceResetPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}
	return &result, nil
}

var (
	workspaceSetupPathPattern = fmt.Sprintf("%s/setup", workspacePathPattern)
	workspaceSetupPath        = fmt.Sprintf(workspaceSetupPathPattern, fmt.Sprintf("{%s}", workspaceIDParamName))
)

func (s AuthHTTPStore) GetEnvSetupParams(workspaceID string) (*SetupParamsV0, error) {
	var result SetupParamsV0
	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam(workspaceIDParamName, workspaceID).
		SetResult(&result).
		Get(workspaceSetupPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}
	return &result, nil
}

type KeyPair struct {
	PublicKeyData  string `json:"publicKeyData"`
	PrivateKeyData string `json:"privateKeyData"`
}

type SetupParamsV0 struct {
	WorkspaceHost                    uri.Host `json:"workspaceHost"`
	WorkspacePort                    int      `json:"workspacePort"`
	WorkspaceBaseRepo                string   `json:"workspaceBaseRepo"`
	WorkspaceProjectRepo             string   `json:"workspaceProjectRepo"`
	WorkspaceProjectRepoBranch       string   `json:"workspaceProjectRepoBranch"`
	WorkspaceApplicationStartScripts []string `json:"workspaceApplicationStartScripts"`
	WorkspaceUsername                string   `json:"workspaceUsername"`
	WorkspaceEmail                   string   `json:"workspaceEmail"`
	WorkspacePassword                string   `json:"workspacePassword"`
	WorkspaceKeyPair                 *KeyPair `json:"workspaceKeyPair"`

	ProjectSetupScript *string `json:"setupScript"`

	ProjectFolderName    string `json:"projectFolderName"`
	ProjectBrevPath      string `json:"brevPath"`
	ProjectSetupExecPath string `json:"projectSetupExecPath"`

	UserBrevPath      string `json:"userBrevPath"`
	UserSetupExecPath string `json:"userSetupExecPath"`

	ReposV0 entity.ReposV0 `json:"repos"` // the new way to handle repos, user and project should be here
	ExecsV0 entity.ExecsV0 `json:"execs"` // the new way to handle setup scripts

	ReposV1 entity.ReposV1 `json:"reposV1"`
	ExecsV1 entity.ExecsV1 `json:"execsV1"`

	IDEConfigs IDEConfigs `json:"ideConfig"`

	VerbYaml string `json:"verbYaml"`

	DisableSetup bool `json:"disableSetup"`
}

type (
	IDEName   string
	IdeConfig struct {
		ExtensionIDs []string `json:"extensionIds"`
	}
	IDEConfigs map[IDEName]IdeConfig
)

type BuildVerbReqBody struct {
	VerbYaml       string `json:"verbYaml"`
	IdempotencyKey string `json:"idempotencyKey"`
}

type BuildVerbRes struct {
	LogFilePath string `json:"logFilePath"`
}

var (
	buildVerbPathPattern = "api/workspaces/%s/buildverb"
	buildVerbSetupPath   = fmt.Sprintf(buildVerbPathPattern, fmt.Sprintf("{%s}", workspaceIDParamName))
)

// Builds verb container after the workspace is created
func (s AuthHTTPStore) BuildVerbContainer(workspaceID string, verbYaml string) (*BuildVerbRes, error) {
	var result BuildVerbRes

	idempotencyKey := uuid.New().String()

	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam(workspaceIDParamName, workspaceID).
		SetBody(BuildVerbReqBody{
			VerbYaml:       verbYaml,
			IdempotencyKey: idempotencyKey,
		}).
		SetResult(&result).
		Post(buildVerbSetupPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}
	return &result, nil
}

func (f FileStore) GetSetupParams() (*SetupParamsV0, error) {
	file, err := f.fs.Open("/etc/meta/setup_v0.json")
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	fileB, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	var sp SetupParamsV0
	err = json.Unmarshal(fileB, &sp)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}

	err = file.Close()
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	return &sp, nil
}

const setupScriptPath = "/etc/setup_script.sh"

func (f FileStore) WriteSetupScript(script string) error {
	err := afero.WriteFile(f.fs, setupScriptPath, []byte(script), 744) // owner can exec everyone can read
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}
	return nil
}

func (f FileStore) GetSetupScriptPath() string {
	return setupScriptPath
}

var (
	modifyCloudflareAccessPattern = "api/applications/modifypublicity/%s"
	modifyCloudflareAccessPath    = fmt.Sprintf(modifyCloudflareAccessPattern, fmt.Sprintf("{%s}", workspaceIDParamName))
)

type ModifyApplicationPublicityRequest struct {
	ApplicationName string `json:"applicationName"`
	Public          bool   `json:"public"`
}

// Modify publicity of tunnel after creation
func (s AuthHTTPStore) ModifyPublicity(workspace *entity.Workspace, applicationName string, publicity bool) (*entity.Tunnel, error) {
	var result entity.Tunnel

	res, err := s.authHTTPClient.restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetPathParam(workspaceIDParamName, workspace.ID).
		SetBody(ModifyApplicationPublicityRequest{
			ApplicationName: applicationName,
			Public:          publicity,
		}).
		SetResult(&result).
		Post(modifyCloudflareAccessPath)
	if err != nil {
		return nil, breverrors.WrapAndTrace(err)
	}
	if res.IsError() {
		return nil, NewHTTPResponseError(res)
	}
	return &result, nil
}

type OllamaRegistrySuccessResponse struct {
	SchemaVersion int           `json:"schemaVersion"`
	MediaType     string        `json:"mediaType"`
	Config        OllamaConfig  `json:"config"`
	Layers        []OllamaLayer `json:"layers"`
}

type OllamaConfig struct {
	MediaType string `json:"mediaType"`
	Size      int    `json:"size"`
	Digest    string `json:"digest"`
}

type OllamaLayer struct {
	MediaType string `json:"mediaType"`
	Size      int    `json:"size"`
	Digest    string `json:"digest"`
}

type OllamaRegistryFailureResponse struct {
	Errors []OllamaRegistryError `json:"errors"`
}

type OllamaRegistryError struct {
	Code    string                    `json:"code"`
	Message string                    `json:"message"`
	Detail  OllamaRegistryErrorDetail `json:"detail"`
}

type OllamaRegistryErrorDetail struct {
	Tag string `json:"Tag"`
}

type OllamaModelRequest struct {
	Model string
	Tag   string
}

var (
	modelNameParamName     = "modelName"
	tagNameParamName       = "tagName"
	ollamaModelPathPattern = "v2/library/%s/manifests/%s"
	ollamaModelPath        = fmt.Sprintf(ollamaModelPathPattern, fmt.Sprintf("{%s}", modelNameParamName), fmt.Sprintf("{%s}", tagNameParamName))
)

func ValidateOllamaModel(model string, tag string) (bool, error) {
	restyClient := resty.New().SetBaseURL(config.NewConstants().GetOllamaAPIURL())
	if tag == "" {
		tag = "latest"
	}
	res, err := restyClient.R().
		SetHeader("Accept", "application/vnd.docker.distribution.manifest.v2+json").
		SetPathParam(modelNameParamName, model).
		SetPathParam(tagNameParamName, tag).
		Get(ollamaModelPath)
	if err != nil {
		return false, breverrors.WrapAndTrace(err)
	}
	if res.StatusCode() == 200 { //nolint:gocritic // 200 is a valid status code
		if err := json.Unmarshal(res.Body(), &OllamaRegistrySuccessResponse{}); err != nil {
			return false, breverrors.WrapAndTrace(err)
		}
		return true, nil
	} else if res.StatusCode() == 404 {
		if err := json.Unmarshal(res.Body(), &OllamaRegistryFailureResponse{}); err != nil {
			return false, breverrors.WrapAndTrace(err)
		}
		return false, nil
	} else {
		return false, breverrors.New("invalid response from ollama registry")
	}
}
