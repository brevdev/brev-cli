# \TokenApi

All URIs are relative to *https://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**ApiTokenLoginPost**](TokenApi.md#ApiTokenLoginPost) | **Post** /api/token/login | Login
[**ApiTokenLogoutPost**](TokenApi.md#ApiTokenLogoutPost) | **Post** /api/token/logout | Logout
[**ApiTokenRefreshPost**](TokenApi.md#ApiTokenRefreshPost) | **Post** /api/token/refresh | Refresh auth tokens


# **ApiTokenLoginPost**
> Token ApiTokenLoginPost(ctx, payload)
Login

log into the API, generating a pair of JWT tokens on success

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **payload** | [**LoginRequest**](LoginRequest.md)| Login request | 

### Return type

[**Token**](Token.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiTokenLogoutPost**
> Message ApiTokenLogoutPost(ctx, )
Logout

Invalidates the context user session

### Required Parameters
This endpoint does not need any parameter.

### Return type

[**Message**](Message.md)

### Authorization

[ApiKeyAuth](../README.md#ApiKeyAuth)

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiTokenRefreshPost**
> Token ApiTokenRefreshPost(ctx, payload)
Refresh auth tokens

generates a new pair of JWT tokens using the provided refresh token

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
  **payload** | [**RefreshTokenRequest**](RefreshTokenRequest.md)| Refresh token request | 

### Return type

[**Token**](Token.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

