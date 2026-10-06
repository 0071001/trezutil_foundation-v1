package reservations

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/domain"
)

const reservationTestDBName = "trezutil_reservation_test"

func testUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func setupReservationTestDB(t *testing.T) *sql.DB {
	t.Helper()

	adminDSN := "host=localhost port=5432 user=trezutil password=trezutil dbname=postgres sslmode=disable"
	adminDB, err := sql.Open("postgres", adminDSN)
	require.NoError(t, err)
	defer adminDB.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	adminDB.ExecContext(ctx, fmt.Sprintf("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '%s' AND pid <> pg_backend_pid()", reservationTestDBName))
	adminDB.ExecContext(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", reservationTestDBName))
	if _, err := adminDB.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", reservationTestDBName)); err != nil {
		t.Fatalf("failed to create test database: %v", err)
	}

	testDSN := fmt.Sprintf("host=localhost port=5432 user=trezutil password=trezutil dbname=%s sslmode=disable", reservationTestDBName)
	db, err := sql.Open("postgres", testDSN)
	require.NoError(t, err)

	require.NoError(t, applyReservationMigrations(ctx, db))

	return db
}

func applyReservationMigrations(ctx context.Context, db *sql.DB) error {
	migrationsDir := "../../../migrations"
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", file, err)
		}
		upSQL := extractReservationUpSection(string(sqlBytes))
		if _, err := db.ExecContext(ctx, upSQL); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", file, err)
		}
	}
	return nil
}

func extractReservationUpSection(content string) string {
	upMarker := "-- +goose Up"
	downMarker := "-- +goose Down"
	upIdx := strings.Index(content, upMarker)
	downIdx := strings.Index(content, downMarker)
	if upIdx == -1 {
		return content
	}
	start := upIdx + len(upMarker)
	if downIdx == -1 {
		return strings.TrimSpace(content[start:])
	}
	return strings.TrimSpace(content[start:downIdx])
}

func createTestOrg(t *testing.T, db *sql.DB) string {
	orgRepo := postgres.NewOrganizationRepository(db)
	org := &domain.Organization{
		ID:          testUUID(),
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

// uniqueTestPhone returns a distinct E.164 number (phones are unique per user).
func uniqueTestPhone() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("+1555%07d", (uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]))%10000000)
}

func createTestUser(t *testing.T, db *sql.DB, orgID string, role domain.Role) string {
	userRepo := postgres.NewUserRepository(db)
	user := &domain.User{
		ID:          testUUID(),
		Phone:       uniqueTestPhone(),
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

// grantPlatformRole gives the user a platform role (createTestUser only fills the struct: roles are not stored by Create).
func grantPlatformRole(t *testing.T, db *sql.DB, userID string, role domain.Role) string {
	t.Helper()
	require.NoError(t, postgres.NewUserRepository(db).AssignRole(context.Background(), userID, role))
	return userID
}

func createTestSeller(t *testing.T, db *sql.DB, orgID, userID string) string {
	sellerRepo := postgres.NewSellerRepository(db)
	seller := &domain.SellerProfile{
		ID:             testUUID(),
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

func createTestStore(t *testing.T, db *sql.DB, orgID string) string {
	storeRepo := postgres.NewStoreRepository(db)
	store := &domain.Store{
		ID:             testUUID(),
		OrganizationID: orgID,
		Name:           "Test Store",
		Slug:           "test-store-" + testUUID()[:8],
		Country:        "US",
		Phone:          "+15551234567",
		Status:         domain.StoreStatusActive,
		CreatedAt:      time.Now().Unix(),
		UpdatedAt:      time.Now().Unix(),
	}
	err := storeRepo.Create(context.Background(), store)
	require.NoError(t, err)
	return store.ID
}

func createTestCategory(t *testing.T, db *sql.DB) string {
	categoryRepo := postgres.NewCategoryRepository(db)
	category := &domain.Category{
		ID:        testUUID(),
		Name:      "Test Category",
		Slug:      "test-category-" + testUUID()[:8],
		Status:    domain.CategoryStatusActive,
		Scope:     domain.CategoryScopeGlobal,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	err := categoryRepo.Create(context.Background(), category)
	require.NoError(t, err)
	return category.ID
}

func createTestListing(t *testing.T, db *sql.DB, orgID, sellerID, storeID, categoryID string) string {
	listingRepo := postgres.NewListingRepository(db)
	listing := &domain.Listing{
		ID:             testUUID(),
		OrganizationID: orgID,
		SellerID:       sellerID,
		StoreID:        storeID,
		CategoryID:     categoryID,
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

func newTestUseCases(db *sql.DB) *ReservationUseCasesImpl {
	return NewReservationUseCases(
		postgres.NewReservationRepository(db),
		postgres.NewListingRepository(db),
		postgres.NewSellerRepository(db),
		postgres.NewUserRepository(db),
		postgres.NewOrganizationRepository(db),
		&mockAvailabilityProvider{available: true},
		&fakeCapacityRepo{},
		&fakeInventoryRepo{},
		&fakeFulfillmentRepo{},
		&fakeAuditLogger{},
		ConfirmationPolicy{RequiresConfirmation: false, AutoConfirm: true},
	)
}

func TestReservationUseCases_CreateReservation_Success(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	userID := createTestUser(t, db, orgID, domain.RoleAdmin)
	sellerID := createTestSeller(t, db, orgID, userID)
	storeID := createTestStore(t, db, orgID)
	categoryID := createTestCategory(t, db)
	listingID := createTestListing(t, db, orgID, sellerID, storeID, categoryID)

	uc := newTestUseCases(db)
	ctx := context.Background()

	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()

	reservation, err := uc.CreateReservation(ctx, userID, listingID, startAt, endAt, 1, 2, "")
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

func TestReservationUseCases_CreateReservation_CustomerCanBook(t *testing.T) {
	// Approved buyer rule: a plain active customer (no org membership, not a
	// seller) may book any published listing.
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	customerID := createTestUser(t, db, orgID, domain.RoleCustomer)
	sellerUserID := createTestUser(t, db, orgID, domain.RoleAdmin)
	sellerID := createTestSeller(t, db, orgID, sellerUserID)
	storeID := createTestStore(t, db, orgID)
	categoryID := createTestCategory(t, db)
	listingID := createTestListing(t, db, orgID, sellerID, storeID, categoryID)

	uc := newTestUseCases(db)
	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()

	reservation, err := uc.CreateReservation(context.Background(), customerID, listingID, startAt, endAt, 1, 2, "")
	require.NoError(t, err)
	assert.Equal(t, customerID, reservation.CustomerID)
}

func TestReservationUseCases_CreateReservation_SuspendedUserDenied(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	customerID := createTestUser(t, db, orgID, domain.RoleCustomer)
	_, err := db.Exec(`UPDATE users SET status = 'suspended' WHERE id = $1`, customerID)
	require.NoError(t, err)
	sellerUserID := createTestUser(t, db, orgID, domain.RoleAdmin)
	sellerID := createTestSeller(t, db, orgID, sellerUserID)
	storeID := createTestStore(t, db, orgID)
	categoryID := createTestCategory(t, db)
	listingID := createTestListing(t, db, orgID, sellerID, storeID, categoryID)

	uc := newTestUseCases(db)
	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()

	_, err = uc.CreateReservation(context.Background(), customerID, listingID, startAt, endAt, 1, 2, "")
	assert.ErrorIs(t, err, domain.ErrInsufficientPermissions)
}

func TestReservationUseCases_CreateReservation_UnpublishedListingRejected(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	customerID := createTestUser(t, db, orgID, domain.RoleCustomer)
	sellerUserID := createTestUser(t, db, orgID, domain.RoleAdmin)
	sellerID := createTestSeller(t, db, orgID, sellerUserID)
	storeID := createTestStore(t, db, orgID)
	categoryID := createTestCategory(t, db)
	listingID := createTestListing(t, db, orgID, sellerID, storeID, categoryID)
	_, err := db.Exec(`UPDATE listings SET status = 'paused' WHERE id = $1`, listingID)
	require.NoError(t, err)

	uc := newTestUseCases(db)
	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()

	_, err = uc.CreateReservation(context.Background(), customerID, listingID, startAt, endAt, 1, 2, "")
	assert.ErrorIs(t, err, domain.ErrListingUnavailable)
}

func TestReservationUseCases_OverlapRejectedAndIdempotencyScoped(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	alice := createTestUser(t, db, orgID, domain.RoleCustomer)
	bob := createTestUser(t, db, orgID, domain.RoleCustomer)
	sellerUserID := createTestUser(t, db, orgID, domain.RoleAdmin)
	sellerID := createTestSeller(t, db, orgID, sellerUserID)
	storeID := createTestStore(t, db, orgID)
	categoryID := createTestCategory(t, db)
	listingID := createTestListing(t, db, orgID, sellerID, storeID, categoryID)

	uc := newTestUseCases(db)
	ctx := context.Background()
	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()

	first, err := uc.CreateReservation(ctx, alice, listingID, startAt, endAt, 1, 2, "alice-key")
	require.NoError(t, err)

	// Replay by the same customer returns the same reservation even though the
	// slot is now taken (idempotency resolved before availability).
	replay, err := uc.CreateReservation(ctx, alice, listingID, startAt, endAt, 1, 2, "alice-key")
	require.NoError(t, err)
	assert.Equal(t, first.ID, replay.ID)

	// Another customer reusing the key must not see Alice's reservation.
	_, err = uc.CreateReservation(ctx, bob, listingID, startAt, endAt, 1, 2, "alice-key")
	assert.ErrorIs(t, err, domain.ErrIdempotencyConflict)

	// Overlapping booking is rejected by the DB exclusion constraint.
	_, err = uc.CreateReservation(ctx, bob, listingID, startAt+3600, endAt+3600, 1, 2, "bob-key")
	assert.ErrorIs(t, err, domain.ErrReservationConflict)

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM reservations WHERE listing_id = $1`, listingID).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestReservationUseCases_CreateReservation_Unavailable(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	userID := createTestUser(t, db, orgID, domain.RoleAdmin)
	sellerID := createTestSeller(t, db, orgID, userID)
	storeID := createTestStore(t, db, orgID)
	categoryID := createTestCategory(t, db)
	listingID := createTestListing(t, db, orgID, sellerID, storeID, categoryID)

	uc := newTestUseCases(db)
	uc.provider = &mockAvailabilityProvider{available: false}
	ctx := context.Background()

	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()

	_, err := uc.CreateReservation(ctx, userID, listingID, startAt, endAt, 1, 2, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unavailable")
}

func TestReservationUseCases_GetReservation_Success(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	userID := createTestUser(t, db, orgID, domain.RoleAdmin)
	sellerID := createTestSeller(t, db, orgID, userID)
	storeID := createTestStore(t, db, orgID)
	categoryID := createTestCategory(t, db)
	listingID := createTestListing(t, db, orgID, sellerID, storeID, categoryID)

	uc := newTestUseCases(db)
	ctx := context.Background()

	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()

	created, err := uc.CreateReservation(ctx, userID, listingID, startAt, endAt, 1, 2, "")
	require.NoError(t, err)

	found, err := uc.GetReservation(ctx, userID, created.ID)
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
	storeID := createTestStore(t, db, orgID1)
	categoryID := createTestCategory(t, db)
	listingID := createTestListing(t, db, orgID1, sellerID, storeID, categoryID)

	uc := newTestUseCases(db)
	ctx := context.Background()

	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()

	created, err := uc.CreateReservation(ctx, userID1, listingID, startAt, endAt, 1, 2, "")
	require.NoError(t, err)

	_, err = uc.GetReservation(ctx, userID2, created.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient permissions")
}

func TestReservationUseCases_CancelReservation_Success(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	userID := createTestUser(t, db, orgID, domain.RoleAdmin)
	sellerID := createTestSeller(t, db, orgID, userID)
	storeID := createTestStore(t, db, orgID)
	categoryID := createTestCategory(t, db)
	listingID := createTestListing(t, db, orgID, sellerID, storeID, categoryID)

	uc := newTestUseCases(db)
	ctx := context.Background()

	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()

	created, err := uc.CreateReservation(ctx, userID, listingID, startAt, endAt, 1, 2, "")
	require.NoError(t, err)

	err = uc.CancelReservation(ctx, userID, created.ID)
	require.NoError(t, err)

	found, err := uc.GetReservation(ctx, userID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ReservationStatusCancelled, found.Status)
}

func TestReservationUseCases_CancelReservation_AlreadyCancelled(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	userID := createTestUser(t, db, orgID, domain.RoleAdmin)
	sellerID := createTestSeller(t, db, orgID, userID)
	storeID := createTestStore(t, db, orgID)
	categoryID := createTestCategory(t, db)
	listingID := createTestListing(t, db, orgID, sellerID, storeID, categoryID)

	uc := newTestUseCases(db)
	ctx := context.Background()

	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()

	created, err := uc.CreateReservation(ctx, userID, listingID, startAt, endAt, 1, 2, "")
	require.NoError(t, err)

	err = uc.CancelReservation(ctx, userID, created.ID)
	require.NoError(t, err)

	err = uc.CancelReservation(ctx, userID, created.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reservation cannot be cancelled from current status")
}

func TestReservationUseCases_Idempotency(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	userID := createTestUser(t, db, orgID, domain.RoleAdmin)
	sellerID := createTestSeller(t, db, orgID, userID)
	storeID := createTestStore(t, db, orgID)
	categoryID := createTestCategory(t, db)
	listingID := createTestListing(t, db, orgID, sellerID, storeID, categoryID)

	uc := newTestUseCases(db)
	ctx := context.Background()

	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()
	idempotencyKey := "idem-test-" + testUUID()

	reservation1, err := uc.CreateReservation(ctx, userID, listingID, startAt, endAt, 1, 2, idempotencyKey)
	require.NoError(t, err)

	reservation2, err := uc.CreateReservation(ctx, userID, listingID, startAt, endAt, 1, 2, idempotencyKey)
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

type fakeCapacityRepo struct{}

func (f *fakeCapacityRepo) Create(ctx context.Context, unit *domain.CapacityUnit) error { return nil }
func (f *fakeCapacityRepo) FindByID(ctx context.Context, tenantID, id string) (*domain.CapacityUnit, error) {
	return nil, domain.ErrCapacityNotFound
}
func (f *fakeCapacityRepo) FindByInventoryID(ctx context.Context, tenantID, inventoryID string) ([]*domain.CapacityUnit, error) {
	return nil, nil
}
func (f *fakeCapacityRepo) FindByInventoryIDAndRange(ctx context.Context, tenantID, inventoryID string, startAt, endAt int64) ([]*domain.CapacityUnit, error) {
	return nil, nil
}
func (f *fakeCapacityRepo) FindByInventoryIDAndRangeForUpdate(ctx context.Context, tenantID, inventoryID string, startAt, endAt int64) ([]*domain.CapacityUnit, error) {
	return nil, nil
}
func (f *fakeCapacityRepo) Update(ctx context.Context, unit *domain.CapacityUnit) error { return nil }
func (f *fakeCapacityRepo) Reserve(ctx context.Context, id string, quantity int64, expectedVersion int64) error {
	return nil
}
func (f *fakeCapacityRepo) Release(ctx context.Context, id string, quantity int64, expectedVersion int64) error {
	return nil
}
func (f *fakeCapacityRepo) ReserveCapacity(ctx context.Context, tenantID, listingID string, startAt, endAt int64, quantity int64, reservationID string) error {
	return nil
}
func (f *fakeCapacityRepo) ReleaseCapacity(ctx context.Context, tenantID, listingID string, startAt, endAt int64, quantity int64, reservationID string) error {
	return nil
}

type fakeInventoryRepo struct{}

func (f *fakeInventoryRepo) Create(ctx context.Context, resource *domain.InventoryResource) error {
	return nil
}
func (f *fakeInventoryRepo) FindByID(ctx context.Context, tenantID, id string) (*domain.InventoryResource, error) {
	return nil, domain.ErrInventoryNotFound
}
func (f *fakeInventoryRepo) FindByListingID(ctx context.Context, tenantID, listingID string) (*domain.InventoryResource, error) {
	return nil, domain.ErrInventoryNotFound
}
func (f *fakeInventoryRepo) FindByListingIDForUpdate(ctx context.Context, tenantID, listingID string) (*domain.InventoryResource, error) {
	return nil, domain.ErrInventoryNotFound
}
func (f *fakeInventoryRepo) Update(ctx context.Context, resource *domain.InventoryResource) error {
	return nil
}
func (f *fakeInventoryRepo) FindByTenantID(ctx context.Context, tenantID string, offset, limit int) ([]*domain.InventoryResource, error) {
	return nil, nil
}
func (f *fakeInventoryRepo) Reserve(ctx context.Context, id string, quantity int64, expectedVersion int64) error {
	return nil
}
func (f *fakeInventoryRepo) Release(ctx context.Context, id string, quantity int64, expectedVersion int64) error {
	return nil
}
func (f *fakeInventoryRepo) Consume(ctx context.Context, id string, quantity int64, expectedVersion int64) error {
	return nil
}

type fakeFulfillmentRepo struct{}

func (f *fakeFulfillmentRepo) Create(ctx context.Context, fulfillment *domain.Fulfillment) error {
	return nil
}
func (f *fakeFulfillmentRepo) FindByID(ctx context.Context, tenantID, id string) (*domain.Fulfillment, error) {
	return nil, domain.ErrFulfillmentNotFound
}
func (f *fakeFulfillmentRepo) FindByIDForUpdate(ctx context.Context, tenantID, id string) (*domain.Fulfillment, error) {
	return nil, domain.ErrFulfillmentNotFound
}
func (f *fakeFulfillmentRepo) FindByOrderID(ctx context.Context, tenantID, orderID string) (*domain.Fulfillment, error) {
	return nil, domain.ErrFulfillmentNotFound
}
func (f *fakeFulfillmentRepo) FindByReservationID(ctx context.Context, tenantID, reservationID string) (*domain.Fulfillment, error) {
	return nil, domain.ErrFulfillmentNotFound
}
func (f *fakeFulfillmentRepo) FindByAssignedTo(ctx context.Context, tenantID, actorID string, status *domain.FulfillmentStatus) ([]*domain.Fulfillment, error) {
	return nil, nil
}
func (f *fakeFulfillmentRepo) Update(ctx context.Context, fulfillment *domain.Fulfillment) error {
	return nil
}

type fakeAuditLogger struct{}

func (f *fakeAuditLogger) Record(ctx context.Context, entry domain.AuditEntry) error { return nil }

// Finding S-5: platform staff could read any order but not the reservation behind it.
func TestReservationUseCases_PlatformStaffCanReadAndCancelAnyReservation(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	customerID := createTestUser(t, db, orgID, domain.RoleCustomer)
	sellerUser := createTestUser(t, db, orgID, domain.RoleSeller)
	sellerID := createTestSeller(t, db, orgID, sellerUser)
	listingID := createTestListing(t, db, orgID, sellerID, createTestStore(t, db, orgID), createTestCategory(t, db))
	operator := grantPlatformRole(t, db, createTestUser(t, db, orgID, domain.RoleCustomer), domain.RoleOperator)
	admin := grantPlatformRole(t, db, createTestUser(t, db, orgID, domain.RoleCustomer), domain.RoleAdmin)
	stranger := createTestUser(t, db, orgID, domain.RoleCustomer)

	uc := newTestUseCases(db)
	ctx := context.Background()
	start, end := time.Now().Add(24*time.Hour).Unix(), time.Now().Add(48*time.Hour).Unix()
	booked, err := uc.CreateReservation(ctx, customerID, listingID, start, end, 1, 2, "")
	require.NoError(t, err)

	// a customer who is not the booker still cannot see it
	_, err = uc.GetReservation(ctx, stranger, booked.ID)
	assert.ErrorIs(t, err, domain.ErrInsufficientPermissions)
	assert.ErrorIs(t, uc.CancelReservation(ctx, stranger, booked.ID), domain.ErrInsufficientPermissions)

	// support can read it, whatever kind of staff it is
	for name, staff := range map[string]string{"operator": operator, "admin": admin} {
		got, err := uc.GetReservation(ctx, staff, booked.ID)
		require.NoError(t, err, name)
		assert.Equal(t, booked.ID, got.ID, name)
	}

	// and cancel it, once
	require.NoError(t, uc.CancelReservation(ctx, operator, booked.ID))
	got, err := uc.GetReservation(ctx, admin, booked.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ReservationStatusCancelled, got.Status)
	assert.ErrorIs(t, uc.CancelReservation(ctx, admin, booked.ID), domain.ErrReservationCannotBeCancelled)
}

func TestReservationUseCases_ASuspendedStaffAccountReadsNothing(t *testing.T) {
	db := setupReservationTestDB(t)
	defer db.Close()

	orgID := createTestOrg(t, db)
	customerID := createTestUser(t, db, orgID, domain.RoleCustomer)
	sellerID := createTestSeller(t, db, orgID, createTestUser(t, db, orgID, domain.RoleSeller))
	listingID := createTestListing(t, db, orgID, sellerID, createTestStore(t, db, orgID), createTestCategory(t, db))
	staff := grantPlatformRole(t, db, createTestUser(t, db, orgID, domain.RoleCustomer), domain.RoleOperator)

	uc := newTestUseCases(db)
	ctx := context.Background()
	booked, err := uc.CreateReservation(ctx, customerID, listingID, time.Now().Add(24*time.Hour).Unix(), time.Now().Add(48*time.Hour).Unix(), 1, 2, "")
	require.NoError(t, err)

	_, err = db.Exec(`UPDATE users SET status = 'suspended' WHERE id = $1`, staff)
	require.NoError(t, err)
	_, err = uc.GetReservation(ctx, staff, booked.ID)
	assert.ErrorIs(t, err, domain.ErrInsufficientPermissions)
	assert.ErrorIs(t, uc.CancelReservation(ctx, staff, booked.ID), domain.ErrInsufficientPermissions)
}
