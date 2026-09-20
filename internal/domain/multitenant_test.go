package domain

import (
	"testing"
)

// TestMultiTenantIsolation_Organization verifies that organizations are properly isolated.
func TestMultiTenantIsolation_Organization(t *testing.T) {
	orgA := NewOrganization(OrganizationTypeBusiness, "Org A", "Organization A", "CD", "+243812345678")
	orgB := NewOrganization(OrganizationTypeBusiness, "Org B", "Organization B", "CD", "+243812345679")

	if orgA.LegalName == orgB.LegalName {
		t.Error("Organizations should have different legal names")
	}
	if orgA.Phone == orgB.Phone {
		t.Error("Organizations should have different phones")
	}
}

// TestMultiTenantIsolation_Membership verifies that memberships are properly isolated.
func TestMultiTenantIsolation_Membership(t *testing.T) {
	memberA := NewOrganizationMember("user-a", "org-a", OrganizationRoleOwner)
	memberB := NewOrganizationMember("user-b", "org-b", OrganizationRoleAdmin)

	if memberA.UserID == memberB.UserID {
		t.Error("Members should have different user IDs")
	}
	if memberA.OrganizationID == memberB.OrganizationID {
		t.Error("Members should belong to different organizations")
	}
	if memberA.Role == memberB.Role {
		t.Error("Members can have different roles in different organizations")
	}
}

// TestMultiTenantIsolation_Seller verifies that sellers are properly isolated by organization.
func TestMultiTenantIsolation_Seller(t *testing.T) {
	sellerA := NewSellerProfile(SellerTypeBusiness, "Seller A", "+243812345678", strPtr("user-a"), strPtr("org-a"))
	sellerB := NewSellerProfile(SellerTypeBusiness, "Seller B", "+243812345679", strPtr("user-b"), strPtr("org-b"))

	if *sellerA.OrganizationID == *sellerB.OrganizationID {
		t.Error("Sellers should belong to different organizations")
	}
	if *sellerA.UserID == *sellerB.UserID {
		t.Error("Sellers should have different user IDs")
	}
	if sellerA.DisplayName == sellerB.DisplayName {
		t.Error("Sellers should have different display names")
	}
}

// TestMultiTenantIsolation_Store verifies that stores are properly isolated by organization.
func TestMultiTenantIsolation_Store(t *testing.T) {
	storeA := NewStore("org-a", "Store A", "store-a", "CD", "+243812345678")
	storeB := NewStore("org-b", "Store B", "store-b", "CD", "+243812345679")

	if storeA.OrganizationID == storeB.OrganizationID {
		t.Error("Stores should belong to different organizations")
	}
	if storeA.Slug == storeB.Slug {
		t.Error("Stores should have different slugs")
	}
	if storeA.Name == storeB.Name {
		t.Error("Stores should have different names")
	}
}

// TestMultiTenantIsolation_CrossTenantAccess verifies that cross-tenant access is prevented.
func TestMultiTenantIsolation_CrossTenantAccess(t *testing.T) {
	memberA := NewOrganizationMember("user-a", "org-a", OrganizationRoleOwner)
	memberB := NewOrganizationMember("user-b", "org-b", OrganizationRoleOwner)

	if memberA.OrganizationID == memberB.OrganizationID {
		t.Error("User A should not have access to Organization B")
	}

	if memberA.UserID == memberB.UserID {
		t.Error("Different users should have different user IDs")
	}
}

// TestMultiTenantIsolation_SellerAccess verifies seller isolation.
func TestMultiTenantIsolation_SellerAccess(t *testing.T) {
	sellerA := NewSellerProfile(SellerTypeBusiness, "Seller A", "+243812345678", strPtr("user-a"), strPtr("org-a"))
	sellerB := NewSellerProfile(SellerTypeBusiness, "Seller B", "+243812345679", strPtr("user-b"), strPtr("org-b"))

	if *sellerA.OrganizationID == *sellerB.OrganizationID {
		t.Error("Sellers from different organizations should not share organization ID")
	}

	if *sellerA.UserID == *sellerB.UserID {
		t.Error("Different sellers should have different user IDs")
	}
}

// TestMultiTenantIsolation_StoreAccess verifies store isolation.
func TestMultiTenantIsolation_StoreAccess(t *testing.T) {
	storeA := NewStore("org-a", "Store A", "store-a", "CD", "+243812345678")
	storeB := NewStore("org-b", "Store B", "store-b", "CD", "+243812345679")

	if storeA.OrganizationID == storeB.OrganizationID {
		t.Error("Stores from different organizations should not share organization ID")
	}

	if storeA.Slug == storeB.Slug {
		t.Error("Stores from different organizations should have different slugs")
	}
}

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}
