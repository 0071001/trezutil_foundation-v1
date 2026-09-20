# Sprint 12 — Authorization Matrix

## Actor Types

| Actor Type | Description |
|------------|-------------|
| `customer` | End user making reservations |
| `seller` | Listing owner/manager |
| `org_member` | Organization member (owner/admin/manager/staff) |
| `admin` | Platform admin |
| `operator` | Platform operator |

## Resource Ownership

| Resource | Owner |
|----------|-------|
| Listing | Seller (via SellerProfile → Organization) |
| Reservation | Customer (CustomerID) + Organization (OrganizationID) |

## Permission Rules

### Create Reservation
```
ALLOWED:
- Customer (any authenticated user) for published listings
- Org members (owner/admin/manager) for their org's listings
- Seller for their own listings
- Admin/Operator (any)

DENIED:
- Unauthenticated
- Customer for draft/paused listings
- Org members from other orgs
- Seller for other sellers' listings
```

### Get Reservation
```
ALLOWED:
- Reservation customer (CustomerID matches actor)
- Org members of reservation's organization (owner/admin/manager)
- Seller of the listing
- Admin/Operator

DENIED:
- Other customers
- Org members of other orgs
- Unauthenticated
```

### Cancel Reservation
```
ALLOWED:
- Reservation customer (if status allows: PENDING, CONFIRMED)
- Org members of reservation's organization (owner/admin/manager)
- Seller of the listing (if status allows)
- Admin/Operator

DENIED:
- Other customers
- Cancellation of COMPLETED reservations
- Cancellation of already CANCELLED reservations
- Unauthenticated
```

### Check Availability
```
ALLOWED:
- Any authenticated actor
- Public listings (no auth required for GET /v1/availability in future)

DENIED:
- Unauthenticated (currently requires user_id)
```

### Modify Reservation
```
NOT IMPLEMENTED — See MODIFY_RESERVATION = NOT_IMPLEMENTED
```

## Implementation

### canCreateReservation (ReservationUseCasesImpl)
```go
func (u *ReservationUseCasesImpl) canCreateReservation(ctx, actor, tenantID) bool {
    // Platform roles
    if actor.HasRole(RoleAdmin) || actor.HasRole(RoleOperator) {
        return true
    }
    // Org membership
    if memberRole == Owner || memberRole == Admin || memberRole == Manager {
        return true
    }
    // Seller ownership
    if seller != nil && seller.UserID == actor.ID {
        return true
    }
    return false
}
```

### canAccessReservation (ReservationUseCasesImpl)
```go
func (u *ReservationUseCasesImpl) canAccessReservation(ctx, reservation, actorID) bool {
    // Customer owns reservation
    if reservation.CustomerID == actorID {
        return true
    }
    // Org member with sufficient role
    if memberRole == Owner || memberRole == Admin || memberRole == Manager {
        return true
    }
    return false
}
```

## Matrix Summary

| Action | Customer (owner) | Customer (other) | Seller (owner) | Seller (other) | Org Member (owner) | Org Member (other) | Admin/Operator |
|--------|------------------|------------------|----------------|----------------|--------------------|--------------------|----------------|
| Check Availability | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Create Reservation | ✅ | ✅* | ✅ | ❌ | ✅ | ❌ | ✅ |
| Get Reservation | ✅ | ❌ | ✅ | ❌ | ✅ | ❌ | ✅ |
| Cancel Reservation | ✅** | ❌ | ✅** | ❌ | ✅** | ❌ | ✅ |
| Modify Reservation | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |

* For published listings only
** Only if status is PENDING or CONFIRMED

## Security Notes

1. **Never trust client-provided**: `tenant_id`, `customer_id`, `seller_id`, `organization_id`
2. **Server-side resolution**: All IDs resolved from authenticated actor + listing
3. **Tenant isolation**: Cross-tenant queries return 404 (not 403) to avoid enumeration
4. **Audit logging**: All reservation actions recorded with actor, tenant, action, target

## Status

**DONE** — Implemented in `internal/application/reservations/reservation_use_cases.go`