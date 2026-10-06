package application

import (
	"context"
	"errors"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

// countingFulfillments counts the fulfillment records created for orders.
type countingFulfillments struct {
	fakeFulfillmentRepoForOrder
	created []*domain.Fulfillment
}

func (c *countingFulfillments) Create(ctx context.Context, f *domain.Fulfillment) error {
	c.created = append(c.created, f)
	return nil
}

// staleOrderRepo behaves like the real repository when another request saved the order first.
type staleOrderRepo struct{ *fakeOrderRepo }

func (s staleOrderRepo) Update(ctx context.Context, order *domain.Order) error {
	return domain.ErrOptimisticLock
}

type lifecycleFixture struct {
	order    *domain.Order
	uc       *OrderUseCases
	fulfills *countingFulfillments
}

// newLifecycleFixture builds an order of seller-1 (whose owner is "seller-user") placed by "buyer" in the given status.
func newLifecycleFixture(status domain.OrderStatus, wrapRepo func(*fakeOrderRepo) domain.OrderRepository) *lifecycleFixture {
	item := &domain.OrderItem{ListingID: "listing-1", Quantity: 1}
	order := domain.NewOrder("org-1", "seller-1", "buyer", "store-1", []*domain.OrderItem{item}, "USD", 1000)
	order.ID = "order-1"
	order.Status = status

	repo := &fakeOrderRepo{order: order}
	var orderRepo domain.OrderRepository = repo
	if wrapRepo != nil {
		orderRepo = wrapRepo(repo)
	}
	fulfills := &countingFulfillments{}
	uc := NewOrderUseCases(
		orderRepo,
		&fakeListingRepoForOrder{listing: publishedListing(10)},
		&fakeSellerRepoForOrder{seller: &domain.SellerProfile{ID: "seller-1", UserID: strPtr("seller-user"), OrganizationID: strPtr("org-1")}},
		&fakeOrgRepoForOrder{},
		&fakeUserRepoForOrder{user: &domain.User{ID: "seller-user", Status: domain.UserStatusActive}},
		&fakeInventoryRepoForOrder{}, fulfills, &fakeAuditLoggerForOrder{},
	)
	return &lifecycleFixture{order: order, uc: uc, fulfills: fulfills}
}

func TestOrderLifecycle_SellerCarriesAnOrderFromConfirmedToCompleted(t *testing.T) {
	f := newLifecycleFixture(domain.OrderStatusConfirmed, nil)
	ctx := context.Background()

	steps := []struct {
		name string
		do   func() (*domain.Order, error)
		want domain.OrderStatus
	}{
		{"start preparing", func() (*domain.Order, error) { return f.uc.StartProcessing(ctx, "seller-user", "order-1") }, domain.OrderStatusProcessing},
		{"mark ready", func() (*domain.Order, error) { return f.uc.ReadyForFulfillment(ctx, "seller-user", "order-1") }, domain.OrderStatusReadyForFulfillment},
		{"mark delivered", func() (*domain.Order, error) { return f.uc.FulfilOrder(ctx, "seller-user", "order-1") }, domain.OrderStatusFulfilled},
		{"complete", func() (*domain.Order, error) { return f.uc.CompleteOrder(ctx, "seller-user", "order-1") }, domain.OrderStatusCompleted},
	}
	for _, s := range steps {
		got, err := s.do()
		if err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		if got.Status != s.want {
			t.Fatalf("%s: status = %s, want %s", s.name, got.Status, s.want)
		}
	}
	if len(f.fulfills.created) != 1 {
		t.Fatalf("fulfillment records created = %d, want exactly one (when the order became ready)", len(f.fulfills.created))
	}
	if !f.order.IsTerminal() {
		t.Fatal("a completed order is terminal")
	}
}

func TestOrderLifecycle_ARefusedStepLeavesTheOrderAlone(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		status domain.OrderStatus
		do     func(uc *OrderUseCases) error
	}{
		{"start preparing a paid order", domain.OrderStatusPaid, func(uc *OrderUseCases) error { _, err := uc.StartProcessing(ctx, "seller-user", "order-1"); return err }},
		{"mark ready an order not yet started", domain.OrderStatusConfirmed, func(uc *OrderUseCases) error {
			_, err := uc.ReadyForFulfillment(ctx, "seller-user", "order-1")
			return err
		}},
		{"deliver an order not yet ready", domain.OrderStatusProcessing, func(uc *OrderUseCases) error { _, err := uc.FulfilOrder(ctx, "seller-user", "order-1"); return err }},
		{"complete an order not yet delivered", domain.OrderStatusReadyForFulfillment, func(uc *OrderUseCases) error { _, err := uc.CompleteOrder(ctx, "seller-user", "order-1"); return err }},
		{"start preparing a completed order", domain.OrderStatusCompleted, func(uc *OrderUseCases) error { _, err := uc.StartProcessing(ctx, "seller-user", "order-1"); return err }},
		{"complete a cancelled order", domain.OrderStatusCancelled, func(uc *OrderUseCases) error { _, err := uc.CompleteOrder(ctx, "seller-user", "order-1"); return err }},
		{"confirm an order that is already confirmed", domain.OrderStatusConfirmed, func(uc *OrderUseCases) error { _, err := uc.ConfirmOrder(ctx, "seller-user", "order-1"); return err }},
	}
	for _, c := range cases {
		f := newLifecycleFixture(c.status, nil)
		err := c.do(f.uc)
		if !errors.Is(err, domain.ErrInvalidOrderTransition) {
			t.Errorf("%s: err = %v, want ErrInvalidOrderTransition", c.name, err)
		}
		if f.order.Status != c.status {
			t.Errorf("%s: status changed to %s", c.name, f.order.Status)
		}
		if len(f.fulfills.created) != 0 {
			t.Errorf("%s: a fulfillment record was created", c.name)
		}
	}
}

func TestOrderLifecycle_OnlyTheSellerSideMayMoveAnOrder(t *testing.T) {
	ctx := context.Background()
	for _, actor := range []string{"buyer", "stranger"} {
		f := newLifecycleFixture(domain.OrderStatusConfirmed, nil)
		if _, err := f.uc.StartProcessing(ctx, actor, "order-1"); !errors.Is(err, domain.ErrInsufficientPermissions) {
			t.Errorf("%s started the order: %v", actor, err)
		}
		if f.order.Status != domain.OrderStatusConfirmed {
			t.Errorf("%s moved the order to %s", actor, f.order.Status)
		}
	}
	for _, status := range []domain.OrderStatus{domain.OrderStatusProcessing, domain.OrderStatusReadyForFulfillment, domain.OrderStatusFulfilled} {
		f := newLifecycleFixture(status, nil)
		steps := map[domain.OrderStatus]func() error{
			domain.OrderStatusProcessing:          func() error { _, err := f.uc.ReadyForFulfillment(ctx, "buyer", "order-1"); return err },
			domain.OrderStatusReadyForFulfillment: func() error { _, err := f.uc.FulfilOrder(ctx, "buyer", "order-1"); return err },
			domain.OrderStatusFulfilled:           func() error { _, err := f.uc.CompleteOrder(ctx, "buyer", "order-1"); return err },
		}
		if err := steps[status](); !errors.Is(err, domain.ErrInsufficientPermissions) {
			t.Errorf("buyer moved an order that was %s: %v", status, err)
		}
		if f.order.Status != status {
			t.Errorf("an order that was %s became %s", status, f.order.Status)
		}
	}
	// no caller, no order
	f := newLifecycleFixture(domain.OrderStatusConfirmed, nil)
	if _, err := f.uc.StartProcessing(ctx, "", "order-1"); !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Errorf("anonymous: %v", err)
	}
	if _, err := f.uc.StartProcessing(ctx, "seller-user", "missing"); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Errorf("unknown order: %v", err)
	}
}

func TestOrderLifecycle_APlatformOperatorMayMoveAnOrder(t *testing.T) {
	f := newLifecycleFixture(domain.OrderStatusConfirmed, nil)
	f.uc.userRepo = &fakeUserRepoForOrder{user: &domain.User{ID: "ops", Status: domain.UserStatusActive, Roles: []domain.Role{domain.RoleOperator}}}
	if _, err := f.uc.StartProcessing(context.Background(), "ops", "order-1"); err != nil {
		t.Fatalf("operator: %v", err)
	}
}

// Two clicks racing: the one that loses the order's version must not repeat the step, and above all must not
// leave a second fulfillment record behind.
func TestOrderLifecycle_ALosingConcurrentClickChangesNothing(t *testing.T) {
	f := newLifecycleFixture(domain.OrderStatusProcessing, func(r *fakeOrderRepo) domain.OrderRepository { return staleOrderRepo{r} })
	_, err := f.uc.ReadyForFulfillment(context.Background(), "seller-user", "order-1")
	if !errors.Is(err, domain.ErrOptimisticLock) {
		t.Fatalf("err = %v, want ErrOptimisticLock", err)
	}
	if len(f.fulfills.created) != 0 {
		t.Fatalf("the losing click created %d fulfillment records", len(f.fulfills.created))
	}
}
