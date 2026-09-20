package application

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

type fakeOrderRepo struct {
	order *domain.Order
	err   error
}

func (f *fakeOrderRepo) Create(ctx context.Context, order *domain.Order) error {
	if f.err != nil {
		return f.err
	}
	f.order = order
	return nil
}
func (f *fakeOrderRepo) FindByID(ctx context.Context, orderID string) (*domain.Order, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.order != nil && f.order.ID == orderID {
		return f.order, nil
	}
	return nil, domain.ErrOrderNotFound
}
func (f *fakeOrderRepo) Update(ctx context.Context, order *domain.Order) error {
	if f.err != nil {
		return f.err
	}
	return nil
}

type fakeListingRepoForOrder struct {
	listing *domain.Listing
	err     error
}

func (f *fakeListingRepoForOrder) Create(ctx context.Context, listing *domain.Listing) error {
	return nil
}
func (f *fakeListingRepoForOrder) FindByID(ctx context.Context, id string) (*domain.Listing, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.listing != nil && f.listing.ID == id {
		return f.listing, nil
	}
	return nil, domain.ErrListingNotFound
}
func (f *fakeListingRepoForOrder) FindByOrganizationID(ctx context.Context, orgID string, offset, limit int) ([]domain.Listing, error) {
	return nil, nil
}
func (f *fakeListingRepoForOrder) FindBySellerID(ctx context.Context, sellerID string, offset, limit int) ([]domain.Listing, error) {
	return nil, nil
}
func (f *fakeListingRepoForOrder) FindByStoreID(ctx context.Context, storeID string, offset, limit int) ([]domain.Listing, error) {
	return nil, nil
}
func (f *fakeListingRepoForOrder) FindByCategoryID(ctx context.Context, categoryID string, offset, limit int) ([]domain.Listing, error) {
	return nil, nil
}
func (f *fakeListingRepoForOrder) Update(ctx context.Context, listing *domain.Listing) error {
	return nil
}
func (f *fakeListingRepoForOrder) Delete(ctx context.Context, id string) error { return nil }
func (f *fakeListingRepoForOrder) CountByOrganizationID(ctx context.Context, orgID string) (int, error) {
	return 0, nil
}

type fakeSellerRepoForOrder struct {
	seller *domain.SellerProfile
	err    error
}

func (f *fakeSellerRepoForOrder) Create(ctx context.Context, seller *domain.SellerProfile) error {
	return nil
}
func (f *fakeSellerRepoForOrder) FindByID(ctx context.Context, id string) (*domain.SellerProfile, error) {
	return f.seller, nil
}
func (f *fakeSellerRepoForOrder) Update(ctx context.Context, seller *domain.SellerProfile) error {
	return nil
}
func (f *fakeSellerRepoForOrder) FindByUserID(ctx context.Context, userID string) (*domain.SellerProfile, error) {
	if f.seller != nil && f.seller.UserID != nil && *f.seller.UserID == userID {
		return f.seller, nil
	}
	return nil, domain.ErrSellerNotFound
}
func (f *fakeSellerRepoForOrder) FindByOrganizationID(ctx context.Context, orgID string) ([]domain.SellerProfile, error) {
	return nil, nil
}

type fakeOrgRepoForOrder struct {
	memberRole domain.OrganizationRole
	err        error
}

func (f *fakeOrgRepoForOrder) Create(ctx context.Context, org *domain.Organization) error { return nil }
func (f *fakeOrgRepoForOrder) FindByID(ctx context.Context, id string) (*domain.Organization, error) {
	return nil, nil
}
func (f *fakeOrgRepoForOrder) Update(ctx context.Context, org *domain.Organization) error { return nil }
func (f *fakeOrgRepoForOrder) AddMember(ctx context.Context, member *domain.OrganizationMember) error {
	return nil
}
func (f *fakeOrgRepoForOrder) RemoveMember(ctx context.Context, orgID, userID string) error {
	return nil
}
func (f *fakeOrgRepoForOrder) GetMembers(ctx context.Context, orgID string) ([]domain.OrganizationMember, error) {
	return nil, nil
}
func (f *fakeOrgRepoForOrder) HasMember(ctx context.Context, orgID, userID string) (bool, error) {
	return false, nil
}
func (f *fakeOrgRepoForOrder) GetMemberRole(ctx context.Context, orgID, userID string) (domain.OrganizationRole, error) {
	return f.memberRole, f.err
}

type fakeUserRepoForOrder struct {
	user *domain.User
	err  error
}

func (f *fakeUserRepoForOrder) FindByID(ctx context.Context, id string) (*domain.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.user != nil && f.user.ID == id {
		return f.user, nil
	}
	return nil, domain.ErrOrganizationNotFound
}
func (f *fakeUserRepoForOrder) FindByPhone(ctx context.Context, phone string) (*domain.User, error) {
	return nil, nil
}
func (f *fakeUserRepoForOrder) Create(ctx context.Context, user *domain.User) error { return nil }
func (f *fakeUserRepoForOrder) Update(ctx context.Context, user *domain.User) error { return nil }
func (f *fakeUserRepoForOrder) AssignRole(ctx context.Context, userID string, role domain.Role) error {
	return nil
}
func (f *fakeUserRepoForOrder) RemoveRole(ctx context.Context, userID string, role domain.Role) error {
	return nil
}
func (f *fakeUserRepoForOrder) GetUserRoles(ctx context.Context, userID string) ([]domain.Role, error) {
	return nil, nil
}
func (f *fakeUserRepoForOrder) HasRole(ctx context.Context, userID string, role domain.Role) (bool, error) {
	if f.user != nil {
		for _, r := range f.user.Roles {
			if r == role {
				return true, nil
			}
		}
	}
	return false, nil
}

type fakeAuditLoggerForOrder struct{}

func (f *fakeAuditLoggerForOrder) Record(ctx context.Context, entry domain.AuditEntry) error {
	return nil
}

func NewTestOrderUseCases(
	orderRepo *fakeOrderRepo,
	listingRepo *fakeListingRepoForOrder,
	sellerRepo *fakeSellerRepoForOrder,
	orgRepo *fakeOrgRepoForOrder,
	userRepo *fakeUserRepoForOrder,
) *OrderUseCases {
	return NewOrderUseCases(orderRepo, listingRepo, sellerRepo, orgRepo, userRepo, &fakeAuditLoggerForOrder{})
}

func TestOrderUseCases_CreateOrder_Success(t *testing.T) {
	listing := domain.NewListing("org-1", "seller-1", "store-1", "cat-1", "Test Listing", "physical_product", "fixed", "USD", 99.99)
	listing.ID = "listing-1"
	listingRepo := &fakeListingRepoForOrder{listing: listing}
	sellerRepo := &fakeSellerRepoForOrder{seller: &domain.SellerProfile{ID: "seller-1", UserID: strPtr("user-1"), OrganizationID: strPtr("org-1")}}
	userRepo := &fakeUserRepoForOrder{user: &domain.User{ID: "user-1"}}
	orderRepo := &fakeOrderRepo{}
	orgRepo := &fakeOrgRepoForOrder{}

	useCases := NewTestOrderUseCases(orderRepo, listingRepo, sellerRepo, orgRepo, userRepo)

	order, err := useCases.CreateOrder(context.Background(), "user-1", "listing-1", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order == nil {
		t.Fatal("expected non-nil order")
	}
	if order.OrganizationID != "org-1" {
		t.Errorf("expected org-1, got %s", order.OrganizationID)
	}
	if order.CustomerID != "user-1" {
		t.Errorf("expected user-1, got %s", order.CustomerID)
	}
	if len(order.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(order.Items))
	}
	if order.Items[0].Quantity != 2 {
		t.Errorf("expected quantity 2, got %d", order.Items[0].Quantity)
	}
	if order.PricingSnapshot == nil {
		t.Fatal("expected pricing snapshot")
	}
	if order.PricingSnapshot.TotalAmount.AmountMinor != 19998 {
		t.Errorf("expected total 19998, got %d", order.PricingSnapshot.TotalAmount.AmountMinor)
	}
}

func TestOrderUseCases_CreateOrder_MissingListing(t *testing.T) {
	listingRepo := &fakeListingRepoForOrder{err: domain.ErrListingNotFound}
	userRepo := &fakeUserRepoForOrder{user: &domain.User{ID: "user-1"}}
	orderRepo := &fakeOrderRepo{}
	sellerRepo := &fakeSellerRepoForOrder{}
	orgRepo := &fakeOrgRepoForOrder{}

	useCases := NewTestOrderUseCases(orderRepo, listingRepo, sellerRepo, orgRepo, userRepo)

	_, err := useCases.CreateOrder(context.Background(), "user-1", "missing", 1)
	if err == nil {
		t.Error("expected error for missing listing")
	}
}

func TestOrderUseCases_CreateOrder_InvalidQuantity(t *testing.T) {
	listingRepo := &fakeListingRepoForOrder{listing: &domain.Listing{ID: "l1", PriceAmount: 10, PriceCurrency: "USD", OrganizationID: "org-1", SellerID: "s1", StoreID: "st1", Title: "T"}}
	userRepo := &fakeUserRepoForOrder{user: &domain.User{ID: "user-1"}}
	orderRepo := &fakeOrderRepo{}
	sellerRepo := &fakeSellerRepoForOrder{}
	orgRepo := &fakeOrgRepoForOrder{}

	useCases := NewTestOrderUseCases(orderRepo, listingRepo, sellerRepo, orgRepo, userRepo)

	_, err := useCases.CreateOrder(context.Background(), "user-1", "l1", 0)
	if err == nil {
		t.Error("expected error for zero quantity")
	}
	_, err = useCases.CreateOrder(context.Background(), "user-1", "l1", -1)
	if err == nil {
		t.Error("expected error for negative quantity")
	}
}

func TestOrderUseCases_CreateOrder_EmptyActor(t *testing.T) {
	listingRepo := &fakeListingRepoForOrder{listing: &domain.Listing{ID: "l1", PriceAmount: 10, PriceCurrency: "USD", OrganizationID: "org-1", SellerID: "s1", StoreID: "st1", Title: "T"}}
	userRepo := &fakeUserRepoForOrder{}
	orderRepo := &fakeOrderRepo{}
	sellerRepo := &fakeSellerRepoForOrder{}
	orgRepo := &fakeOrgRepoForOrder{}

	useCases := NewTestOrderUseCases(orderRepo, listingRepo, sellerRepo, orgRepo, userRepo)

	_, err := useCases.CreateOrder(context.Background(), "", "l1", 1)
	if err == nil {
		t.Error("expected error for empty actor")
	}
}

func TestOrderUseCases_CreateOrder_Unauthorized(t *testing.T) {
	listing := domain.NewListing("org-1", "seller-1", "store-1", "cat-1", "Test", "physical_product", "fixed", "USD", 99.99)
	listing.ID = "listing-1"
	listingRepo := &fakeListingRepoForOrder{listing: listing}
	sellerRepo := &fakeSellerRepoForOrder{seller: &domain.SellerProfile{ID: "seller-1", UserID: strPtr("other-user"), OrganizationID: strPtr("org-1")}}
	userRepo := &fakeUserRepoForOrder{user: &domain.User{ID: "user-2"}}
	orderRepo := &fakeOrderRepo{}
	orgRepo := &fakeOrgRepoForOrder{}

	useCases := NewTestOrderUseCases(orderRepo, listingRepo, sellerRepo, orgRepo, userRepo)

	_, err := useCases.CreateOrder(context.Background(), "user-2", "listing-1", 1)
	if err != domain.ErrInsufficientPermissions {
		t.Errorf("expected ErrInsufficientPermissions, got %v", err)
	}
}

func TestOrderUseCases_GetOrder_Success(t *testing.T) {
	order := domain.NewOrder("org-1", "seller-1", "user-1", "store-1", []*domain.OrderItem{}, "USD", 1000)
	order.ID = "order-1"
	orderRepo := &fakeOrderRepo{order: order}
	userRepo := &fakeUserRepoForOrder{user: &domain.User{ID: "user-1"}}
	sellerRepo := &fakeSellerRepoForOrder{}
	orgRepo := &fakeOrgRepoForOrder{}
	listingRepo := &fakeListingRepoForOrder{}

	useCases := NewTestOrderUseCases(orderRepo, listingRepo, sellerRepo, orgRepo, userRepo)

	result, err := useCases.GetOrder(context.Background(), "user-1", "order-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ID != "order-1" {
		t.Errorf("expected order-1, got %s", result.ID)
	}
}

func TestOrderUseCases_GetOrder_NotFound(t *testing.T) {
	orderRepo := &fakeOrderRepo{err: domain.ErrOrderNotFound}
	userRepo := &fakeUserRepoForOrder{user: &domain.User{ID: "user-1"}}
	sellerRepo := &fakeSellerRepoForOrder{}
	orgRepo := &fakeOrgRepoForOrder{}
	listingRepo := &fakeListingRepoForOrder{}

	useCases := NewTestOrderUseCases(orderRepo, listingRepo, sellerRepo, orgRepo, userRepo)

	_, err := useCases.GetOrder(context.Background(), "user-1", "missing")
	if err != domain.ErrOrderNotFound {
		t.Errorf("expected ErrOrderNotFound, got %v", err)
	}
}

func TestOrderUseCases_GetOrder_EmptyActor(t *testing.T) {
	orderRepo := &fakeOrderRepo{}
	userRepo := &fakeUserRepoForOrder{}
	sellerRepo := &fakeSellerRepoForOrder{}
	orgRepo := &fakeOrgRepoForOrder{}
	listingRepo := &fakeListingRepoForOrder{}

	useCases := NewTestOrderUseCases(orderRepo, listingRepo, sellerRepo, orgRepo, userRepo)

	_, err := useCases.GetOrder(context.Background(), "", "order-1")
	if err == nil {
		t.Error("expected error for empty actor")
	}
}

func TestOrderUseCases_CancelOrder_Success(t *testing.T) {
	order := domain.NewOrder("org-1", "seller-1", "user-1", "store-1", []*domain.OrderItem{}, "USD", 1000)
	order.ID = "order-1"
	order.Status = domain.OrderStatusDraft
	orderRepo := &fakeOrderRepo{order: order}
	userRepo := &fakeUserRepoForOrder{user: &domain.User{ID: "user-1"}}
	sellerRepo := &fakeSellerRepoForOrder{}
	orgRepo := &fakeOrgRepoForOrder{}
	listingRepo := &fakeListingRepoForOrder{}

	useCases := NewTestOrderUseCases(orderRepo, listingRepo, sellerRepo, orgRepo, userRepo)

	err := useCases.CancelOrder(context.Background(), "user-1", "order-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != domain.OrderStatusCancelled {
		t.Errorf("expected cancelled, got %s", order.Status)
	}
}

func TestOrderUseCases_CancelOrder_NotFound(t *testing.T) {
	orderRepo := &fakeOrderRepo{err: domain.ErrOrderNotFound}
	userRepo := &fakeUserRepoForOrder{user: &domain.User{ID: "user-1"}}
	sellerRepo := &fakeSellerRepoForOrder{}
	orgRepo := &fakeOrgRepoForOrder{}
	listingRepo := &fakeListingRepoForOrder{}

	useCases := NewTestOrderUseCases(orderRepo, listingRepo, sellerRepo, orgRepo, userRepo)

	err := useCases.CancelOrder(context.Background(), "user-1", "missing")
	if err != domain.ErrOrderNotFound {
		t.Errorf("expected ErrOrderNotFound, got %v", err)
	}
}

func TestOrderUseCases_CancelOrder_InvalidState(t *testing.T) {
	order := domain.NewOrder("org-1", "seller-1", "user-1", "store-1", []*domain.OrderItem{}, "USD", 1000)
	order.ID = "order-1"
	order.Status = domain.OrderStatusPaid
	orderRepo := &fakeOrderRepo{order: order}
	userRepo := &fakeUserRepoForOrder{user: &domain.User{ID: "user-1"}}
	sellerRepo := &fakeSellerRepoForOrder{}
	orgRepo := &fakeOrgRepoForOrder{}
	listingRepo := &fakeListingRepoForOrder{}

	useCases := NewTestOrderUseCases(orderRepo, listingRepo, sellerRepo, orgRepo, userRepo)

	err := useCases.CancelOrder(context.Background(), "user-1", "order-1")
	if err != domain.ErrOrderCannotBeCancelled {
		t.Errorf("expected ErrOrderCannotBeCancelled, got %v", err)
	}
}

func TestOrderUseCases_CancelOrder_EmptyActor(t *testing.T) {
	orderRepo := &fakeOrderRepo{}
	userRepo := &fakeUserRepoForOrder{}
	sellerRepo := &fakeSellerRepoForOrder{}
	orgRepo := &fakeOrgRepoForOrder{}
	listingRepo := &fakeListingRepoForOrder{}

	useCases := NewTestOrderUseCases(orderRepo, listingRepo, sellerRepo, orgRepo, userRepo)

	err := useCases.CancelOrder(context.Background(), "", "order-1")
	if err == nil {
		t.Error("expected error for empty actor")
	}
}

func TestOrderUseCases_CancelOrder_Unauthorized(t *testing.T) {
	order := domain.NewOrder("org-1", "seller-1", "other-user", "store-1", []*domain.OrderItem{}, "USD", 1000)
	order.ID = "order-1"
	order.Status = domain.OrderStatusDraft
	orderRepo := &fakeOrderRepo{order: order}
	userRepo := &fakeUserRepoForOrder{user: &domain.User{ID: "user-2"}}
	sellerRepo := &fakeSellerRepoForOrder{}
	orgRepo := &fakeOrgRepoForOrder{}
	listingRepo := &fakeListingRepoForOrder{}

	useCases := NewTestOrderUseCases(orderRepo, listingRepo, sellerRepo, orgRepo, userRepo)

	err := useCases.CancelOrder(context.Background(), "user-2", "order-1")
	if err != domain.ErrInsufficientPermissions {
		t.Errorf("expected ErrInsufficientPermissions, got %v", err)
	}
}

func TestOrderUseCases_GetOrder_TenantIsolation(t *testing.T) {
	order := domain.NewOrder("org-1", "seller-1", "user-1", "store-1", []*domain.OrderItem{}, "USD", 1000)
	order.ID = "order-1"
	orderRepo := &fakeOrderRepo{order: order}
	userRepo := &fakeUserRepoForOrder{user: &domain.User{ID: "user-2"}}
	sellerRepo := &fakeSellerRepoForOrder{}
	orgRepo := &fakeOrgRepoForOrder{}
	listingRepo := &fakeListingRepoForOrder{}

	useCases := NewTestOrderUseCases(orderRepo, listingRepo, sellerRepo, orgRepo, userRepo)

	_, err := useCases.GetOrder(context.Background(), "user-2", "order-1")
	if err != domain.ErrInsufficientPermissions {
		t.Errorf("expected ErrInsufficientPermissions, got %v", err)
	}
}

func strPtr(s string) *string { return &s }
