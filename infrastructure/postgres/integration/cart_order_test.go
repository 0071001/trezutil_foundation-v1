package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/application/hotel"
	"github.com/trezutil/trezutil/internal/domain"
)

// Multi-item orders (a cart) on real PostgreSQL: one order, several lines from
// one store, an exact total, and idempotent retries.
func TestCartOrder_MultipleItemsOneStore(t *testing.T) {
	f := newHotelFixture(t)
	ctx := context.Background()
	mk := func(actor, store, seller, title string, price float64) string {
		rt, err := f.svc.CreateRoomType(ctx, actor, hotel.NewRoomType{
			StoreID: store, SellerID: seller, CategoryID: hCategory, Title: title, Description: "x",
			PriceAmount: price, PriceCurrency: "USD", MaxOccupancy: 2, Units: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := f.listings.PublishListing(ctx, actor, rt.ListingID); err != nil {
			t.Fatal(err)
		}
		return rt.ListingID
	}
	a1 := mk(hOwnerA, hStoreA, hSellerA, "Item one", 19.99)
	a2 := mk(hOwnerA, hStoreA, hSellerA, "Item two", 5)
	b1 := mk(hOwnerB, hStoreB, hSellerB, "Other store item", 7)

	orders := application.NewOrderUseCases(postgres.NewOrderRepository(f.db.DB), postgres.NewListingRepository(f.db.DB),
		postgres.NewSellerRepository(f.db.DB), postgres.NewOrganizationRepository(f.db.DB), postgres.NewUserRepository(f.db.DB),
		postgres.NewInventoryRepository(f.db.DB), postgres.NewFulfillmentRepository(f.db.DB), nil)

	lines := []domain.OrderLine{{ListingID: a1, Quantity: 2}, {ListingID: a2, Quantity: 3}, {ListingID: a1, Quantity: 1}}
	order, err := orders.CreateOrderWithLines(ctx, hGuest, lines, "cart-1")
	if err != nil {
		t.Fatal(err)
	}
	// 3 x 19.99 + 3 x 5.00 = 74.97 USD, repeated listings merged.
	if len(order.Items) != 2 || order.TotalAmount.AmountMinor != 7497 || order.CustomerID != hGuest {
		t.Fatalf("order = %+v total=%+v", order.Items, order.TotalAmount)
	}
	stored, err := postgres.NewOrderRepository(f.db.DB).FindByID(ctx, order.ID)
	if err != nil || len(stored.Items) != 2 || stored.TotalAmount.AmountMinor != 7497 {
		t.Fatalf("stored = %+v %v", stored, err)
	}

	// Same key + same items returns the original; other items conflict.
	again, err := orders.CreateOrderWithLines(ctx, hGuest, []domain.OrderLine{{ListingID: a2, Quantity: 3}, {ListingID: a1, Quantity: 3}}, "cart-1")
	if err != nil || again.ID != order.ID {
		t.Fatalf("retry = %v %v", again, err)
	}
	if _, err := orders.CreateOrderWithLines(ctx, hGuest, []domain.OrderLine{{ListingID: a1, Quantity: 1}}, "cart-1"); !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("different items with the same key: %v", err)
	}

	// A cart cannot span stores; nothing is created.
	if _, err := orders.CreateOrderWithLines(ctx, hGuest, []domain.OrderLine{{ListingID: a1, Quantity: 1}, {ListingID: b1, Quantity: 1}}, ""); !errors.Is(err, domain.ErrOrderMixedStores) {
		t.Fatalf("mixed stores: %v", err)
	}
	// Unknown listings and empty carts are refused.
	if _, err := orders.CreateOrderWithLines(ctx, hGuest, []domain.OrderLine{{ListingID: "40fe1000-0000-4000-8000-0000000fffff", Quantity: 1}}, ""); !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("unknown listing: %v", err)
	}
	if _, err := orders.CreateOrderWithLines(ctx, hGuest, nil, ""); !errors.Is(err, domain.ErrInvalidOrderLines) {
		t.Fatalf("empty cart: %v", err)
	}
	if _, err := orders.CreateOrderWithLines(ctx, "", lines, ""); !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Fatalf("no actor: %v", err)
	}
	var n int
	if err := f.db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM orders`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("orders after refused carts = %d %v", n, err)
	}
}
