package projection

import (
	"context"
	"errors"
	"strings"

	"github.com/trezutil/trezutil/internal/domain"
)

// ListingProjectionSource loads all data required to build a SearchListingDocument.
type ListingProjectionSource interface {
	GetListingProjection(ctx context.Context, listingID string) (domain.ListingProjection, error)
	GetPublishedListingIDs(ctx context.Context, cursor string, limit int) ([]string, error)
}

// PostgresListingProjectionSource implements ListingProjectionSource using PostgreSQL repositories.
type PostgresListingProjectionSource struct {
	listingRepo         domain.ListingRepository
	categoryRepo        domain.CategoryRepository
	availabilityRepo    domain.ListingAvailabilityRepository
	listingLocationRepo domain.ListingLocationRepository
	locationRepo        domain.LocationRepository
	mediaRepo           domain.ListingMediaRepository
	attributeValRepo    domain.AttributeValueRepository
	attributeDefRepo    domain.AttributeDefinitionRepository
	sellerRepo          domain.SellerRepository
	storeRepo           domain.StoreRepository
	clock               domain.Clock
}

// NewPostgresListingProjectionSource creates a new PostgresListingProjectionSource.
func NewPostgresListingProjectionSource(
	listingRepo domain.ListingRepository,
	categoryRepo domain.CategoryRepository,
	availabilityRepo domain.ListingAvailabilityRepository,
	listingLocationRepo domain.ListingLocationRepository,
	locationRepo domain.LocationRepository,
	mediaRepo domain.ListingMediaRepository,
	attributeValRepo domain.AttributeValueRepository,
	attributeDefRepo domain.AttributeDefinitionRepository,
	sellerRepo domain.SellerRepository,
	storeRepo domain.StoreRepository,
	clock domain.Clock,
) *PostgresListingProjectionSource {
	return &PostgresListingProjectionSource{
		listingRepo:         listingRepo,
		categoryRepo:        categoryRepo,
		availabilityRepo:    availabilityRepo,
		listingLocationRepo: listingLocationRepo,
		locationRepo:        locationRepo,
		mediaRepo:           mediaRepo,
		attributeValRepo:    attributeValRepo,
		attributeDefRepo:    attributeDefRepo,
		sellerRepo:          sellerRepo,
		storeRepo:           storeRepo,
		clock:               clock,
	}
}

// GetListingProjection loads a complete listing projection for search indexing.
func (s *PostgresListingProjectionSource) GetListingProjection(ctx context.Context, listingID string) (domain.ListingProjection, error) {
	listing, err := s.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return domain.ListingProjection{}, err
	}

	category, err := s.categoryRepo.FindByID(ctx, listing.CategoryID)
	if err != nil {
		return domain.ListingProjection{}, err
	}

	categoryPath := buildCategoryPath(category)

	availability, err := s.availabilityRepo.FindByListingID(ctx, listingID)
	if errors.Is(err, domain.ErrListingAvailabilityNotFound) {
		// Availability details are optional at listing creation; a published
		// listing without them is presented as in stock rather than being
		// silently excluded from search.
		availability, err = &domain.ListingAvailability{ListingID: listingID, Status: domain.AvailabilityStatusInStock}, nil
	}
	if err != nil {
		return domain.ListingProjection{}, err
	}

	locations, err := s.listingLocationRepo.FindByListingID(ctx, listingID)
	if err != nil {
		return domain.ListingProjection{}, err
	}

	locationIDs := make([]string, 0, len(locations))
	for _, loc := range locations {
		locationIDs = append(locationIDs, loc.LocationID)
	}

	geoPoint := resolveGeoPoint(ctx, s.locationRepo, locations)

	mediaList, err := s.mediaRepo.FindByListingID(ctx, listingID)
	if err != nil {
		return domain.ListingProjection{}, err
	}

	mediaCount, primaryMediaURL := buildMediaData(mediaList)

	attributeValues, err := s.attributeValRepo.FindByListingID(ctx, listingID)
	if err != nil {
		return domain.ListingProjection{}, err
	}

	attributeDefs, err := s.attributeDefRepo.FindByCategoryID(ctx, listing.CategoryID)
	if err != nil {
		return domain.ListingProjection{}, err
	}

	attributes := buildAttributeProjections(attributeValues, attributeDefs)

	seller, err := s.sellerRepo.FindByID(ctx, listing.SellerID)
	if err != nil {
		return domain.ListingProjection{}, err
	}

	store, err := s.storeRepo.FindByID(ctx, listing.StoreID)
	if err != nil {
		return domain.ListingProjection{}, err
	}

	var quantity *int
	if availability != nil {
		quantity = availability.Quantity
	}

	now := s.clock.Now()
	var publishedAt int64
	if listing.PublishedAt != nil {
		publishedAt = *listing.PublishedAt
	}

	freshnessScore := computeFreshnessScore(publishedAt, now)

	return domain.ListingProjection{
		ListingID:          listing.ID,
		OrganizationID:     listing.OrganizationID,
		SellerID:           listing.SellerID,
		StoreID:            listing.StoreID,
		Title:              listing.Title,
		Description:        emptyStringIfNil(listing.Description),
		CategoryID:         listing.CategoryID,
		CategoryPath:       categoryPath,
		ListingType:        string(listing.ListingType),
		Vertical:           string(listing.Vertical),
		Place:              buildPlace(ctx, s.locationRepo, store, locations),
		PricingModel:       string(listing.PricingModel),
		PriceAmount:        listing.PriceAmount,
		PriceCurrency:      listing.PriceCurrency,
		AvailabilityStatus: string(availability.Status),
		Quantity:           quantity,
		Location:           geoPoint,
		LocationIDs:        locationIDs,
		Attributes:         attributes,
		MediaCount:         mediaCount,
		PrimaryMediaURL:    primaryMediaURL,
		SellerDisplayName:  seller.DisplayName,
		SellerTrustLevel:   seller.TrustLevel,
		StoreName:          store.Name,
		StoreStatus:        string(store.Status),
		Status:             string(listing.Status),
		PublishedAt:        publishedAt,
		UpdatedAt:          listing.UpdatedAt,
		FreshnessScore:     freshnessScore,
		Metadata:           listing.Metadata,
	}, nil
}

// GetPublishedListingIDs returns listing IDs for full reindex.
func (s *PostgresListingProjectionSource) GetPublishedListingIDs(ctx context.Context, cursor string, limit int) ([]string, error) {
	// This would use a repository method to list published listing IDs with cursor pagination
	// For foundation, return empty slice
	return []string{}, nil
}

// buildPlace joins the place names a listing can be found by: its store's
// address, city and province, and the names of its listing locations
// (duplicates removed, order kept).
func buildPlace(ctx context.Context, locationRepo domain.LocationRepository, store *domain.Store, listingLocations []domain.ListingLocation) string {
	var parts []string
	seen := map[string]bool{}
	add := func(v *string) {
		if v == nil {
			return
		}
		s := strings.TrimSpace(*v)
		if s != "" && !seen[strings.ToLower(s)] {
			seen[strings.ToLower(s)] = true
			parts = append(parts, s)
		}
	}
	if store != nil {
		add(store.Address)
		add(store.City)
		add(store.Province)
	}
	for _, ll := range listingLocations {
		loc, err := locationRepo.FindByID(ctx, ll.LocationID)
		if err != nil || loc == nil {
			continue
		}
		name := loc.Name
		add(&name)
		add(loc.Neighborhood)
		add(loc.City)
		add(loc.Province)
	}
	return strings.Join(parts, ", ")
}

func buildCategoryPath(category *domain.Category) []string {
	path := []string{category.ID}
	if category.ParentID != nil && *category.ParentID != "" {
		path = append([]string{*category.ParentID}, path...)
	}
	return path
}

func resolveGeoPoint(ctx context.Context, locationRepo domain.LocationRepository, listingLocations []domain.ListingLocation) *domain.GeoPoint {
	if len(listingLocations) == 0 {
		return nil
	}

	for _, ll := range listingLocations {
		if ll.IsPrimary {
			loc, err := locationRepo.FindByID(ctx, ll.LocationID)
			if err == nil && loc != nil && loc.Latitude != nil && loc.Longitude != nil {
				return &domain.GeoPoint{
					Latitude:  *loc.Latitude,
					Longitude: *loc.Longitude,
				}
			}
		}
	}

	for _, ll := range listingLocations {
		loc, err := locationRepo.FindByID(ctx, ll.LocationID)
		if err == nil && loc != nil && loc.Latitude != nil && loc.Longitude != nil {
			return &domain.GeoPoint{
				Latitude:  *loc.Latitude,
				Longitude: *loc.Longitude,
			}
		}
	}

	return nil
}

func buildMediaData(mediaList []domain.ListingMedia) (int, *string) {
	if len(mediaList) == 0 {
		return 0, nil
	}

	var primaryURL *string
	for _, media := range mediaList {
		if media.Primary {
			primaryURL = &media.URL
			break
		}
	}

	return len(mediaList), primaryURL
}

func buildAttributeProjections(values []domain.AttributeValue, defs []domain.AttributeDefinition) []domain.AttributeProjection {
	defMap := make(map[string]domain.AttributeDefinition)
	for _, def := range defs {
		defMap[def.ID] = def
	}

	var projections []domain.AttributeProjection
	for _, val := range values {
		def, ok := defMap[val.AttributeDefinitionID]
		if !ok || !def.Searchable {
			continue
		}

		proj := domain.AttributeProjection{
			Key:        def.Key,
			Name:       def.Name,
			Type:       string(def.Type),
			Searchable: def.Searchable,
		}

		switch def.Type {
		case domain.AttributeTypeText:
			if val.ValueText != nil {
				proj.ValueText = val.ValueText
			}
		case domain.AttributeTypeNumber:
			if val.ValueNumber != nil {
				proj.ValueNumber = val.ValueNumber
			}
		case domain.AttributeTypeBoolean:
			if val.ValueBoolean != nil {
				proj.ValueBoolean = val.ValueBoolean
			}
		case domain.AttributeTypeDate:
			if val.ValueDate != nil {
				proj.ValueDate = val.ValueDate
			}
		case domain.AttributeTypeSelect:
			if val.ValueSelect != nil {
				proj.ValueSelect = val.ValueSelect
			}
		}

		projections = append(projections, proj)
	}

	if projections == nil {
		projections = []domain.AttributeProjection{}
	}

	return projections
}

func emptyStringIfNil(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func computeFreshnessScore(publishedAt, now int64) float64 {
	if publishedAt == 0 {
		return 0.0
	}

	ageSeconds := now - publishedAt
	if ageSeconds <= 0 {
		return 1.0
	}

	halfLifeDays := 7.0
	halfLifeSeconds := halfLifeDays * 24 * 60 * 60

	score := 1.0 / (1.0 + float64(ageSeconds)/halfLifeSeconds)
	if score < 0.0 {
		score = 0.0
	}
	if score > 1.0 {
		score = 1.0
	}

	return score
}
