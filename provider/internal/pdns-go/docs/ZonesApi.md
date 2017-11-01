# \ZonesApi

All URIs are relative to *http://localhost:8081/api/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**ListZone**](ZonesApi.md#ListZone) | **Get** /servers/{server_id}/zones/{zone_id} | zone managed by a server
[**ListZones**](ZonesApi.md#ListZones) | **Get** /servers/{server_id}/zones | List all Zones in a server
[**PatchZone**](ZonesApi.md#PatchZone) | **Patch** /servers/{server_id}/zones/{zone_id} | Modifies present RRsets and comments. Returns 204 No Content on success.


# **ListZone**
> Zone ListZone(ctx, serverId, zoneId)
zone managed by a server

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context containing the authentication | nil if no authentication
  **serverId** | **string**| The id of the server to retrieve | 
  **zoneId** | **string**| The id of the zone to retrieve | 

### Return type

[**Zone**](Zone.md)

### Authorization

[APIKeyHeader](../README.md#APIKeyHeader)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **ListZones**
> []Zone ListZones(ctx, serverId)
List all Zones in a server

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context containing the authentication | nil if no authentication
  **serverId** | **string**| The id of the server to retrieve | 

### Return type

[**[]Zone**](Zone.md)

### Authorization

[APIKeyHeader](../README.md#APIKeyHeader)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **PatchZone**
> PatchZone(ctx, serverId, zoneId, zoneStruct)
Modifies present RRsets and comments. Returns 204 No Content on success.

### Required Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **ctx** | **context.Context** | context containing the authentication | nil if no authentication
  **serverId** | **string**| The id of the server to retrieve | 
  **zoneId** | **string**|  | 
  **zoneStruct** | [**Zone**](Zone.md)| The zone struct to patch with | 

### Return type

 (empty response body)

### Authorization

[APIKeyHeader](../README.md#APIKeyHeader)

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

