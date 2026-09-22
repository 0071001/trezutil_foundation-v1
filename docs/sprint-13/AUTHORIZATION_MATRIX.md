# Sprint 13 — Authorization Matrix

## Actor Types

| Actor Type | Description |
|------------|-------------|
| `CUSTOMER` | End customer placing orders/reservations |
| `SELLER` | Seller owning listings |
| `STORE_OPERATOR` | Operator managing a specific store |
| `ORGANIZATION_ADMIN` | Admin managing organization resources |
| `ORGANIZATION_OWNER` | Owner of organization |
| `ORGANIZATION_MANAGER` | Manager within organization |
| `PLATFORM_OPERATOR` | Platform-level operator |
| `PLATFORM_ADMIN` | Platform administrator |
| `SYSTEM` | Automated system processes |

---

## Inventory Permissions

| Action | Customer | Seller | Store Operator | Org Manager | Org Admin | Org Owner | Platform Operator | Platform Admin | System |
|--------|----------|--------|----------------|-------------|-----------|-----------|-------------------|----------------|--------|
| `GET /inventory` | ❌ | ✅ (own) | ✅ (own store) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `GET /inventory/{id}` | ❌ | ✅ (own) | ✅ (own store) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `POST /inventory/{id}/adjust` | ❌ | ✅ (own) | ✅ (own store) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

**Rules:**
- Seller can only access inventory for their own listings
- Store Operator can only access inventory for listings in their store
- Org roles can access all inventory in their organization
- Platform roles can access all inventory
- Adjustments require `RECEIVE`, `DAMAGE`, `CORRECTION`, `RESTOCK`, or `COUNT_ADJUSTMENT` reason

---

## Capacity Permissions

| Action | Customer | Seller | Store Operator | Org Manager | Org Admin | Org Owner | Platform Operator | Platform Admin | System |
|--------|----------|--------|----------------|-------------|-----------|-----------|-------------------|----------------|--------|
| `GET /capacity/{id}` | ❌ | ✅ (own) | ✅ (own store) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `POST /capacity/check` | ✅ (published) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

**Rules:**
- Capacity check for published listings is public
- Capacity unit listing requires seller/org permissions
- Customers can check availability for booking purposes

---

## Fulfillment Permissions

| Action | Customer | Seller | Store Operator | Org Manager | Org Admin | Org Owner | Platform Operator | Platform Admin | System |
|--------|----------|--------|----------------|-------------|-----------|-----------|-------------------|----------------|--------|
| `GET /fulfillments/{id}` | ✅ (own) | ✅ (own) | ✅ (own store) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `GET /orders/{id}/fulfillment` | ✅ (own) | ✅ (own) | ✅ (own store) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `GET /reservations/{id}/fulfillment` | ✅ (own) | ✅ (own) | ✅ (own store) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `POST /fulfillments/{id}/transition` | ❌ | ✅ (assigned) | ✅ (assigned) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `POST /fulfillments/{id}/assign` | ❌ | ✅ (own) | ✅ (own store) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| `POST /fulfillments/{id}/tracking` | ❌ | ✅ (assigned) | ✅ (assigned) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |

**Rules:**
- Customer can only view fulfillment for their own order/reservation
- Seller/Store Operator can view fulfillments for their listings
- Only assigned actor or org/platform admins can transition status
- Assignment requires org/platform admin or seller for their own listings
- Tracking updates only by assigned actor or admins

---

## Authorization Implementation

### Tenant Resolution

```go
func (h *InventoryHandler) getTenantID(r *http.Request) string {
    // From authenticated actor (set by middleware)
    actor := getAuthenticatedActor(r)
    return actor.OrgID
}
```

### Permission Checks

```go
func (uc *InventoryUseCases) canAccessInventory(ctx context.Context, actor *domain.User, tenantID, listingID string) bool {
    // Platform roles
    if actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator) {
        return true
    }
    
    // Org membership
    if tenantID != "" {
        role, err := uc.orgRepo.GetMemberRole(ctx, tenantID, actor.ID)
        if err == nil {
            switch role {
            case domain.OrganizationRoleOwner, 
                 domain.OrganizationRoleAdmin,
                 domain.OrganizationRoleManager:
                return true
            }
        }
    }
    
    // Seller ownership
    if listingID != "" {
        seller, err := uc.sellerRepo.FindByUserID(ctx, actor.ID)
        if err == nil && seller != nil {
            listing, err := uc.listingRepo.FindByID(ctx, listingID)
            if err == nil && listing.SellerID == seller.ID {
                return true
            }
        }
    }
    
    return false
}
```

---

## Idempotency & Authorization

- Idempotency keys scoped per tenant
- Same key + different actor = conflict (409)
- Authorization checked before idempotency lookup

---

## Audit Requirements

All inventory/capacity/fulfillment mutations must audit:

| Event | Actor | Target | Metadata |
|-------|-------|--------|----------|
| `inventory.adjusted` | Seller/Admin | InventoryResource | `delta_total`, `delta_reserved`, `delta_consumed`, `reason` |
| `capacity.reserved` | Customer/System | CapacityUnit | `quantity`, `reservation_id` |
| `capacity.released` | Customer/System | CapacityUnit | `quantity`, `reservation_id` |
| `fulfillment.transitioned` | Assigned/Admin | Fulfillment | `from_status`, `to_status` |
| `fulfillment.assigned` | Admin | Fulfillment | `assigned_to` |
| `fulfillment.tracking_updated` | Assigned/Admin | Fulfillment | `tracking_ref` |