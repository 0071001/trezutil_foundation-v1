package events

// Event represents a domain event envelope.
type Event struct {
	EventID        string                 `json:"event_id"`
	EventName      string                 `json:"event_name"`
	AggregateID    string                 `json:"aggregate_id"`
	AggregateType  string                 `json:"aggregate_type"`
	OrganizationID string                 `json:"organization_id"`
	SellerID       string                 `json:"seller_id"`
	StoreID        string                 `json:"store_id"`
	ListingID      string                 `json:"listing_id"`
	Version        int                    `json:"version"`
	Payload        map[string]interface{} `json:"payload"`
	OccurredAt     int64                  `json:"occurred_at"`
	ProducedBy     string                 `json:"produced_by"`
}
