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

// AdminOrder is one order as the administration console lists it: enough to find it and to see where it stands,
// with the names of the business and of the customer and no contact data.
type AdminOrder struct {
	ID               string
	Status           string
	TotalMinor       int64
	Currency         string
	Title            string // the first line of the order
	ItemCount        int
	OrganizationID   string
	OrganizationName string
	CustomerID       string
	CustomerName     string
	CreatedAt        int64
	UpdatedAt        int64
}

// AdminReservation is one booking as the administration console lists it.
type AdminReservation struct {
	ID               string
	Status           string
	Title            string // the listing booked
	ListingID        string
	OrganizationID   string
	OrganizationName string
	CustomerID       string
	CustomerName     string
	StartAt          int64
	EndAt            int64
	Quantity         int
	PartySize        int
	CreatedAt        int64
}

// AdminOrganization is one business as the administration console lists it. Contact data stays out of the list.
type AdminOrganization struct {
	ID                 string
	DisplayName        string
	Type               string
	Status             string
	VerificationStatus string
	City               string
	Country            string
	Members            int
	Stores             int
	PublishedListings  int
	Orders             int
	CreatedAt          int64
}

// AdminListFilter narrows an administration listing. Empty fields match everything.
type AdminListFilter struct {
	// Query matches an id prefix and the names involved (business, customer, listing).
	Query string
	// Status is an exact status; the HTTP layer only passes values it knows.
	Status string
}
