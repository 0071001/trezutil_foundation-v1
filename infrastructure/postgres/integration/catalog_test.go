package integration

import (
	"context"
	"testing"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/domain"
)

// createTestCategory creates a test category.
func createTestCategory(t *testing.T, db *TestDB, ctx context.Context, id string) *domain.Category {
	t.Helper()
	repo := postgres.NewCategoryRepository(db.DB)
	category := domain.NewCategory("Electronics", "electronics", nil)
	category.ID = id
	category.CreatedAt = time.Now().Unix()
	category.UpdatedAt = time.Now().Unix()

	err := repo.Create(ctx, category)
	if err != nil {
		t.Fatalf("failed to create test category: %v", err)
	}

	return category
}

// createTestOrganization creates a test organization.
func createTestOrganization(t *testing.T, db *TestDB, ctx context.Context, id string) *domain.Organization {
	t.Helper()
	repo := postgres.NewOrganizationRepository(db.DB)
	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "Test Org", "Test Organization", "CD", "+243812345678")
	org.ID = id
	org.CreatedAt = time.Now().Unix()
	org.UpdatedAt = time.Now().Unix()

	err := repo.Create(ctx, org)
	if err != nil {
		t.Fatalf("failed to create test organization: %v", err)
	}

	return org
}

// createTestUser creates a test user.
func createTestUser(t *testing.T, db *TestDB, ctx context.Context, id string) *domain.User {
	t.Helper()
	repo := postgres.NewUserRepository(db.DB)
	user := &domain.User{
		ID:          id,
		Phone:       "+243812345678",
		DisplayName: "Test User",
		Status:      domain.UserStatusActive,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
		Roles:       []domain.Role{},
	}

	err := repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	return user
}

// createTestSeller creates a test seller.
func createTestSeller(t *testing.T, db *TestDB, ctx context.Context, id string, userID string, orgID string) *domain.SellerProfile {
	t.Helper()
	repo := postgres.NewSellerRepository(db.DB)
	seller := domain.NewSellerProfile(domain.SellerTypeIndividual, "Test Seller", "+243812345678", &userID, &orgID)
	seller.ID = id
	seller.CreatedAt = time.Now().Unix()
	seller.UpdatedAt = time.Now().Unix()

	err := repo.Create(ctx, seller)
	if err != nil {
		t.Fatalf("failed to create test seller: %v", err)
	}

	return seller
}

// createTestStore creates a test store.
func createTestStore(t *testing.T, db *TestDB, ctx context.Context, id string, orgID string) *domain.Store {
	t.Helper()
	repo := postgres.NewStoreRepository(db.DB)
	store := domain.NewStore(orgID, "Test Store", "test-store", "CD", "+243812345678")
	store.ID = id
	store.CreatedAt = time.Now().Unix()
	store.UpdatedAt = time.Now().Unix()

	err := repo.Create(ctx, store)
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}

	return store
}

// createTestListing creates a test listing with all prerequisites.
func createTestListing(t *testing.T, db *TestDB, ctx context.Context, listingID string, orgID string, sellerID string, storeID string, categoryID string) *domain.Listing {
	t.Helper()
	listingRepo := postgres.NewListingRepository(db.DB)

	listing := domain.NewListing(orgID, sellerID, storeID, categoryID, "Test Listing", string(domain.ListingTypePhysicalProduct), string(domain.PricingModelFixed), "USD", 100.0)
	listing.ID = listingID
	listing.CreatedAt = time.Now().Unix()
	listing.UpdatedAt = time.Now().Unix()

	err := listingRepo.Create(ctx, listing)
	if err != nil {
		t.Fatalf("failed to create test listing: %v", err)
	}

	return listing
}

func TestCategoryRepository_Create(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	repo := postgres.NewCategoryRepository(db.DB)
	ctx := context.Background()

	category := domain.NewCategory("Electronics", "electronics", nil)
	category.ID = "123e4567-e89b-12d3-a456-426614174001"
	category.CreatedAt = time.Now().Unix()
	category.UpdatedAt = time.Now().Unix()

	err := repo.Create(ctx, category)
	if err != nil {
		t.Fatalf("failed to create category: %v", err)
	}

	found, err := repo.FindByID(ctx, "123e4567-e89b-12d3-a456-426614174001")
	if err != nil {
		t.Fatalf("failed to find category: %v", err)
	}

	if found.Name != "Electronics" {
		t.Errorf("Expected Name 'Electronics', got '%s'", found.Name)
	}
	if found.Slug != "electronics" {
		t.Errorf("Expected Slug 'electronics', got '%s'", found.Slug)
	}
	if found.Status != domain.CategoryStatusActive {
		t.Errorf("Expected Status 'active', got '%s'", found.Status)
	}
}

func TestCategoryRepository_FindBySlug(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	repo := postgres.NewCategoryRepository(db.DB)
	ctx := context.Background()

	category := domain.NewCategory("Electronics", "electronics", nil)
	category.ID = "123e4567-e89b-12d3-a456-426614174001"
	category.CreatedAt = time.Now().Unix()
	category.UpdatedAt = time.Now().Unix()

	err := repo.Create(ctx, category)
	if err != nil {
		t.Fatalf("failed to create category: %v", err)
	}

	found, err := repo.FindBySlug(ctx, "electronics")
	if err != nil {
		t.Fatalf("failed to find category by slug: %v", err)
	}

	if found.Name != "Electronics" {
		t.Errorf("Expected Name 'Electronics', got '%s'", found.Name)
	}
}

func TestCategoryRepository_Hierarchy(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	repo := postgres.NewCategoryRepository(db.DB)
	ctx := context.Background()

	parentID := "123e4567-e89b-12d3-a456-426614174002"
	parent := domain.NewCategory("Electronics", "electronics", nil)
	parent.ID = parentID
	parent.CreatedAt = time.Now().Unix()
	parent.UpdatedAt = time.Now().Unix()

	err := repo.Create(ctx, parent)
	if err != nil {
		t.Fatalf("failed to create parent category: %v", err)
	}

	childID := "123e4567-e89b-12d3-a456-426614174003"
	child := domain.NewCategory("Smartphones", "smartphones", &parentID)
	child.ID = childID
	child.CreatedAt = time.Now().Unix()
	child.UpdatedAt = time.Now().Unix()

	err = repo.Create(ctx, child)
	if err != nil {
		t.Fatalf("failed to create child category: %v", err)
	}

	children, err := repo.FindByParentID(ctx, parentID)
	if err != nil {
		t.Fatalf("failed to find children: %v", err)
	}

	if len(children) != 1 {
		t.Errorf("Expected 1 child, got %d", len(children))
	}

	if children[0].Name != "Smartphones" {
		t.Errorf("Expected child Name 'Smartphones', got '%s'", children[0].Name)
	}
}

func TestListingRepository_Create(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	orgID := "123e4567-e89b-12d3-a456-426614174020"
	userID := "123e4567-e89b-12d3-a456-426614174023"
	sellerID := "123e4567-e89b-12d3-a456-426614174021"
	storeID := "123e4567-e89b-12d3-a456-426614174022"
	categoryID := "123e4567-e89b-12d3-a456-426614174001"

	createTestOrganization(t, db, ctx, orgID)
	createTestUser(t, db, ctx, userID)
	createTestSeller(t, db, ctx, sellerID, userID, orgID)
	createTestStore(t, db, ctx, storeID, orgID)
	createTestCategory(t, db, ctx, categoryID)

	listingRepo := postgres.NewListingRepository(db.DB)

	listing := domain.NewListing(orgID, sellerID, storeID, categoryID, "Test Listing", string(domain.ListingTypePhysicalProduct), string(domain.PricingModelFixed), "USD", 100.0)
	listing.ID = "123e4567-e89b-12d3-a456-426614174004"
	listing.CreatedAt = time.Now().Unix()
	listing.UpdatedAt = time.Now().Unix()

	err := listingRepo.Create(ctx, listing)
	if err != nil {
		t.Fatalf("failed to create listing: %v", err)
	}

	found, err := listingRepo.FindByID(ctx, "123e4567-e89b-12d3-a456-426614174004")
	if err != nil {
		t.Fatalf("failed to find listing: %v", err)
	}

	if found.Title != "Test Listing" {
		t.Errorf("Expected Title 'Test Listing', got '%s'", found.Title)
	}
	if found.PriceAmount != 100.0 {
		t.Errorf("Expected PriceAmount 100.0, got %f", found.PriceAmount)
	}
}

func TestListingRepository_FindByOrganizationID(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	orgID := "123e4567-e89b-12d3-a456-426614174020"
	userID := "123e4567-e89b-12d3-a456-426614174023"
	sellerID := "123e4567-e89b-12d3-a456-426614174021"
	storeID := "123e4567-e89b-12d3-a456-426614174022"
	categoryID := "123e4567-e89b-12d3-a456-426614174001"

	createTestOrganization(t, db, ctx, orgID)
	createTestUser(t, db, ctx, userID)
	createTestSeller(t, db, ctx, sellerID, userID, orgID)
	createTestStore(t, db, ctx, storeID, orgID)
	createTestCategory(t, db, ctx, categoryID)

	listingRepo := postgres.NewListingRepository(db.DB)

	listing1 := domain.NewListing(orgID, sellerID, storeID, categoryID, "Listing 1", string(domain.ListingTypePhysicalProduct), string(domain.PricingModelFixed), "USD", 100.0)
	listing1.ID = "123e4567-e89b-12d3-a456-426614174005"
	listing1.CreatedAt = time.Now().Unix()
	listing1.UpdatedAt = time.Now().Unix()

	listing2 := domain.NewListing(orgID, sellerID, storeID, categoryID, "Listing 2", string(domain.ListingTypePhysicalProduct), string(domain.PricingModelFixed), "USD", 200.0)
	listing2.ID = "123e4567-e89b-12d3-a456-426614174006"
	listing2.CreatedAt = time.Now().Unix()
	listing2.UpdatedAt = time.Now().Unix()

	err := listingRepo.Create(ctx, listing1)
	if err != nil {
		t.Fatalf("failed to create listing 1: %v", err)
	}
	err = listingRepo.Create(ctx, listing2)
	if err != nil {
		t.Fatalf("failed to create listing 2: %v", err)
	}

	listings, err := listingRepo.FindByOrganizationID(ctx, orgID, 0, 100)
	if err != nil {
		t.Fatalf("failed to find listings by org: %v", err)
	}

	if len(listings) != 2 {
		t.Errorf("Expected 2 listings, got %d", len(listings))
	}
}

func TestListingRepository_StatusTransitions(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	orgID := "123e4567-e89b-12d3-a456-426614174020"
	userID := "123e4567-e89b-12d3-a456-426614174023"
	sellerID := "123e4567-e89b-12d3-a456-426614174021"
	storeID := "123e4567-e89b-12d3-a456-426614174022"
	categoryID := "123e4567-e89b-12d3-a456-426614174001"
	listingID := "123e4567-e89b-12d3-a456-426614174007"

	createTestOrganization(t, db, ctx, orgID)
	createTestUser(t, db, ctx, userID)
	createTestSeller(t, db, ctx, sellerID, userID, orgID)
	createTestStore(t, db, ctx, storeID, orgID)
	createTestCategory(t, db, ctx, categoryID)
	listing := createTestListing(t, db, ctx, listingID, orgID, sellerID, storeID, categoryID)

	listingRepo := postgres.NewListingRepository(db.DB)

	// Test publish
	listing.Publish()
	err := listingRepo.Update(ctx, listing)
	if err != nil {
		t.Fatalf("failed to update listing: %v", err)
	}

	found, err := listingRepo.FindByID(ctx, listingID)
	if err != nil {
		t.Fatalf("failed to find listing: %v", err)
	}

	if found.Status != domain.ListingStatusPublished {
		t.Errorf("Expected Status 'published', got '%s'", found.Status)
	}
}

func TestAttributeDefinitionRepository_Create(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	categoryID := "123e4567-e89b-12d3-a456-426614174001"

	createTestCategory(t, db, ctx, categoryID)

	repo := postgres.NewAttributeDefinitionRepository(db.DB)

	attr := domain.NewAttributeDefinition(categoryID, "Brand", "brand", domain.AttributeTypeText)
	attr.ID = "123e4567-e89b-12d3-a456-426614174010"
	attr.CreatedAt = time.Now().Unix()
	attr.UpdatedAt = time.Now().Unix()

	err := repo.Create(ctx, attr)
	if err != nil {
		t.Fatalf("failed to create attribute definition: %v", err)
	}

	found, err := repo.FindByID(ctx, "123e4567-e89b-12d3-a456-426614174010")
	if err != nil {
		t.Fatalf("failed to find attribute definition: %v", err)
	}

	if found.Name != "Brand" {
		t.Errorf("Expected Name 'Brand', got '%s'", found.Name)
	}
	if found.Key != "brand" {
		t.Errorf("Expected Key 'brand', got '%s'", found.Key)
	}
}

func TestAttributeValueRepository_Create(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	orgID := "123e4567-e89b-12d3-a456-426614174020"
	userID := "123e4567-e89b-12d3-a456-426614174023"
	sellerID := "123e4567-e89b-12d3-a456-426614174021"
	storeID := "123e4567-e89b-12d3-a456-426614174022"
	categoryID := "123e4567-e89b-12d3-a456-426614174001"
	listingID := "123e4567-e89b-12d3-a456-426614174008"

	createTestOrganization(t, db, ctx, orgID)
	createTestUser(t, db, ctx, userID)
	createTestSeller(t, db, ctx, sellerID, userID, orgID)
	createTestStore(t, db, ctx, storeID, orgID)
	createTestCategory(t, db, ctx, categoryID)
	createTestListing(t, db, ctx, listingID, orgID, sellerID, storeID, categoryID)

	attrDefRepo := postgres.NewAttributeDefinitionRepository(db.DB)
	attrDef := domain.NewAttributeDefinition(categoryID, "Brand", "brand", domain.AttributeTypeText)
	attrDef.ID = "123e4567-e89b-12d3-a456-426614174024"
	attrDef.CreatedAt = time.Now().Unix()
	attrDef.UpdatedAt = time.Now().Unix()
	err := attrDefRepo.Create(ctx, attrDef)
	if err != nil {
		t.Fatalf("failed to create attribute definition: %v", err)
	}

	valueRepo := postgres.NewAttributeValueRepository(db.DB)

	value := domain.NewAttributeValue(listingID, "123e4567-e89b-12d3-a456-426614174024")
	value.ID = "123e4567-e89b-12d3-a456-426614174011"
	value.ValueText = strPtr("Samsung")
	value.CreatedAt = time.Now().Unix()
	value.UpdatedAt = time.Now().Unix()

	err = valueRepo.Create(ctx, value)
	if err != nil {
		t.Fatalf("failed to create attribute value: %v", err)
	}

	values, err := valueRepo.FindByListingID(ctx, listingID)
	if err != nil {
		t.Fatalf("failed to find attribute values: %v", err)
	}

	if len(values) != 1 {
		t.Errorf("Expected 1 value, got %d", len(values))
	}

	if values[0].ValueText == nil || *values[0].ValueText != "Samsung" {
		t.Errorf("Expected ValueText 'Samsung', got %v", values[0].ValueText)
	}
}

func TestListingMediaRepository_Create(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	orgID := "123e4567-e89b-12d3-a456-426614174020"
	userID := "123e4567-e89b-12d3-a456-426614174023"
	sellerID := "123e4567-e89b-12d3-a456-426614174021"
	storeID := "123e4567-e89b-12d3-a456-426614174022"
	categoryID := "123e4567-e89b-12d3-a456-426614174001"
	listingID := "123e4567-e89b-12d3-a456-426614174009"

	createTestOrganization(t, db, ctx, orgID)
	createTestUser(t, db, ctx, userID)
	createTestSeller(t, db, ctx, sellerID, userID, orgID)
	createTestStore(t, db, ctx, storeID, orgID)
	createTestCategory(t, db, ctx, categoryID)
	createTestListing(t, db, ctx, listingID, orgID, sellerID, storeID, categoryID)

	mediaRepo := postgres.NewListingMediaRepository(db.DB)

	media := domain.NewListingMedia(listingID, domain.MediaTypeImage, "https://example.com/image.jpg")
	media.ID = "123e4567-e89b-12d3-a456-426614174012"
	media.Primary = true
	media.CreatedAt = time.Now().Unix()
	media.UpdatedAt = time.Now().Unix()

	err := mediaRepo.Create(ctx, media)
	if err != nil {
		t.Fatalf("failed to create media: %v", err)
	}

	mediaList, err := mediaRepo.FindByListingID(ctx, listingID)
	if err != nil {
		t.Fatalf("failed to find media: %v", err)
	}

	if len(mediaList) != 1 {
		t.Errorf("Expected 1 media, got %d", len(mediaList))
	}

	if mediaList[0].URL != "https://example.com/image.jpg" {
		t.Errorf("Expected URL 'https://example.com/image.jpg', got '%s'", mediaList[0].URL)
	}
}

func TestListingAvailabilityRepository_Create(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	orgID := "123e4567-e89b-12d3-a456-426614174020"
	userID := "123e4567-e89b-12d3-a456-426614174023"
	sellerID := "123e4567-e89b-12d3-a456-426614174021"
	storeID := "123e4567-e89b-12d3-a456-426614174022"
	categoryID := "123e4567-e89b-12d3-a456-426614174001"
	listingID := "123e4567-e89b-12d3-a456-426614174010"

	createTestOrganization(t, db, ctx, orgID)
	createTestUser(t, db, ctx, userID)
	createTestSeller(t, db, ctx, sellerID, userID, orgID)
	createTestStore(t, db, ctx, storeID, orgID)
	createTestCategory(t, db, ctx, categoryID)
	createTestListing(t, db, ctx, listingID, orgID, sellerID, storeID, categoryID)

	availabilityRepo := postgres.NewListingAvailabilityRepository(db.DB)

	qty := 10
	availability := domain.NewListingAvailability(listingID, domain.AvailabilityStatusInStock)
	availability.ID = "123e4567-e89b-12d3-a456-426614174013"
	availability.Quantity = &qty
	availability.CreatedAt = time.Now().Unix()
	availability.UpdatedAt = time.Now().Unix()

	err := availabilityRepo.Create(ctx, availability)
	if err != nil {
		t.Fatalf("failed to create availability: %v", err)
	}

	found, err := availabilityRepo.FindByListingID(ctx, listingID)
	if err != nil {
		t.Fatalf("failed to find availability: %v", err)
	}

	if found.Status != domain.AvailabilityStatusInStock {
		t.Errorf("Expected Status 'in_stock', got '%s'", found.Status)
	}
	if found.Quantity == nil || *found.Quantity != 10 {
		t.Errorf("Expected Quantity 10, got %v", found.Quantity)
	}
}

func TestLocationRepository_Create(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	repo := postgres.NewLocationRepository(db.DB)
	ctx := context.Background()

	location := domain.NewLocation("Kinshasa", domain.LocationTypeCity, "CD")
	location.ID = "123e4567-e89b-12d3-a456-426614174014"
	location.CreatedAt = time.Now().Unix()
	location.UpdatedAt = time.Now().Unix()

	err := repo.Create(ctx, location)
	if err != nil {
		t.Fatalf("failed to create location: %v", err)
	}

	found, err := repo.FindByID(ctx, "123e4567-e89b-12d3-a456-426614174014")
	if err != nil {
		t.Fatalf("failed to find location: %v", err)
	}

	if found.Name != "Kinshasa" {
		t.Errorf("Expected Name 'Kinshasa', got '%s'", found.Name)
	}
	if found.Type != domain.LocationTypeCity {
		t.Errorf("Expected Type 'city', got '%s'", found.Type)
	}
	if found.Country != "CD" {
		t.Errorf("Expected Country 'CD', got '%s'", found.Country)
	}
}

func TestListingLocationRepository_Create(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	orgID := "123e4567-e89b-12d3-a456-426614174020"
	userID := "123e4567-e89b-12d3-a456-426614174023"
	sellerID := "123e4567-e89b-12d3-a456-426614174021"
	storeID := "123e4567-e89b-12d3-a456-426614174022"
	categoryID := "123e4567-e89b-12d3-a456-426614174001"
	listingID := "123e4567-e89b-12d3-a456-426614174011"
	locationID := "123e4567-e89b-12d3-a456-426614174015"

	createTestOrganization(t, db, ctx, orgID)
	createTestUser(t, db, ctx, userID)
	createTestSeller(t, db, ctx, sellerID, userID, orgID)
	createTestStore(t, db, ctx, storeID, orgID)
	createTestCategory(t, db, ctx, categoryID)
	createTestListing(t, db, ctx, listingID, orgID, sellerID, storeID, categoryID)

	locationRepo := postgres.NewLocationRepository(db.DB)
	llRepo := postgres.NewListingLocationRepository(db.DB)

	location := domain.NewLocation("Kinshasa", domain.LocationTypeCity, "CD")
	location.ID = locationID
	location.CreatedAt = time.Now().Unix()
	location.UpdatedAt = time.Now().Unix()

	err := locationRepo.Create(ctx, location)
	if err != nil {
		t.Fatalf("failed to create location: %v", err)
	}

	ll := domain.NewListingLocation(listingID, locationID)
	ll.ID = "123e4567-e89b-12d3-a456-426614174016"
	ll.IsPrimary = true
	ll.CreatedAt = time.Now().Unix()
	ll.UpdatedAt = time.Now().Unix()

	err = llRepo.Create(ctx, ll)
	if err != nil {
		t.Fatalf("failed to create listing location: %v", err)
	}

	locations, err := llRepo.FindByListingID(ctx, listingID)
	if err != nil {
		t.Fatalf("failed to find listing locations: %v", err)
	}

	if len(locations) != 1 {
		t.Errorf("Expected 1 location, got %d", len(locations))
	}

	if !locations[0].IsPrimary {
		t.Error("Expected IsPrimary to be true")
	}
}

func TestUserLocationRepository_Create(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	userID := "123e4567-e89b-12d3-a456-426614174017"
	locationID := "123e4567-e89b-12d3-a456-426614174018"

	createTestUser(t, db, ctx, userID)

	locationRepo := postgres.NewLocationRepository(db.DB)
	repo := postgres.NewUserLocationRepository(db.DB)

	location := domain.NewLocation("Kinshasa", domain.LocationTypeCity, "CD")
	location.ID = locationID
	location.CreatedAt = time.Now().Unix()
	location.UpdatedAt = time.Now().Unix()

	err := locationRepo.Create(ctx, location)
	if err != nil {
		t.Fatalf("failed to create location: %v", err)
	}

	ul := domain.NewUserLocation(userID, locationID)
	ul.ID = "123e4567-e89b-12d3-a456-426614174019"
	ul.IsDefault = true
	ul.CreatedAt = time.Now().Unix()
	ul.UpdatedAt = time.Now().Unix()

	err = repo.Create(ctx, ul)
	if err != nil {
		t.Fatalf("failed to create user location: %v", err)
	}

	locations, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("failed to find user locations: %v", err)
	}

	if len(locations) != 1 {
		t.Errorf("Expected 1 location, got %d", len(locations))
	}

	if !locations[0].IsDefault {
		t.Error("Expected IsDefault to be true")
	}
}
