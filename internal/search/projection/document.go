package projection

import "github.com/trezutil/trezutil/internal/domain"

// SearchListingDocument represents the OpenSearch search document.
type SearchListingDocument struct {
	ListingID          string                       `json:"listing_id"`
	OrganizationID     string                       `json:"organization_id"`
	SellerID           string                       `json:"seller_id"`
	StoreID            string                       `json:"store_id"`
	Title              string                       `json:"title"`
	Description        string                       `json:"description"`
	CategoryID         string                       `json:"category_id"`
	CategoryPath       []string                     `json:"category_path"`
	ListingType        string                       `json:"listing_type"`
	Vertical           string                       `json:"vertical,omitempty"`
	Place              string                       `json:"place,omitempty"`
	PricingModel       string                       `json:"pricing_model"`
	PriceAmount        float64                      `json:"price_amount"`
	PriceCurrency      string                       `json:"price_currency"`
	AvailabilityStatus string                       `json:"availability_status"`
	Quantity           *int                         `json:"quantity,omitempty"`
	Location           *domain.GeoPoint             `json:"location,omitempty"`
	LocationIDs        []string                     `json:"location_ids"`
	Attributes         []domain.AttributeProjection `json:"attributes"`
	MediaCount         int                          `json:"media_count"`
	PrimaryMediaURL    *string                      `json:"primary_media_url,omitempty"`
	SellerDisplayName  string                       `json:"seller_display_name"`
	SellerTrustLevel   int                          `json:"seller_trust_level"`
	StoreName          string                       `json:"store_name"`
	StoreStatus        string                       `json:"store_status"`
	Status             string                       `json:"status"`
	PublishedAt        *int64                       `json:"published_at,omitempty"`
	UpdatedAt          int64                        `json:"updated_at"`
	FreshnessScore     float64                      `json:"freshness_score"`
	Metadata           map[string]interface{}       `json:"metadata,omitempty"`
}

// SearchListingDocumentMapper maps domain projections to OpenSearch documents.
type SearchListingDocumentMapper struct{}

// NewSearchListingDocumentMapper creates a new mapper.
func NewSearchListingDocumentMapper() *SearchListingDocumentMapper {
	return &SearchListingDocumentMapper{}
}

// MapToDocument maps a domain ListingProjection to a SearchListingDocument.
func (m *SearchListingDocumentMapper) MapToDocument(projection domain.ListingProjection) (*SearchListingDocument, error) {
	doc := &SearchListingDocument{
		ListingID:          projection.ListingID,
		OrganizationID:     projection.OrganizationID,
		SellerID:           projection.SellerID,
		StoreID:            projection.StoreID,
		Title:              projection.Title,
		Description:        projection.Description,
		CategoryID:         projection.CategoryID,
		CategoryPath:       projection.CategoryPath,
		ListingType:        projection.ListingType,
		Vertical:           projection.Vertical,
		Place:              projection.Place,
		PricingModel:       projection.PricingModel,
		PriceAmount:        projection.PriceAmount,
		PriceCurrency:      projection.PriceCurrency,
		AvailabilityStatus: projection.AvailabilityStatus,
		Quantity:           projection.Quantity,
		Location:           projection.Location,
		LocationIDs:        projection.LocationIDs,
		Attributes:         projection.Attributes,
		MediaCount:         projection.MediaCount,
		PrimaryMediaURL:    projection.PrimaryMediaURL,
		SellerDisplayName:  projection.SellerDisplayName,
		SellerTrustLevel:   projection.SellerTrustLevel,
		StoreName:          projection.StoreName,
		StoreStatus:        projection.StoreStatus,
		Status:             projection.Status,
		PublishedAt:        &projection.PublishedAt,
		UpdatedAt:          projection.UpdatedAt,
		FreshnessScore:     projection.FreshnessScore,
		Metadata:           projection.Metadata,
	}

	if projection.PublishedAt == 0 {
		doc.PublishedAt = nil
	}

	return doc, nil
}
