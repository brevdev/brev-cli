# \RolesApi

All URIs are relative to *https://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**ApiRolesGet**](RolesApi.md#ApiRolesGet) | **Get** /api/roles | List roles
[**ApiRolesIdGet**](RolesApi.md#ApiRolesIdGet) | **Get** /api/roles/{id} | Get a role


# **ApiRolesGet**
> []RoleJson ApiRolesGet(ctx, )
List roles

Retrieve all roles

### Required Parameters
This endpoint does not need any parameter.

### Return type

[**[]RoleJson**](RoleJSON.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiRolesIdGet**
> RoleJson ApiRolesIdGet(ctx, id)
Get a role

get role by ID

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **id** | **string**| Role ID | 

### Return type

[**RoleJson**](RoleJSON.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

