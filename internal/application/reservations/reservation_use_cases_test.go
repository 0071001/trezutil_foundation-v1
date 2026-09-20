package reservations

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/domain"
)

func setupReservationTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("postgres", "postgres://trezutil:trezutil@localhost:5432/trezutil_test?sslmode=disable")
	require.NoError(t, err)
	return db
}

func createTestOrg(t *testing.T, db *sql.DB) string {
	orgRepo := postgres.NewOrganizationRepository(db)
	org := &domain.Organization{
		ID:          "org-test-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Type:        domain.OrganizationTypeBusiness,
		LegalName:   "Test Org",
		DisplayName: "Test Org",
		Country:     "US",
		Status:      domain.OrganizationStatusActive,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}
	err := orgRepo.Create(context.Background(), org)
	require.NoError(t, err)
	return org.ID
}

func createTestUser(t *testing.T, db *sql.DB, orgID string, role domain.Role) string {
	userRepo := postgres.NewUserRepository(db)
	user := &domain.User{
		ID:          "user-test-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Phone:       "+15551234567",
		DisplayName: "Test User",
		Status:      domain.UserStatusActive,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
		Roles:       []domain.Role{role},
	}
	err := userRepo.Create(context.Background(), user)
	require.NoError(t, err)
	return user.ID
}

func createTestSeller(t *testing.T, db *sql.DB, orgID, userID string) string {
	sellerRepo := postgres.NewSellerRepository(db)
	seller := &domain.SellerProfile{
		ID:             "seller-test-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		UserID:         &userID,
		OrganizationID: &orgID,
		SellerType:     domain.SellerTypeOrganization,
		DisplayName:    "Test Seller",
		Status:         domain.SellerStatusActive,
		CreatedAt:      time.Now().Unix(),
		UpdatedAt:      time.Now().Unix(),
	}
	err := sellerRepo.Create(context.Background(), seller)
	require.NoError(t, err)
	return seller.ID
}

func createTestListing(t *testing.T, db *sql.DB, orgID, sellerID string) string {
	listingRepo := postgres.NewListingRepository(db)
	listing := &domain.Listing{
		ID:             "listing-test-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		OrganizationID: orgID,
		SellerID:       sellerID,
		Title:          "Test Listing",
		ListingType:    domain.ListingTypeService,
		PricingModel:   domain.PricingModelFixed,
		PriceAmount:    100.0,
		PriceCurrency:  "USD",
		Status:         domain.ListingStatusPublished,
		CreatedAt:      time.Now().Unix(),
		UpdatedAt:      time.Now().Unix(),
	}
	err := listingRepo.Create(context.Background(), listing)
	require.NoError(t, err)
	return listing.ID
}

func TestReservationUseCases_CreateReservation_Success(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	userID := createTestUser(t, db, orgID, domain.RoleCustomer)
	sellerID := createTestSeller(t, db, orgID, userID)
	listingID := createTestListing(t, db, orgID, sellerID)

	reservationRepo := postgres.NewReservationRepository(db)
	listingRepo := postgres.NewListingRepository(db)
	sellerRepo := postgres.NewSellerRepository(db)
	userRepo := postgres.NewUserRepository(db)
	orgRepo := postgres.NewOrganizationRepository(db)
	availabilityProvider := &mockAvailabilityProvider{available: true}

	uc := NewReservationUseCases(
		reservationRepo,
		listingRepo,
		sellerRepo,
		userRepo,
		orgRepo,
		availabilityProvider,
		nil, // auditLogger
		ConfirmationPolicy{RequiresConfirmation: false, AutoConfirm: true},
	)

	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()

	reservation, err := uc.CreateReservation(context.Background(), userID, listingID, startAt, endAt, 1, 2, "")
	require.NoError(t, err)
	assert.NotEmpty(t, reservation.ID)
	assert.Equal(t, domain.ReservationStatusConfirmed, reservation.Status)
	assert.Equal(t, userID, reservation.CustomerID)
	assert.Equal(t, listingID, reservation.ListingID)
	assert.Equal(t, startAt, reservation.StartAt)
	assert.Equal(t, endAt, reservation.EndAt)
	assert.Equal(t, 1, reservation.Quantity)
	assert.Equal(t, 2, reservation.PartySize)
}

func TestReservationUseCases_CreateReservation_Unauthorized(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	userID := createTestUser(t, db, orgID, domain.RoleCustomer) // Customer role, no org membership
	sellerID := createTestSeller(t, db, orgID, userID)
	listingID := createTestListing(t, db, orgID, sellerID)

	reservationRepo := postgres.NewReservationRepository(db)
	listingRepo := postgres.NewListingRepository(db)
	sellerRepo := postgres.NewSellerRepository(db)
	userRepo := postgres.NewUserRepository(db)
	orgRepo := postgres.NewOrganizationRepository(db)
	availabilityProvider := &mockAvailabilityProvider{available: true}

	uc := NewReservationUseCases(
		reservationRepo,
		listingRepo,
		sellerRepo,
		userRepo,
		orgRepo,
		availabilityProvider,
		nil,
		ConfirmationPolicy{RequiresConfirmation: false, AutoConfirm: true},
	)

	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()

	_, err := uc.CreateReservation(context.Background(), userID, listingID, startAt, endAt, 1, 2, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient permissions")
}

func TestReservationUseCases_CreateReservation_Unavailable(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	userID := createTestUser(t, db, orgID, domain.RoleAdmin)
	sellerID := createTestSeller(t, db, orgID, userID)
	listingID := createTestListing(t, db, orgID, sellerID)

	reservationRepo := postgres.NewReservationRepository(db)
	listingRepo := postgres.NewListingRepository(db)
	sellerRepo := postgres.NewSellerRepository(db)
	userRepo := postgres.NewUserRepository(db)
	orgRepo := postgres.NewOrganizationRepository(db)
	availabilityProvider := &mockAvailabilityProvider{available: false}

	uc := NewReservationUseCases(
		reservationRepo,
		listingRepo,
		sellerRepo,
		userRepo,
		orgRepo,
		availabilityProvider,
		nil,
		ConfirmationPolicy{RequiresConfirmation: false, AutoConfirm: true},
	)

	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()

	_, err := uc.CreateReservation(context.Background(), userID, listingID, startAt, endAt, 1, 2, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestReservationUseCases_GetReservation_Success(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	userID := createTestUser(t, db, orgID, domain.RoleAdmin)
	sellerID := createTestSeller(t, db, orgID, userID)
	listingID := createTestListing(t, db, orgID, sellerID)

	reservationRepo := postgres.NewReservationRepository(db)
	listingRepo := postgres.NewListingRepository(db)
	sellerRepo := postgres.NewSellerRepository(db)
	userRepo := postgres.NewUserRepository(db)
	orgRepo := postgres.NewOrganizationRepository(db)
	availabilityProvider := &mockAvailabilityProvider{available: true}

	uc := NewReservationUseCases(
		reservationRepo,
		listingRepo,
		sellerRepo,
		userRepo,
		orgRepo,
		availabilityProvider,
		nil,
		ConfirmationPolicy{RequiresConfirmation: false, AutoConfirm: true},
	)

	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()

	created, err := uc.CreateReservation(context.Background(), userID, listingID, startAt, endAt, 1, 2, "")
	require.NoError(t, err)

	found, err := uc.GetReservation(context.Background(), userID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.Status, found.Status)
}

func TestReservationUseCases_GetReservation_Forbidden(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID1 := createTestOrg(t, db)
	orgID2 := createTestOrg(t, db)
	userID1 := createTestUser(t, db, orgID1, domain.RoleAdmin)
	userID2 := createTestUser(t, db, orgID2, domain.RoleAdmin)
	sellerID := createTestSeller(t, db, orgID1, userID1)
	listingID := createTestListing(t, db, orgID1, sellerID)

	reservationRepo := postgres.NewReservationRepository(db)
	listingRepo := postgres.NewListingRepository(db)
	sellerRepo := postgres.NewSellerRepository(db)
	userRepo := postgres.NewUserRepository(db)
	orgRepo := postgres.NewOrganizationRepository(db)
	availabilityProvider := &mockAvailabilityProvider{available: true}

	uc := NewReservationUseCases(
		reservationRepo,
		listingRepo,
		sellerRepo,
		userRepo,
		orgRepo,
		availabilityProvider,
		nil,
		ConfirmationPolicy{RequiresConfirmation: false, AutoConfirm: true},
	)

	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()

	created, err := uc.CreateReservation(context.Background(), userID1, listingID, startAt, endAt, 1, 2, "")
	require.NoError(t, err)

	// User from different org tries to access
	_, err = uc.GetReservation(context.Background(), userID2, created.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient permissions")
}

func TestReservationUseCases_CancelReservation_Success(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	userID := createTestUser(t, db, orgID, domain.RoleAdmin)
	sellerID := createTestSeller(t, db, orgID, userID)
	listingID := createTestListing(t, db, orgID, sellerID)

	reservationRepo := postgres.NewReservationRepository(db)
	listingRepo := postgres.NewListingRepository(db)
	sellerRepo := postgres.NewSellerRepository(db)
	userRepo := postgres.NewUserRepository(db)
	orgRepo := postgres.NewOrganizationRepository(db)
	availabilityProvider := &mockAvailabilityProvider{available: true}

	uc := NewReservationUseCases(
		reservationRepo,
		listingRepo,
		sellerRepo,
		userRepo,
		orgRepo,
		availabilityProvider,
		nil,
		ConfirmationPolicy{RequiresConfirmation: false, AutoConfirm: true},
	)

	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()

	created, err := uc.CreateReservation(context.Background(), userID, listingID, startAt, endAt, 1, 2, "")
	require.NoError(t, err)

	err = uc.CancelReservation(context.Background(), userID, created.ID)
	require.NoError(t, err)

	found, err := uc.GetReservation(context.Background(), userID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ReservationStatusCancelled, found.Status)
}

func TestReservationUseCases_CancelReservation_AlreadyCancelled(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	userID := createTestUser(t, db, orgID, domain.RoleAdmin)
	sellerID := createTestSeller(t, db, orgID, userID)
	listingID := createTestListing(t, db, orgID, sellerID)

	reservationRepo := postgres.NewReservationRepository(db)
	listingRepo := postgres.NewListingRepository(db)
	sellerRepo := postgres.NewSellerRepository(db)
	userRepo := postgres.NewUserRepository(db)
	orgRepo := postgres.NewOrganizationRepository(db)
	availabilityProvider := &mockAvailabilityProvider{available: true}

	uc := NewReservationUseCases(
		reservationRepo,
		listingRepo,
		sellerRepo,
		userRepo,
		orgRepo,
		availabilityProvider,
		nil,
		ConfirmationPolicy{RequiresConfirmation: false, AutoConfirm: true},
	)

	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()

	created, err := uc.CreateReservation(context.Background(), userID, listingID, startAt, endAt, 1, 2, "")
	require.NoError(t, err)

	err = uc.CancelReservation(context.Background(), userID, created.ID)
	require.NoError(t, err)

	err = uc.CancelReservation(context.Background(), userID, created.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot cancel reservation in terminal status")
}

func TestReservationUseCases_Idempotency(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	userID := createTestUser(t, db, orgID, domain.RoleAdmin)
	sellerID := createTestSeller(t, db, orgID, userID)
	listingID := createTestListing(t, db, orgID, sellerID)

	reservationRepo := postgres.NewReservationRepository(db)
	listingRepo := postgres.NewListingRepository(db)
	sellerRepo := postgres.NewSellerRepository(db)
	userRepo := postgres.NewUserRepository(db)
	orgRepo := postgres.NewOrganizationRepository(db)
	availabilityProvider := &mockAvailabilityProvider{available: true}

	uc := NewReservationUseCases(
		reservationRepo,
		listingRepo,
		sellerRepo,
		userRepo,
		orgRepo,
		availabilityProvider,
		nil,
		ConfirmationPolicy{RequiresConfirmation: false, AutoConfirm: true},
	)

	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()
	idempotencyKey := "idem-test-" + fmt.Sprintf("%d", time.Now().UnixNano())

	reservation1, err := uc.CreateReservation(context.Background(), userID, listingID, startAt, endAt, 1, 2, idempotencyKey)
	require.NoError(t, err)

	reservation2, err := uc.CreateReservation(context.Background(), userID, listingID, startAt, endAt, 1, 2, idempotencyKey)
	require.NoError(t, err)

	assert.Equal(t, reservation1.ID, reservation2.ID)
}

type mockAvailabilityProvider struct {
	available bool
}

func (m *mockAvailabilityProvider) CheckAvailability(ctx context.Context, query domain.AvailabilityQuery) (domain.AvailabilityResult, error) {
	if m.available {
		return domain.AvailabilityResult{
			Available: true,
			Capacity:  10,
			Remaining: 5,
			StartAt:   query.StartAt,
			EndAt:     query.EndAt,
		}, nil
	}
	return domain.AvailabilityResult{
		Available: false,
		Reason:    "not available",
		StartAt:   query.StartAt,
		EndAt:     query.EndAt,
	}, nil
}
