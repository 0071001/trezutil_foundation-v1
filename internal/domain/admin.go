package domain

// AdminOverview is the platform's headline numbers for the administration console.
type AdminOverview struct {
	GeneratedAt int64

	UsersTotal     int
	UsersActive    int
	UsersSuspended int
	UsersNew7d     int
	UsersByRole    map[string]int

	Organizations     int
	ListingsByStatus  map[string]int
	OrdersTotal       int
	Orders7d          int
	ReservationsTotal int
	Conversations7d   int
}

// AdminUser is one account as the administration console lists it.
type AdminUser struct {
	ID          string
	Email       string
	Phone       string
	DisplayName string
	Status      string
	Roles       []string
	CreatedAt   int64
	LastLoginAt int64 // 0 when the account never signed in with a password
}
