package domain

import "context"

// SearchSortField represents the field by which results may be sorted.
type SearchSortField string

const (
	SearchSortFieldRelevance    SearchSortField = "relevance"
	SearchSortFieldDistance     SearchSortField = "distance"
	SearchSortFieldPriceAsc     SearchSortField = "price_asc"
	SearchSortFieldPriceDesc    SearchSortField = "price_desc"
	SearchSortFieldNewest       SearchSortField = "newest"
	SearchSortFieldOldest       SearchSortField = "oldest"
	SearchSortFieldAvailability SearchSortField = "availability"
	// SearchSortFieldDiscount orders by the size of the saving (biggest first);
	// listings not on promotion come last.
	SearchSortFieldDiscount SearchSortField = "discount"
)

// SearchSortDirection represents the sort direction.
type SearchSortDirection string

const (
	SearchSortDirectionAsc  SearchSortDirection = "asc"
	SearchSortDirectionDesc SearchSortDirection = "desc"
)

// SearchSort represents the requested sort order.
type SearchSort struct {
	Field     SearchSortField
	Direction SearchSortDirection
}

// NewSearchSort creates a new SearchSort with sensible defaults.
func NewSearchSort(field SearchSortField, direction SearchSortDirection) SearchSort {
	if direction == "" {
		direction = SearchSortDirectionDesc
	}
	return SearchSort{
		Field:     field,
		Direction: direction,
	}
}

// PaginationMode represents the pagination strategy.
type PaginationMode string

const (
	PaginationModeCursor PaginationMode = "cursor"
	PaginationModeOffset PaginationMode = "offset"
)

// PaginationCursor represents cursor-based pagination.
type PaginationCursor struct {
	Mode     PaginationMode
	Offset   *int
	Cursor   string
	PageSize int
}

// NewPaginationCursor creates a new PaginationCursor.
func NewPaginationCursor(pageSize int) PaginationCursor {
	return PaginationCursor{
		Mode:     PaginationModeCursor,
		PageSize: pageSize,
	}
}

// SearchListingType represents the type of listing for search filtering.
type SearchListingType string

const (
	SearchListingTypePhysicalProduct SearchListingType = "physical_product"
	SearchListingTypeRealEstate      SearchListingType = "real_estate"
	SearchListingTypeVehicle         SearchListingType = "vehicle"
	SearchListingTypeFood            SearchListingType = "food"
	SearchListingTypeService         SearchListingType = "service"
	SearchListingTypeOther           SearchListingType = "other"
)

// SearchAvailabilityStatus represents the availability status for search filtering.
type SearchAvailabilityStatus string

const (
	SearchAvailabilityStatusInStock     SearchAvailabilityStatus = "in_stock"
	SearchAvailabilityStatusOutOfStock  SearchAvailabilityStatus = "out_of_stock"
	SearchAvailabilityStatusPreOrder    SearchAvailabilityStatus = "pre_order"
	SearchAvailabilityStatusReserved    SearchAvailabilityStatus = "reserved"
	SearchAvailabilityStatusAvailable   SearchAvailabilityStatus = "available"
	SearchAvailabilityStatusUnavailable SearchAvailabilityStatus = "unavailable"
)

// SearchLocation represents the origin location for geo-aware search.
type SearchLocation struct {
	LocationID *string
	Latitude   *float64
	Longitude  *float64
	Address    *string
}

// CategoryFilter represents a category filter with optional children inclusion.
type CategoryFilter struct {
	CategoryID      string
	IncludeChildren bool
}

// PriceRange represents price bounds.
type PriceRange struct {
	Min *float64
	Max *float64
}

// AttributeOperator represents the operator for attribute filtering.
type AttributeOperator string

const (
	AttributeOperatorEq       AttributeOperator = "eq"
	AttributeOperatorNeq      AttributeOperator = "neq"
	AttributeOperatorGt       AttributeOperator = "gt"
	AttributeOperatorGte      AttributeOperator = "gte"
	AttributeOperatorLt       AttributeOperator = "lt"
	AttributeOperatorLte      AttributeOperator = "lte"
	AttributeOperatorIn       AttributeOperator = "in"
	AttributeOperatorContains AttributeOperator = "contains"
	AttributeOperatorSearch   AttributeOperator = "search"
)

// AttributeFilter represents a typed attribute filter.
type AttributeFilter struct {
	Key      string
	Operator AttributeOperator
	Value    interface{}
}

// DeliveryRequirements represents delivery constraints (reserved for future use).
type DeliveryRequirements struct {
	ToLocationID    string
	ToLatitude      *float64
	ToLongitude     *float64
	RequireDelivery bool
}

// DiscoveryQuery is the canonical, channel-independent representation of a search
// or discovery request. It is independent of REST parameters, OpenSearch DSL,
// PostgreSQL SQL, WhatsApp message format, or any LLM provider.
type DiscoveryQuery struct {
	Text         string
	Category     *CategoryFilter
	Attributes   []AttributeFilter
	PriceRange   *PriceRange
	Currency     string
	Availability *SearchAvailabilityStatus
	Location     *SearchLocation
	RadiusMeters *float64
	SellerID     string
	StoreID      string
	Delivery     *DeliveryRequirements
	ListingType  *SearchListingType
	// Vertical restricts results to one commerce vertical. Callers validate it
	// against the vertical catalogue; an unknown vertical is an error, never OTHER.
	Vertical Vertical
	// OnPromotion restricts results to listings currently on sale (a regular
	// price above the selling price).
	OnPromotion bool
	// Place restricts results to listings whose place (store address, city and
	// province, listing locations) or title/description contains every word,
	// e.g. "Gombe". Geo-radius search stays in Location/RadiusMeters.
	Place      string
	Sort       *SearchSort
	Pagination PaginationCursor
}

// MaxSearchPlaceLength bounds DiscoveryQuery.Place.
const MaxSearchPlaceLength = 100

// SearchResult represents a single search result projection.
// It is distinct from the authoritative Listing domain entity.
type SearchResult struct {
	ListingID          string
	Title              string
	Vertical           string
	PriceAmount        float64
	PriceCurrency      string
	AvailabilityStatus string
	DistanceMeters     *float64
	Score              float64
	MatchedAttributes  []MatchedAttribute
	Location           *SearchResultLocation
	Seller             *SearchResultSeller
	Store              *SearchResultStore
	PrimaryMediaURL    *string
	// CompareAtAmount is the regular price a sale replaced; nil when the listing
	// is not on promotion. DiscountPercent is the saving (0 when none).
	CompareAtAmount *float64
	DiscountPercent int
	PublishedAt     int64
}

// MatchedAttribute represents an attribute that matched during search.
type MatchedAttribute struct {
	Key   string
	Value interface{}
}

// SearchResultLocation represents location information in a search result.
type SearchResultLocation struct {
	City         *string
	Neighborhood *string
	// Latitude and Longitude place the listing on a map (the primary location's
	// coordinates); nil when its location has none.
	Latitude  *float64
	Longitude *float64
}

// SearchResultSeller represents seller information in a search result.
type SearchResultSeller struct {
	SellerID    string
	DisplayName string
	TrustLevel  int
}

// SearchResultStore represents store information in a search result.
type SearchResultStore struct {
	StoreID string
	Name    string
}

// SearchResultPage represents a page of search results.
type SearchResultPage struct {
	Items               []SearchResult
	Total               int
	HasNext             bool
	NextCursor          *string
	QueryInterpretation *QueryInterpretation
}

// QueryInterpretation represents AI-injected interpretation of the query (future).
type QueryInterpretation struct {
	OriginalText     string
	CategoryInferred *string
	LocationInferred *string
}

// SuggestRequest represents a suggestion/autocomplete request.
type SuggestRequest struct {
	Text       string
	CategoryID *string
	Limit      int
}

// SuggestResponse represents a suggestion/autocomplete response.
type SuggestResponse struct {
	Suggestions []Suggestion
}

// Suggestion represents a single autocomplete suggestion.
type Suggestion struct {
	Text       string
	CategoryID *string
	Type       string
}

// SearchErrorCode represents a search error code.
type SearchErrorCode string

const (
	SearchErrorCodeInvalidRequest SearchErrorCode = "SEARCH_INVALID_REQUEST"
	SearchErrorCodeUnavailable    SearchErrorCode = "SEARCH_UNAVAILABLE"
	SearchErrorCodeRateLimited    SearchErrorCode = "SEARCH_RATE_LIMITED"
)

// SearchError represents a structured search error.
type SearchError struct {
	Code    SearchErrorCode
	Message string
	Details []string
}

// Error implements the error interface.
func (e SearchError) Error() string {
	return e.Message
}

// SearchEnginePort defines the interface for search engine implementations.
// Implementations must not expose OpenSearch-specific types.
type SearchEnginePort interface {
	ExecuteSearch(ctx context.Context, query DiscoveryQuery) (SearchResultPage, error)
}

// SearchHealth represents the health status of the search engine.
type SearchHealth string

const (
	SearchHealthAvailable   SearchHealth = "available"
	SearchHealthUnavailable SearchHealth = "unavailable"
)

// SearchIndexPort defines the interface for search index operations.
type SearchIndexPort interface {
	IndexListingDocument(ctx context.Context, document interface{}) error
	DeleteListingDocument(ctx context.Context, listingID string) error
}

// VisibilityResolver defines the interface for resolving listing visibility.
type VisibilityResolver interface {
	ResolveVisibility(ctx context.Context, actorID string) (SearchVisibility, error)
}

// SearchVisibility represents the visibility scope for a search request.
type SearchVisibility struct {
	Mode           SearchVisibilityMode
	OrganizationID *string
}

// SearchVisibilityMode represents the visibility mode.
type SearchVisibilityMode string

const (
	SearchVisibilityModePublic   SearchVisibilityMode = "public"
	SearchVisibilityModeInternal SearchVisibilityMode = "internal"
)

// AuthorizationResolver defines the interface for resolving search authorization.
type AuthorizationResolver interface {
	AuthorizeSearch(ctx context.Context, actorID string, visibility SearchVisibility) error
}

// ListingProjectionReader defines the interface for reading listing projections.
type ListingProjectionReader interface {
	GetListingProjection(ctx context.Context, listingID string) (ListingProjection, error)
}

// ListingProjection represents a denormalized listing projection for search.
type ListingProjection struct {
	ListingID          string
	OrganizationID     string
	SellerID           string
	StoreID            string
	Title              string
	Description        string
	CategoryID         string
	CategoryPath       []string
	ListingType        string
	Vertical           string
	Place              string
	PricingModel       string
	PriceAmount        float64
	PriceCurrency      string
	AvailabilityStatus string
	Quantity           *int
	Location           *GeoPoint
	LocationIDs        []string
	Attributes         []AttributeProjection
	MediaCount         int
	PrimaryMediaURL    *string
	SellerDisplayName  string
	SellerTrustLevel   int
	StoreName          string
	StoreStatus        string
	Status             string
	PublishedAt        int64
	UpdatedAt          int64
	FreshnessScore     float64
	// CompareAtAmount / DiscountPercent describe an active promotion (see Listing.Promotion).
	CompareAtAmount *float64
	DiscountPercent int
	Metadata        map[string]interface{}
}

// GeoPoint represents a geographic point.
type GeoPoint struct {
	Latitude  float64 `json:"lat"`
	Longitude float64 `json:"lon"`
}

// AttributeProjection represents a searchable attribute projection.
type AttributeProjection struct {
	Key          string
	Name         string
	Type         string
	ValueText    *string
	ValueNumber  *float64
	ValueBoolean *bool
	ValueDate    *int64
	ValueSelect  *string
	Searchable   bool
}

// CursorCodec defines the interface for encoding and decoding pagination cursors.
type CursorCodec interface {
	Encode(cursor PaginationCursor) (string, error)
	Decode(encoded string) (PaginationCursor, error)
}

// Clock defines the interface for time operations.
type Clock interface {
	Now() int64
}
