# \DefaultApi

All URIs are relative to *https://localhost*

Method | HTTP request | Description
------------- | ------------- | -------------
[**ApiHealthGet**](DefaultApi.md#ApiHealthGet) | **Get** /api/health | Health check
[**ApiVersionGet**](DefaultApi.md#ApiVersionGet) | **Get** /api/version | Version


# **ApiHealthGet**
> HealthCheck ApiHealthGet(ctx, )
Health check

Returns the status of the API server

### Required Parameters
This endpoint does not need any parameter.

### Return type

[**HealthCheck**](HealthCheck.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ApiVersionGet**
> Version ApiVersionGet(ctx, )
Version

Returns the version of the API server

### Required Parameters
This endpoint does not need any parameter.

### Return type

[**Version**](Version.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: Not defined
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

