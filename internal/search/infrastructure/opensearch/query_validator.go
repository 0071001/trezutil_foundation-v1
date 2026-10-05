package opensearch

import (
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

// QueryLimits defines safe operational limits for search queries.
type QueryLimits struct {
	MaxPageSize         int
	MaxRadiusMeters     float64
	MaxTextLength       int
	MaxFilters          int
	MaxAttributeFilters int
	MaxSortClauses      int
}

// DefaultQueryLimits returns default query limits.
func DefaultQueryLimits() QueryLimits {
	return QueryLimits{
		MaxPageSize:         100,
		MaxRadiusMeters:     50000,
		MaxTextLength:       500,
		MaxFilters:          20,
		MaxAttributeFilters: 10,
		MaxSortClauses:      3,
	}
}

func validateSearchQuery(query domain.DiscoveryQuery) error {
	// Page size
	if query.Pagination.PageSize > DefaultQueryLimits().MaxPageSize {
		return fmt.Errorf("page_size exceeds maximum %d", DefaultQueryLimits().MaxPageSize)
	}
	if query.Pagination.PageSize < 1 {
		return fmt.Errorf("page_size must be at least 1")
	}

	// Text length
	if len(query.Text) > DefaultQueryLimits().MaxTextLength {
		return fmt.Errorf("text exceeds maximum length %d", DefaultQueryLimits().MaxTextLength)
	}

	// Radius
	if query.RadiusMeters != nil {
		if *query.RadiusMeters <= 0 {
			return fmt.Errorf("radius must be positive")
		}
		if *query.RadiusMeters > DefaultQueryLimits().MaxRadiusMeters {
			return fmt.Errorf("radius exceeds maximum %f", DefaultQueryLimits().MaxRadiusMeters)
		}
	}

	// Sort validation
	if query.Sort != nil {
		switch query.Sort.Field {
		case domain.SearchSortFieldRelevance,
			domain.SearchSortFieldDistance,
			domain.SearchSortFieldPriceAsc,
			domain.SearchSortFieldPriceDesc,
			domain.SearchSortFieldNewest,
			domain.SearchSortFieldOldest,
			domain.SearchSortFieldDiscount,
			domain.SearchSortFieldAvailability:
			// valid
		default:
			return fmt.Errorf("unsupported sort field: %s", query.Sort.Field)
		}
	}

	// Vertical: the caller checks the catalogue; here only the shape.
	if query.Vertical != "" && !query.Vertical.IsWellFormed() {
		return fmt.Errorf("invalid vertical: %s", query.Vertical)
	}
	if len(query.Place) > domain.MaxSearchPlaceLength {
		return fmt.Errorf("location exceeds maximum length %d", domain.MaxSearchPlaceLength)
	}

	// Listing type validation
	if query.ListingType != nil {
		switch string(*query.ListingType) {
		case string(domain.ListingTypePhysicalProduct),
			string(domain.ListingTypeRealEstate),
			string(domain.ListingTypeVehicle),
			string(domain.ListingTypeFood),
			string(domain.ListingTypeService),
			string(domain.ListingTypeOther):
			// valid
		default:
			return fmt.Errorf("unsupported listing type: %s", *query.ListingType)
		}
	}

	// Attribute operators
	for _, attr := range query.Attributes {
		switch attr.Operator {
		case domain.AttributeOperatorEq,
			domain.AttributeOperatorNeq,
			domain.AttributeOperatorGt,
			domain.AttributeOperatorGte,
			domain.AttributeOperatorLt,
			domain.AttributeOperatorLte,
			domain.AttributeOperatorIn,
			domain.AttributeOperatorContains,
			domain.AttributeOperatorSearch:
			// valid
		default:
			return fmt.Errorf("unsupported attribute operator: %s", attr.Operator)
		}
	}

	// Currency format (simple ISO 4217 check)
	if query.Currency != "" {
		if len(query.Currency) != 3 {
			return fmt.Errorf("currency must be 3 characters")
		}
		for _, c := range query.Currency {
			if c < 'A' || c > 'Z' {
				return fmt.Errorf("currency must be uppercase ISO 4217")
			}
		}
	}

	// Number of filters
	filterCount := 0
	if query.Category != nil {
		filterCount++
	}
	if query.PriceRange != nil {
		filterCount++
	}
	if query.Currency != "" {
		filterCount++
	}
	if query.Availability != nil {
		filterCount++
	}
	if query.SellerID != "" {
		filterCount++
	}
	if query.StoreID != "" {
		filterCount++
	}
	if query.ListingType != nil {
		filterCount++
	}
	if query.Location != nil {
		filterCount++
	}
	filterCount += len(query.Attributes)

	if filterCount > DefaultQueryLimits().MaxFilters {
		return fmt.Errorf("too many filters: %d exceeds maximum %d", filterCount, DefaultQueryLimits().MaxFilters)
	}

	if len(query.Attributes) > DefaultQueryLimits().MaxAttributeFilters {
		return fmt.Errorf("too many attribute filters: %d exceeds maximum %d", len(query.Attributes), DefaultQueryLimits().MaxAttributeFilters)
	}

	return nil
}
