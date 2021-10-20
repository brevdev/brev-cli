# \WorkspaceGroupsApi

All URIs are relative to *https://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**ApiWorkspaceGroupsGet**](WorkspaceGroupsApi.md#ApiWorkspaceGroupsGet) | **Get** /api/workspace_groups | Lists workspace groups
[**ApiWorkspaceGroupsIdGet**](WorkspaceGroupsApi.md#ApiWorkspaceGroupsIdGet) | **Get** /api/workspace_groups/{id} | Get a workspace group


# **ApiWorkspaceGroupsGet**
> []WorkspaceGroup ApiWorkspaceGroupsGet(ctx, )
Lists workspace groups

lists workspace groups the context user has permission to view.

### Required Parameters
This endpoint does not need any parameter.

### Return type

[**[]WorkspaceGroup**](WorkspaceGroup.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiWorkspaceGroupsIdGet**
> WorkspaceGroup ApiWorkspaceGroupsIdGet(ctx, id)
Get a workspace group

get workspace group by ID

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Workspace Group ID | 

### Return type

[**WorkspaceGroup**](WorkspaceGroup.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

