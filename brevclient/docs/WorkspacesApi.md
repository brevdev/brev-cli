# \WorkspacesApi

All URIs are relative to *https://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**ApiWorkspacesIdDelete**](WorkspacesApi.md#ApiWorkspacesIdDelete) | **Delete** /api/workspaces/{id} | Deletes a workspace
[**ApiWorkspacesIdGet**](WorkspacesApi.md#ApiWorkspacesIdGet) | **Get** /api/workspaces/{id} | Get a workspace
[**ApiWorkspacesIdPut**](WorkspacesApi.md#ApiWorkspacesIdPut) | **Put** /api/workspaces/{id} | Modifies a workspace
[**ApiWorkspacesIdRoleAttachmentsGet**](WorkspacesApi.md#ApiWorkspacesIdRoleAttachmentsGet) | **Get** /api/workspaces/{id}/role_attachments | Lists a workspace&#39;s role attachments
[**ApiWorkspacesIdRoleAttachmentsPut**](WorkspacesApi.md#ApiWorkspacesIdRoleAttachmentsPut) | **Put** /api/workspaces/{id}/role_attachments | Modifies a workspace&#39;s role attachments
[**ApiWorkspacesIdRoleAttachmentsSubjectIDDelete**](WorkspacesApi.md#ApiWorkspacesIdRoleAttachmentsSubjectIDDelete) | **Delete** /api/workspaces/{id}/role_attachments/{subjectID} | Deletes a workspace role attachment
[**ApiWorkspacesIdStartPut**](WorkspacesApi.md#ApiWorkspacesIdStartPut) | **Put** /api/workspaces/{id}/start | Starts a workspace
[**ApiWorkspacesIdUsersGet**](WorkspacesApi.md#ApiWorkspacesIdUsersGet) | **Get** /api/workspaces/{id}/users | Lists a the users attached to the given workspace


# **ApiWorkspacesIdDelete**
> Workspace ApiWorkspacesIdDelete(ctx, id)
Deletes a workspace

deletes a workspace matching the given ID

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Workspace ID | 

### Return type

[**Workspace**](Workspace.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiWorkspacesIdGet**
> Workspace ApiWorkspacesIdGet(ctx, id)
Get a workspace

get a workspace by ID

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Workspace ID | 

### Return type

[**Workspace**](Workspace.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiWorkspacesIdPut**
> Workspace ApiWorkspacesIdPut(ctx, id, message)
Modifies a workspace

modifies a workspace to match the request object

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Workspace ID | 
  **message** | [**ModifyWorkspaceRequest**](ModifyWorkspaceRequest.md)| Workspace payload | 

### Return type

[**Workspace**](Workspace.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiWorkspacesIdRoleAttachmentsGet**
> []RoleAttachmentJson ApiWorkspacesIdRoleAttachmentsGet(ctx, id)
Lists a workspace's role attachments

lists all role attachments for the workspace matching the given ID

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Workspace ID | 

### Return type

[**[]RoleAttachmentJson**](RoleAttachmentJSON.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiWorkspacesIdRoleAttachmentsPut**
> []RoleAttachmentJson ApiWorkspacesIdRoleAttachmentsPut(ctx, id, message)
Modifies a workspace's role attachments

modifies the role attachment of a workspace for a given subject

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Workspace ID | 
  **message** | [**WorkspaceRoleAttachmentRequest**](WorkspaceRoleAttachmentRequest.md)| Workspace role attachment payload | 

### Return type

[**[]RoleAttachmentJson**](RoleAttachmentJSON.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiWorkspacesIdRoleAttachmentsSubjectIDDelete**
> []RoleAttachmentJson ApiWorkspacesIdRoleAttachmentsSubjectIDDelete(ctx, id)
Deletes a workspace role attachment

deletes the role attachment matching the provided subject ID

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Workspace ID | 

### Return type

[**[]RoleAttachmentJson**](RoleAttachmentJSON.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiWorkspacesIdStartPut**
> Workspace ApiWorkspacesIdStartPut(ctx, id)
Starts a workspace

Starts a workspace

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Workspace ID | 

### Return type

[**Workspace**](Workspace.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiWorkspacesIdUsersGet**
> []WorkspaceUserJson ApiWorkspacesIdUsersGet(ctx, id)
Lists a the users attached to the given workspace

lists all users for the workspace matching the given ID

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Workspace ID | 

### Return type

[**[]WorkspaceUserJson**](WorkspaceUserJSON.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

