package domain

type Role string

const (
	RoleCustomer        Role = "customer"
	RoleSeller          Role = "seller"
	RoleStaff           Role = "staff"
	RoleAdmin           Role = "admin"
	RoleOperator        Role = "operator"
	RoleModerator       Role = "moderator"
	RoleDeliveryPartner Role = "delivery_partner"
)

var AllRoles = []Role{
	RoleCustomer,
	RoleSeller,
	RoleStaff,
	RoleAdmin,
	RoleOperator,
	RoleModerator,
	RoleDeliveryPartner,
}

func (r Role) IsValid() bool {
	switch r {
	case RoleCustomer, RoleSeller, RoleStaff, RoleAdmin, RoleOperator, RoleModerator, RoleDeliveryPartner:
		return true
	default:
		return false
	}
}
