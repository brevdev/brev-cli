package store

import (
	"fmt"
	"strings"

	"github.com/brevdev/brev-cli/pkg/config"
	"github.com/brevdev/brev-cli/pkg/entity"
	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/brevdev/brev-cli/pkg/files"
	"github.com/brevdev/brev-cli/pkg/setupscript"
)

var (
	orgIDParamName          = "organizationID"
	workspaceOrgPathPattern = "api/organizations/%s/workspaces"
	workspaceOrgPath        = fmt.Sprintf(workspaceOrgPathPattern, fmt.Sprintf("{%s}", orgIDParamName))
)

type CreateWorkspacesOptions struct {
	Name                 string               `json:"name"`
	WorkspaceGroupID     string               `json:"workspaceGroupId"`
	WorkspaceClassID     string               `json:"workspaceClassId"`
	GitRepo              string               `json:"gitRepo"`
	IsStoppable          bool                 `json:"isStoppable"`
	WorkspaceTemplateID  string               `json:"workspaceTemplateId"`
	PrimaryApplicationID string               `json:"primaryApplicationId"`
	Applications         []entity.Application `json:"applications"`
	StartupScript        string               `json:"startupScript"`
}

var (
	DefaultWorkspaceClassID    = config.GlobalConfig.GetDefaultWorkspaceClass()
	DefaultWorkspaceTemplateID = config.GlobalConfig.GetDefaultWorkspaceTemplate()
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
		Name:                 name,
		WorkspaceGroupID:     clusterID,
		WorkspaceClassID:     DefaultWorkspaceClassID,
		GitRepo:              "",
		IsStoppable:          false,
		WorkspaceTemplateID:  DefaultWorkspaceTemplateID,
		PrimaryApplicationID: DefaultApplicationID,
		Applications:         DefaultApplicationList,
		StartupScript:        setupscript.DefaultSetupScript,
	}
}

func (c *CreateWorkspacesOptions) WithGitRepo(gitRepo string) *CreateWorkspacesOptions {
	c.GitRepo = gitRepo
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

// Returns VSCode String, Folder Name
func (s AuthHTTPStore) GetVSCodeOpenString(wsIDOrName string) (string, string, error) {
	workspace, workspaces, err := getWorkspaceFromNameOrIDAndReturnWorkspacesPlusWorkspace(wsIDOrName, s)
	if err != nil {
		return "", "", breverrors.WrapAndTrace(err)
	}

	// Get the folder name
	// Get base repo path or use workspace Name
	var folderName string
	if len(workspace.GitRepo) > 0 {
		splitBySlash := strings.Split(workspace.GitRepo, "/")[1]
		repoPath := strings.Split(splitBySlash, ".git")[0]
		folderName = repoPath
	} else {
		folderName = workspace.Name
	}

	// note: intentional decision to just assume the parent folder and inner folder are the same
	vscodeString := fmt.Sprintf("vscode-remote://ssh-remote+%s/home/brev/workspace/%s", workspace.GetLocalIdentifier(*workspaces), folderName)

	return vscodeString, folderName, nil
}

func getWorkspaceFromNameOrIDAndReturnWorkspacesPlusWorkspace(nameOrID string, sstore AuthHTTPStore) (*entity.WorkspaceWithMeta, *[]entity.Workspace, error) {
	// Get Active Org
	org, err := sstore.GetActiveOrganizationOrDefault()
	if err != nil {
		return nil, nil, breverrors.WrapAndTrace(err)
	}
	if org == nil {
		return nil, nil, fmt.Errorf("no orgs exist")
	}

	// Get Current User
	currentUser, err := sstore.GetCurrentUser()
	if err != nil {
		return nil, nil, breverrors.WrapAndTrace(err)
	}

	// Get Workspaces for User
	var workspace *entity.Workspace // this will be the returned workspace
	workspaces, err := sstore.GetWorkspaces(org.ID, &GetWorkspacesOptions{Name: nameOrID, UserID: currentUser.ID})
	if err != nil {
		return nil, nil, breverrors.WrapAndTrace(err)
	}

	switch len(workspaces) {
	case 0:
		// In this case, check workspace by ID
		wsbyid, othererr := sstore.GetWorkspace(nameOrID) // Note: workspaceName is ID in this case
		if othererr != nil {
			return nil, nil, fmt.Errorf("no workspaces found with name or id %s", nameOrID)
		}
		if wsbyid != nil {
			workspace = wsbyid
		} else {
			// Can this case happen?
			return nil, nil, fmt.Errorf("no workspaces found with name or id %s", nameOrID)
		}
	case 1:
		workspace = &workspaces[0]
	default:
		return nil, nil, fmt.Errorf("multiple workspaces found with name %s\n\nTry running the command by id instead of name:\n\tbrev command <id>", nameOrID)
	}

	if workspace == nil {
		return nil, nil, fmt.Errorf("no workspaces found with name or id %s", nameOrID)
	}

	// Get WorkspaceMetaData
	workspaceMetaData, err := sstore.GetWorkspaceMetaData(workspace.ID)
	if err != nil {
		return nil, nil, breverrors.WrapAndTrace(err)
	}

	return &entity.WorkspaceWithMeta{WorkspaceMetaData: *workspaceMetaData, Workspace: *workspace}, &workspaces, nil
}

// This function updates ssh/code local tool commands
func (s AuthHTTPStore) UpdateLocalBashScripts(wss []entity.Workspace, userID string) error {
	functionPath := files.GetBrevFunctionFilePath()
	variablesPath := files.GetBrevVariablesFilePath()

	functionContents := `
code() {
	if [[ " ${wss[*]} " =~ "$1" ]]; then
		code --folder-uri $wssTable[$1]
	fi
	if [[ ! " ${wss[*]} " =~ "$1" ]]; then
		command code "$@"
	fi
}
`
	// Build the workspace list in bash
// declare -A wssTable=( ["brevdev/brev-cli"]="brevdevbrevcli-7tng" ["brev-workspaces-frontend"]="brevworkspacesfrontend-2w2q" )

	workspaceTable := `declare -A wssTable=( `
	sshCompletionFunction := `complete -W "`
	codeCompletionFunction := `complete -W "`
	variablesContent := "wss=("
	var workspacesRunningAndForUser []entity.Workspace
	for _, w := range wss {
		if w.Status=="RUNNING"  && w.CreatedByUserID==userID {
			workspacesRunningAndForUser = append(workspacesRunningAndForUser, w)
		}
	}
	for i, w := range workspacesRunningAndForUser {
		localIdentifier := string(w.GetLocalIdentifier(wss))
		if i < len(workspacesRunningAndForUser)-1 {
			// added a comma here--------------->
			variablesContent += fmt.Sprintf(`"%s",`, localIdentifier)
		} else {
			// no comma
			variablesContent += fmt.Sprintf(`"%s"`, localIdentifier)
		}
		sshCompletionFunction += localIdentifier + " "
		codeCompletionFunction += localIdentifier + " "
		vscodeOpenString, _, err := s.GetVSCodeOpenString(w.ID)
		if err == nil {
			// ["brevdev/brev-cli"]="brevdevbrevcli-7tng"
			workspaceTable += fmt.Sprintf(` ["%s"]="%s" `, localIdentifier, vscodeOpenString)
		}
	}
	sshCompletionFunction += `" ssh`
	codeCompletionFunction += `" code`
	workspaceTable += ` )`
	variablesContent += ")\n\n" + sshCompletionFunction + "\n" + codeCompletionFunction + "\n" + workspaceTable + "\n"
	

	err := files.OverwriteString(functionPath, functionContents)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	err = files.OverwriteString(variablesPath, variablesContent)
	if err != nil {
		return breverrors.WrapAndTrace(err)
	}

	files.AddBrevFunctionAndVarsToBashProfile()

	return nil
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
		myWorkspaces := []entity.Workspace{}
		for _, w := range workspaces {
			if w.CreatedByUserID == options.UserID {
				myWorkspaces = append(myWorkspaces, w)
			}
		}
		workspaces = myWorkspaces
	}

	if options.Name != "" {
		myWorkspaces := []entity.Workspace{}
		for _, w := range workspaces {
			if w.Name == options.Name {
				myWorkspaces = append(myWorkspaces, w)
			}
		}
		workspaces = myWorkspaces
	}

	return workspaces, nil
}

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
