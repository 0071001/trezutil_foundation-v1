package domain

import "context"

/*
 * Read models of the seller workspace and the administration console beyond orders, bookings and businesses.
 * Every one is computed from the platform's tables when asked; nothing is stored for the consoles. Contact data
 * (phone numbers, email addresses) stays out of every model a seller or an operator lists, except the masked
 * phone of a team member, which the team page needs to tell two people apart.
 */

// Page is the common paging of the console lists.
type Page struct {
	Limit  int
	Offset int
}

// --- Seller: money -----------------------------------------------------------------------------------------

// PaymentRow is one payment as a console lists it. Card details and provider secrets are never part of it.
type PaymentRow struct {
	ID               string
	OrderID          string
	OrganizationID   string
	OrganizationName string // filled for the administration list
	AmountMinor      int64
	Currency         string
	Method           string
	ProviderCode     string
	Status           string
	RefundedMinor    int64
	FailureReason    string
	CreatedAt        int64
	CapturedAt       int64
}

// MoneyTotals is what a business took in one currency over a window. Currencies are never added together.
type MoneyTotals struct {
	Currency      string `json:"currency"`
	CapturedMinor int64  `json:"captured_minor"`
	RefundedMinor int64  `json:"refunded_minor"`
	NetMinor      int64  `json:"net_minor"`
	Payments      int    `json:"payments"`
	Failed        int    `json:"failed"`
	// EarnedMinor is the value of the business's orders that count as revenue (paid to completed), whatever the
	// way they were paid, so a business paid in cash still sees what it sold.
	EarnedMinor int64 `json:"earned_minor"`
	Orders      int   `json:"orders"`
}

// PaymentFilter narrows a payments list.
type PaymentFilter struct {
	OrganizationID string // "" for the whole platform (administration)
	Status         string
	Query          string // payment or order id prefix, business name (administration)
}

// --- Seller: customers -------------------------------------------------------------------------------------

// CustomerRow is one person who bought or booked from a business: the name they chose, what they did and
// when. Policy: a seller sees no phone number or email address here (the customer shares those in an order).
type CustomerRow struct {
	CustomerID   string
	DisplayName  string
	Orders       int
	Reservations int
	// Spent is the value of their orders that count as revenue, per currency.
	Spent   map[string]int64
	FirstAt int64
	LastAt  int64
}

// --- Seller: fulfilment ------------------------------------------------------------------------------------

// FulfillmentRow is one delivery, pickup or appointment to carry out, with the order or booking it serves.
type FulfillmentRow struct {
	ID            string
	Reference     string
	Type          string
	Status        string
	OrderID       string
	ReservationID string
	Title         string
	OrderStatus   string
	CustomerName  string
	TrackingRef   string
	AssignedTo    string
	AssignedName  string
	CreatedAt     int64
	UpdatedAt     int64
}

// --- Seller: availability ----------------------------------------------------------------------------------

// CapacitySlot is one bookable period of a resource and how much of it is taken.
type CapacitySlot struct {
	ID       string `json:"id"`
	StartAt  int64  `json:"start_at"`
	EndAt    int64  `json:"end_at"`
	Capacity int64  `json:"capacity"`
	Reserved int64  `json:"reserved"`
}

// CapacityResource is one stock or capacity record of a listing, with its windows in the period asked.
type CapacityResource struct {
	InventoryID  string
	ListingID    string
	ListingTitle string
	ListingType  string
	ResourceType string
	Total        int64
	Reserved     int64
	Consumed     int64
	Windows      []CapacitySlot
}

// --- Seller: usage, plan and integrations ------------------------------------------------------------------

// CreditAccountRow is one credit account of a business for one period.
type CreditAccountRow struct {
	AccountID string
	OwnerType string
	OwnerID   string
	PeriodID  string
	PlanID    string
	Granted   int64
	Reserved  int64
	Consumed  int64
	UpdatedAt int64
}

// UsageEntry is one line of the usage ledger.
type UsageEntry struct {
	ID              string
	ActionType      string
	TransactionType string
	Amount          int64
	BalanceAfter    int64
	PeriodID        string
	CreatedAt       int64
}

// SubscriptionRow is a subscription with the plan it is on.
type SubscriptionRow struct {
	ID                string
	OrganizationID    string
	OrganizationName  string
	OwnerType         string
	Status            string
	PeriodID          string
	CancelAtPeriodEnd bool
	StartedAt         int64
	PlanID            string
	PlanName          string
	PlanSlug          string
	PriceMinor        int64
	Currency          string
	BillingCycle      string
	QuotaAmount       int64
}

// PlanRow is one plan of the catalogue.
type PlanRow struct {
	ID             string
	OrganizationID string
	Name           string
	Slug           string
	PriceMinor     int64
	Currency       string
	BillingCycle   string
	QuotaAmount    int64
	CreditPack     int64
	Status         string
	Subscribers    int
}

// UsageSummary is the usage of one business: its accounts, its latest ledger lines and its subscription.
type UsageSummary struct {
	Accounts     []CreditAccountRow
	Ledger       []UsageEntry
	Subscription *SubscriptionRow
	Policies     []UsagePolicyRow
}

// UsagePolicyRow is what one action costs.
type UsagePolicyRow struct {
	ActionType string `json:"action_type"`
	Channel    string `json:"channel"`
	ActorType  string `json:"actor_type"`
	Cost       int64  `json:"cost"`
	Active     bool   `json:"active"`
}

// ProviderRow is one provider connection, without its configuration or secret reference.
type ProviderRow struct {
	ID               string
	OrganizationID   string
	OrganizationName string
	Code             string
	Name             string
	Type             string
	Status           string
	HealthStatus     string
	Capabilities     []string
	Priority         int
	UpdatedAt        int64
	// Executions over the window asked (7 days by default).
	Attempts    int
	Failures    int
	P50Ms       int64
	P95Ms       int64
	LastFailure int64
	LastError   string
}

// ChannelAccountRow is a messaging channel linked to a business; the external id is masked.
type ChannelAccountRow struct {
	ID         string `json:"id"`
	Channel    string `json:"channel"`
	ExternalID string `json:"external_id"`
	Status     string `json:"status"`
	CreatedAt  int64  `json:"-"`
}

// --- Seller: team ------------------------------------------------------------------------------------------

// TeamMember is one member of a business. The phone number is masked (+243 ••• ••• 12).
type TeamMember struct {
	UserID      string
	DisplayName string
	MaskedPhone string
	Role        string
	JoinedAt    int64
	Status      string
}

// TeamInvitation is a pending invitation by phone number: it becomes a membership when that number next signs
// in, so an invitation never tells the business whether the number already has an account.
type TeamInvitation struct {
	ID          string
	MaskedPhone string
	Role        string
	InvitedBy   string
	InvitedName string
	CreatedAt   int64
	ExpiresAt   int64
}

// --- Administration ----------------------------------------------------------------------------------------

// AdminListing is one listing in the review queue, with the quality flags computed on the server.
type AdminListing struct {
	ID               string
	Title            string
	Status           string
	Vertical         string
	ListingType      string
	PriceAmount      float64
	Currency         string
	OrganizationID   string
	OrganizationName string
	CategoryName     string
	Images           int
	CreatedAt        int64
	UpdatedAt        int64
	Flags            []string
}

// AdminListingFilter narrows the listing queue.
type AdminListingFilter struct {
	Query    string
	Status   string
	Vertical string
	Flag     string // "no_image" | "no_price"
}

// ExceptionRow is one thing that went wrong and may need somebody: a failed payment, a webhook that could not
// be processed, an order paid but not confirmed for a long time.
type ExceptionRow struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	Reference string `json:"reference"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
	Business  string `json:"business"`
	At        int64  `json:"-"`
}

// Exceptions groups the counts (over the window) and the most recent rows of every kind.
type Exceptions struct {
	Counts map[string]int
	Recent []ExceptionRow
}

// ChannelHealth is the delivery health of one inbound channel over the last 24 hours.
type ChannelHealth struct {
	Channel         string
	Events24h       int
	Failed24h       int
	Pending         int
	LastEventAt     int64
	LastFailureAt   int64
	LastError       string
	Accounts        int
	Conversations7d int
}

// StaffAccount is a staff account's sign-in state, for the security view.
type StaffAccount struct {
	UserID         string
	DisplayName    string
	Email          string
	Roles          []string
	Status         string
	LastLoginAt    int64
	FailedAttempts int
	LockedUntil    int64
	PasswordSetAt  int64
}

// ConversationSummary is the assistant's activity over a window, for the AI centre.
type ConversationSummary struct {
	Days            int
	Conversations   int
	ByChannel       map[string]int
	ByState         map[string]int
	MessagesIn      int
	MessagesOut     int
	Confirmations   map[string]int
	ResultsByType   map[string]int
	PerDay          []DayCount
	AIExecutions    int // rows in ai_execution_records: 0 until the AI gateway records its calls
	ActiveCustomers int
}

// MonetizationSummary is the plans catalogue and what the platform's credits did over a window.
type MonetizationSummary struct {
	Plans                 []PlanRow
	SubscriptionsByStatus map[string]int
	Subscriptions         []SubscriptionRow
	Accounts              int
	Granted               int64
	Consumed              int64
	Reserved              int64
	LedgerByType          map[string]int64
	Policies              []UsagePolicyRow
}

// ConsoleReader is the read side behind the new console modules.
type ConsoleReader interface {
	Payments(ctx context.Context, f PaymentFilter, p Page) ([]PaymentRow, int, error)
	MoneyTotals(ctx context.Context, organizationID string, from, to int64) ([]MoneyTotals, error)
	Customers(ctx context.Context, organizationID, query string, p Page) ([]CustomerRow, int, error)
	Fulfillments(ctx context.Context, organizationID, status string, p Page) ([]FulfillmentRow, int, error)
	Capacity(ctx context.Context, organizationID string, from, to int64) ([]CapacityResource, error)
	Usage(ctx context.Context, organizationID string) (*UsageSummary, error)
	Providers(ctx context.Context, organizationID string, since int64) ([]ProviderRow, error)
	ChannelAccounts(ctx context.Context, organizationID string) ([]ChannelAccountRow, error)
	Team(ctx context.Context, organizationID string) ([]TeamMember, error)

	AdminListings(ctx context.Context, f AdminListingFilter, p Page) ([]AdminListing, int, error)
	Exceptions(ctx context.Context, since, stuckBefore int64, limit int) (*Exceptions, error)
	ChannelHealth(ctx context.Context, now int64) ([]ChannelHealth, error)
	StaffAccounts(ctx context.Context) ([]StaffAccount, error)
	ConversationSummary(ctx context.Context, now int64, days int) (*ConversationSummary, error)
	Monetization(ctx context.Context, since int64) (*MonetizationSummary, error)
	SetProviderStatus(ctx context.Context, providerID, from, to string) (organizationID string, err error)
}
