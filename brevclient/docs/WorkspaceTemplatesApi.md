# \WorkspaceTemplatesApi

All URIs are relative to *https://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**ApiWorkspaceTemplatesGet**](WorkspaceTemplatesApi.md#ApiWorkspaceTemplatesGet) | **Get** /api/workspace_templates | Lists workspace templates
[**ApiWorkspaceTemplatesIdGet**](WorkspaceTemplatesApi.md#ApiWorkspaceTemplatesIdGet) | **Get** /api/workspace_templates/{id} | Get a workspace template


# **ApiWorkspaceTemplatesGet**
> []WorkspaceTemplate ApiWorkspaceTemplatesGet(ctx, )
Lists workspace templates

lists workspace templates the context user has permission to view.

### Required Parameters
This endpoint does not need any parameter.

### Return type

[**[]WorkspaceTemplate**](WorkspaceTemplate.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiWorkspaceTemplatesIdGet**
> WorkspaceTemplate ApiWorkspaceTemplatesIdGet(ctx, id)
Get a workspace template

get workspace template by ID

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Workspace Template ID | 

### Return type

[**WorkspaceTemplate**](WorkspaceTemplate.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

