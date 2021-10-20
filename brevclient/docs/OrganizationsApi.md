# \OrganizationsApi

All URIs are relative to *https://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**ApiOrganizationsGet**](OrganizationsApi.md#ApiOrganizationsGet) | **Get** /api/organizations | Lists organizations
[**ApiOrganizationsIdDelete**](OrganizationsApi.md#ApiOrganizationsIdDelete) | **Delete** /api/organizations/{id} | Deletes an organization
[**ApiOrganizationsIdGet**](OrganizationsApi.md#ApiOrganizationsIdGet) | **Get** /api/organizations/{id} | Get an organization
[**ApiOrganizationsIdPut**](OrganizationsApi.md#ApiOrganizationsIdPut) | **Put** /api/organizations/{id} | Modifies an organization
[**ApiOrganizationsIdRoleAttachmentsGet**](OrganizationsApi.md#ApiOrganizationsIdRoleAttachmentsGet) | **Get** /api/organizations/{id}/role_attachments | Lists an organization&#39;s role attachments
[**ApiOrganizationsIdRoleAttachmentsPut**](OrganizationsApi.md#ApiOrganizationsIdRoleAttachmentsPut) | **Put** /api/organizations/{id}/role_attachments | Modifies an organization&#39;s role attachments
[**ApiOrganizationsIdRoleAttachmentsSubjectIDDelete**](OrganizationsApi.md#ApiOrganizationsIdRoleAttachmentsSubjectIDDelete) | **Delete** /api/organizations/{id}/role_attachments/{subjectID} | Deletes an organization role attachment
[**ApiOrganizationsIdUsersGet**](OrganizationsApi.md#ApiOrganizationsIdUsersGet) | **Get** /api/organizations/{id}/users | Lists a the users attached to the given organization
[**ApiOrganizationsIdWorkspacesGet**](OrganizationsApi.md#ApiOrganizationsIdWorkspacesGet) | **Get** /api/organizations/{id}/workspaces | Lists an organization&#39;s workspaces
[**ApiOrganizationsIdWorkspacesPost**](OrganizationsApi.md#ApiOrganizationsIdWorkspacesPost) | **Post** /api/organizations/{id}/workspaces | Creates a workspace
[**ApiOrganizationsPost**](OrganizationsApi.md#ApiOrganizationsPost) | **Post** /api/organizations | Creates an organization


# **ApiOrganizationsGet**
> []Organization ApiOrganizationsGet(ctx, )
Lists organizations

Lists organizations the context user has permission to view.

### Required Parameters
This endpoint does not need any parameter.

### Return type

[**[]Organization**](Organization.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiOrganizationsIdDelete**
> Organization ApiOrganizationsIdDelete(ctx, id)
Deletes an organization

deletes the organization matching the given ID

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Organization ID | 

### Return type

[**Organization**](Organization.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiOrganizationsIdGet**
> Organization ApiOrganizationsIdGet(ctx, id)
Get an organization

get organization by ID

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Organization ID | 

### Return type

[**Organization**](Organization.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiOrganizationsIdPut**
> Organization ApiOrganizationsIdPut(ctx, id, message)
Modifies an organization

modifies an organization to match the request object

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Organization ID | 
  **message** | [**OrganizationRequest**](OrganizationRequest.md)| Organization payload | 

### Return type

[**Organization**](Organization.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiOrganizationsIdRoleAttachmentsGet**
> []RoleAttachmentJson ApiOrganizationsIdRoleAttachmentsGet(ctx, id)
Lists an organization's role attachments

lists all role attachments for the organization matching the given ID

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Organization ID | 

### Return type

[**[]RoleAttachmentJson**](RoleAttachmentJSON.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiOrganizationsIdRoleAttachmentsPut**
> []RoleAttachmentJson ApiOrganizationsIdRoleAttachmentsPut(ctx, id, message)
Modifies an organization's role attachments

replaces the role attachments of an organization for a given subject

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Organization ID | 
  **message** | [**OrganizationRoleAttachmentRequest**](OrganizationRoleAttachmentRequest.md)| Organization role attachment payload | 

### Return type

[**[]RoleAttachmentJson**](RoleAttachmentJSON.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiOrganizationsIdRoleAttachmentsSubjectIDDelete**
> []RoleAttachmentJson ApiOrganizationsIdRoleAttachmentsSubjectIDDelete(ctx, id, subjectID)
Deletes an organization role attachment

deletes the organization role attachment matching the provided subject ID

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Organization ID | 
  **subjectID** | **string**| Subject ID | 

### Return type

[**[]RoleAttachmentJson**](RoleAttachmentJSON.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiOrganizationsIdUsersGet**
> []WorkspaceUserJson ApiOrganizationsIdUsersGet(ctx, id)
Lists a the users attached to the given organization

lists all users for the organization matching the given ID

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Organization ID | 

### Return type

[**[]WorkspaceUserJson**](WorkspaceUserJSON.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiOrganizationsIdWorkspacesGet**
> []Workspace ApiOrganizationsIdWorkspacesGet(ctx, id)
Lists an organization's workspaces

lists all workspaces for the organization matching the given ID

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Organization ID | 

### Return type

[**[]Workspace**](Workspace.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiOrganizationsIdWorkspacesPost**
> Workspace ApiOrganizationsIdWorkspacesPost(ctx, id, message)
Creates a workspace

Creates a new workspace in the given organization

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Organization ID | 
  **message** | [**CreateWorkspaceRequest**](CreateWorkspaceRequest.md)| Workspace payload | 

### Return type

[**Workspace**](Workspace.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiOrganizationsPost**
> Organization ApiOrganizationsPost(ctx, message)
Creates an organization

Creates a new organization. The user initiating the request will become the administrator.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **message** | [**OrganizationRequest**](OrganizationRequest.md)| Organization payload | 

### Return type

[**Organization**](Organization.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

