package domain

// ConversationContext represents the structured context of a conversation.
type ConversationContext struct {
	CurrentIntent          *ConversationIntent   `json:"current_intent,omitempty"`
	CurrentEntities        []ConversationEntity  `json:"current_entities,omitempty"`
	CurrentListing         *ListingRef           `json:"current_listing,omitempty"`
	CurrentOrder           *OrderRef             `json:"current_order,omitempty"`
	CurrentReservation     *ReservationRef       `json:"current_reservation,omitempty"`
	CurrentSearchQuery     *DiscoveryQuery       `json:"current_search_query,omitempty"`
	SearchCriteria         *SearchCriteria       `json:"search_criteria,omitempty"`
	CurrentLocation        *SearchLocation       `json:"current_location,omitempty"`
	CurrentListingLocation *GeoPoint             `json:"current_listing_location,omitempty"`
	PendingAction          *CommerceAction       `json:"pending_action,omitempty"`
	PendingParameters      []PendingParameter    `json:"pending_parameters,omitempty"`
	MissingParameters      []MissingParameter    `json:"missing_parameters,omitempty"`
	ConfirmationState      ConfirmationFlowState `json:"confirmation_state"`
	LastActionID           string                `json:"last_action_id,omitempty"`
	LastActionResult       *CommerceActionResult `json:"last_action_result,omitempty"`
	ResultItems            []ResultItemRef       `json:"result_items,omitempty"`
	SpecialistCode         AISpecialistCode      `json:"specialist_code,omitempty"`
	PackCode               PackCode              `json:"pack_code,omitempty"`
	Language               string                `json:"language,omitempty"`
	LastActivityAt         int64                 `json:"last_activity_at"`
}

// SearchCriteria are the structured criteria of the latest search (vertical,
// place, stay length, party size, start). Follow-ups such as "book it" or "is it
// free?" reuse them instead of asking again.
type SearchCriteria struct {
	Vertical  Vertical `json:"vertical,omitempty"`
	Location  string   `json:"location,omitempty"`
	Nights    int      `json:"nights,omitempty"`
	PartySize int      `json:"party_size,omitempty"`
	StartAt   string   `json:"start_at,omitempty"`
	// What the person was looking for and how: the search words, a budget, an
	// ordering and a category. A refinement ("moins cher", "seulement à louer")
	// starts from these instead of from nothing.
	Text       string  `json:"text,omitempty"`
	PriceMin   float64 `json:"price_min,omitempty"`
	PriceMax   float64 `json:"price_max,omitempty"`
	Sort       string  `json:"sort,omitempty"`
	CategoryID string  `json:"category_id,omitempty"`
}

// IsZero reports whether no criterion is set.
func (c SearchCriteria) IsZero() bool {
	return c == SearchCriteria{}
}

// PendingParameter represents a parameter the system is waiting for the user to provide.
type PendingParameter struct {
	Name        string
	Description string
	Required    bool
}

// MissingParameter represents a parameter required for an action that the user has not yet provided.
type MissingParameter struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	EntityName  string   `json:"entity_name,omitempty"`
	Examples    []string `json:"examples,omitempty"`
}

// ConfirmationFlowState tracks the multi-step confirmation flow for an action.
type ConfirmationFlowState struct {
	ActionID          string
	State             ConfirmationState
	Summary           string
	Details           map[string]interface{}
	RequestedAt       int64
	ConfirmationToken string
}

// ResultItemRef is a reference to a result item (listing, order, reservation)
// stored in context for resolution of follow-up questions like "the first one".
type ResultItemRef struct {
	RefType  string      // "listing", "order", "reservation"
	ID       string      // canonical ID of the referenced entity
	Index    int         // 0-based position in the original result list
	Title    string      // human-readable title for display
	Subtitle string      // secondary text (price, status, etc.)
	Data     interface{} // full entity data if needed for context
}

// HasPendingConfirmation returns true if the context has a pending confirmation flow.
func (c ConversationContext) HasPendingConfirmation() bool {
	return c.ConfirmationState.ActionID != "" && !c.ConfirmationState.State.IsTerminal()
}

// HasMissingParameters returns true if the context has unresolved missing parameters.
func (c ConversationContext) HasMissingParameters() bool {
	return len(c.MissingParameters) > 0
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

// ReservationRef is a lightweight reference to a reservation.
type ReservationRef struct {
	ID        string
	Status    string
	StartAt   int64
	EndAt     int64
	PartySize int
	Quantity  int
}
