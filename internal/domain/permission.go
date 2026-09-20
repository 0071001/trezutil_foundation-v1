package domain

type Permission string

const (
	PermissionCatalogRead    Permission = "catalog.read"
	PermissionCatalogWrite   Permission = "catalog.write"
	PermissionListingCreate  Permission = "listing.create"
	PermissionListingUpdate  Permission = "listing.update"
	PermissionOrderRead      Permission = "order.read"
	PermissionOrderCreate    Permission = "order.create"
	PermissionPaymentRead    Permission = "payment.read"
	PermissionPaymentExecute Permission = "payment.execute"
	PermissionSellerManage   Permission = "seller.manage"
	PermissionAdminManage    Permission = "admin.manage"
	PermissionUserRead       Permission = "user.read"
	PermissionUserUpdate     Permission = "user.update"
	PermissionRoleAssign     Permission = "role.assign"
	PermissionRoleRevoke     Permission = "role.revoke"
)

var AllPermissions = []Permission{
	PermissionCatalogRead,
	PermissionCatalogWrite,
	PermissionListingCreate,
	PermissionListingUpdate,
	PermissionOrderRead,
	PermissionOrderCreate,
	PermissionPaymentRead,
	PermissionPaymentExecute,
	PermissionSellerManage,
	PermissionAdminManage,
	PermissionUserRead,
	PermissionUserUpdate,
	PermissionRoleAssign,
	PermissionRoleRevoke,
}

func (p Permission) IsValid() bool {
	switch p {
	case PermissionCatalogRead, PermissionCatalogWrite, PermissionListingCreate, PermissionListingUpdate,
		PermissionOrderRead, PermissionOrderCreate, PermissionPaymentRead, PermissionPaymentExecute,
		PermissionSellerManage, PermissionAdminManage, PermissionUserRead, PermissionUserUpdate,
		PermissionRoleAssign, PermissionRoleRevoke:
		return true
	default:
		return false
	}
}
