# Sprint 14 — API Endpoints

All endpoints are under `/api/v1/` and require tenant-scoping.

## List Providers

```
GET /api/v1/providers
```

**Query Parameters:**
- `offset` (int, default 0)
- `limit` (int, default 25)

**Returns:** Paginated list of providers for the authenticated organization.

## Get Provider

```
GET /api/v1/providers/{id}
```

**Returns:** Single provider by ID within the tenant scope.

## Create Provider

```
POST /api/v1/providers
```

**Body:**
```json
{
    "code": "hotel-booking",
    "name": "Hotel Booking Provider",
    "type": "hotel",
    "capabilities": ["AVAILABILITY", "RESERVATION"],
    "configuration": {
        "endpoint": "https://api.provider.com"
    },
    "secretRef": "hotel-provider-secret",
    "priority": 10
}
```

**Returns:** Created provider. Provider is created with `status = INACTIVE`.

**Note:** `secretRef` is never returned in any API response.

## Enable Provider

```
POST /api/v1/providers/{id}/enable
```

**Returns:** Updated provider with `status = ACTIVE`.

## Disable Provider

```
POST /api/v1/providers/{id}/disable
```

**Returns:** Updated provider with `status = INACTIVE`.

## Get Provider Capabilities

```
GET /api/v1/providers/{id}/capabilities
```

**Returns:** List of capabilities for the provider.

```json
{
    "provider_id": "uuid",
    "capabilities": ["AVAILABILITY", "RESERVATION"]
}
```

## Authorization

All provider endpoints enforce RBAC based on IAM organization roles:

| Action          | Required Role      |
|-----------------|---------------------|
| Create          | Admin, Manager, Owner |
| List            | Admin, Manager, Owner |
| Get             | Admin, Manager, Owner |
| Enable/Disable  | Owner, Admin      |
| Capabilities    | Admin, Manager, Owner |

## Security

- `secretRef` must never appear in any API response.
- Provider IDs are scoped to the organization — cross-tenant access returns 404.
- Provider creation requires a valid `secretRef` if the provider needs authentication.
- All responses are filtered through a whitelist; only fields explicitly listed in `ProviderResponse` are included.
