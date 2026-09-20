package domain

// ConversationContext represents the structured context of a conversation.
type ConversationContext struct {
	CurrentIntent          *ConversationIntent
	CurrentEntities        []ConversationEntity
	CurrentListing         *ListingRef
	CurrentOrder           *OrderRef
	CurrentSearchQuery     *DiscoveryQuery
	CurrentLocation        *SearchLocation
	CurrentListingLocation *GeoPoint
	PendingAction          *CommerceAction
	Language               string
	LastActivityAt         int64
}

// ListingRef is a lightweight reference to a listing.
type ListingRef struct {
	ID             string
	Title          string
	OrganizationID string
	SellerID       string
	StoreID        string
	Status         string
}

// OrderRef is a lightweight reference to an order.
type OrderRef struct {
	ID          string
	Status      string
	TotalAmount int64
	Currency    string
}
