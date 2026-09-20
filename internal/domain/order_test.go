package domain

import (
	"testing"
	"time"
)

func newTestNow() int64 {
	return time.Now().Unix()
}

func TestNewOrder(t *testing.T) {
	now := newTestNow()
	items := []*OrderItem{
		NewOrderItem("order-1", "listing-1", "Test Product", 2, NewMoney(5000, "USD"), now),
	}
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", items, "USD", now)

	if order.OrganizationID != "org-1" {
		t.Errorf("expected org-1, got %s", order.OrganizationID)
	}
	if order.SellerID != "seller-1" {
		t.Errorf("expected seller-1, got %s", order.SellerID)
	}
	if order.CustomerID != "customer-1" {
		t.Errorf("expected customer-1, got %s", order.CustomerID)
	}
	if order.StoreID != "store-1" {
		t.Errorf("expected store-1, got %s", order.StoreID)
	}
	if order.Status != OrderStatusDraft {
		t.Errorf("expected draft, got %s", order.Status)
	}
	if order.CreatedAt != now {
		t.Errorf("expected created_at %d, got %d", now, order.CreatedAt)
	}
	if order.UpdatedAt != now {
		t.Errorf("expected updated_at %d, got %d", now, order.UpdatedAt)
	}
	if order.Version != 1 {
		t.Errorf("expected version 1, got %d", order.Version)
	}
	if order.TotalAmount.AmountMinor != 10000 {
		t.Errorf("expected total 10000, got %d", order.TotalAmount.AmountMinor)
	}
	if order.Currency != "USD" {
		t.Errorf("expected USD, got %s", order.Currency)
	}
	if len(order.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(order.Items))
	}
}

func TestNewOrderEmptyItems(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)

	if order.TotalAmount.AmountMinor != 0 {
		t.Errorf("expected total 0, got %d", order.TotalAmount.AmountMinor)
	}
}

func TestOrderSubmit(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)

	if err := order.Submit(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if order.Status != OrderStatusPendingPayment {
		t.Errorf("expected pending_payment, got %s", order.Status)
	}
	if order.Version != 2 {
		t.Errorf("expected version 2, got %d", order.Version)
	}
}

func TestOrderSubmitFromNonDraft(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)
	order.Status = OrderStatusPaid

	if err := order.Submit(); err == nil {
		t.Error("expected error when submitting paid order")
	}
}

func TestOrderConfirmPayment(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)
	order.Submit()

	amount := NewMoney(10000, "USD")
	if err := order.ConfirmPayment("stripe", "pi-123", amount, now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if order.Status != OrderStatusPaid {
		t.Errorf("expected paid, got %s", order.Status)
	}
	if order.PaymentReference.Provider != "stripe" {
		t.Errorf("expected stripe, got %s", order.PaymentReference.Provider)
	}
	if order.PaymentReference.ProviderID != "pi-123" {
		t.Errorf("expected pi-123, got %s", order.PaymentReference.ProviderID)
	}
	if order.PaymentReference.Status != "authorized" {
		t.Errorf("expected authorized, got %s", order.PaymentReference.Status)
	}
	if order.PaymentReference.AuthorizedAt == nil {
		t.Error("expected authorized_at to be set")
	}
}

func TestOrderConfirmPaymentFromNonPending(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)

	amount := NewMoney(10000, "USD")
	if err := order.ConfirmPayment("stripe", "pi-123", amount, now); err == nil {
		t.Error("expected error when confirming payment on draft order")
	}
}

func TestOrderConfirm(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)
	order.Submit()
	order.ConfirmPayment("stripe", "pi-123", NewMoney(10000, "USD"), now)

	if err := order.Confirm(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if order.Status != OrderStatusConfirmed {
		t.Errorf("expected confirmed, got %s", order.Status)
	}
	if order.PaymentReference.Status != "captured" {
		t.Errorf("expected captured, got %s", order.PaymentReference.Status)
	}
}

func TestOrderConfirmFromNonPaid(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)

	if err := order.Confirm(now); err == nil {
		t.Error("expected error when confirming draft order")
	}
}

func TestOrderProcess(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)
	order.Submit()
	order.ConfirmPayment("stripe", "pi-123", NewMoney(10000, "USD"), now)
	order.Confirm(now)

	if err := order.Process(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != OrderStatusProcessing {
		t.Errorf("expected processing, got %s", order.Status)
	}
}

func TestOrderProcessFromNonConfirmed(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)

	if err := order.Process(now); err == nil {
		t.Error("expected error when processing draft order")
	}
}

func TestOrderReadyForFulfillment(t *testing.T) {
	now := newTestNow()
	order := createOrderToState(t, now, OrderStatusProcessing)

	if err := order.ReadyForFulfillment(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != OrderStatusReadyForFulfillment {
		t.Errorf("expected ready_for_fulfillment, got %s", order.Status)
	}
}

func TestOrderReadyForFulfillmentFromNonProcessing(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)

	if err := order.ReadyForFulfillment(now); err == nil {
		t.Error("expected error")
	}
}

func TestOrderFulfil(t *testing.T) {
	now := newTestNow()
	order := createOrderToState(t, now, OrderStatusReadyForFulfillment)

	if err := order.Fulfil(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != OrderStatusFulfilled {
		t.Errorf("expected fulfilled, got %s", order.Status)
	}
}

func TestOrderFulfilFromNonReady(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)

	if err := order.Fulfil(now); err == nil {
		t.Error("expected error")
	}
}

func TestOrderComplete(t *testing.T) {
	now := newTestNow()
	order := createFullOrder(t, now)
	order.Fulfil(now)

	if err := order.Complete(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != OrderStatusCompleted {
		t.Errorf("expected completed, got %s", order.Status)
	}
}

func TestOrderCompleteFromNonFulfilled(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)

	if err := order.Complete(now); err == nil {
		t.Error("expected error")
	}
}

func TestOrderCancelFromDraft(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)

	if err := order.Cancel(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != OrderStatusCancelled {
		t.Errorf("expected cancelled, got %s", order.Status)
	}
}

func TestOrderCancelFromPendingPayment(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)
	order.Submit()

	if err := order.Cancel(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != OrderStatusCancelled {
		t.Errorf("expected cancelled, got %s", order.Status)
	}
}

func TestOrderCancelFromPaid(t *testing.T) {
	now := newTestNow()
	order := createFullOrder(t, now)

	if err := order.Cancel(now); err == nil {
		t.Error("expected error when cancelling paid order")
	}
}

func TestOrderCancelFromTerminal(t *testing.T) {
	now := newTestNow()
	order := createFullOrder(t, now)
	order.Complete(now)

	if err := order.Cancel(now); err == nil {
		t.Error("expected error when cancelling completed order")
	}
}

func TestOrderRefundFromPaid(t *testing.T) {
	now := newTestNow()
	order := createOrderToState(t, now, OrderStatusPaid)

	if err := order.Refund(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != OrderStatusRefunded {
		t.Errorf("expected refunded, got %s", order.Status)
	}
}

func TestOrderRefundFromConfirmed(t *testing.T) {
	now := newTestNow()
	order := createOrderToState(t, now, OrderStatusConfirmed)

	if err := order.Refund(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != OrderStatusRefunded {
		t.Errorf("expected refunded, got %s", order.Status)
	}
}

func TestOrderRefundFromNonPaid(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)

	if err := order.Refund(now); err == nil {
		t.Error("expected error when refunding draft order")
	}
}

func TestOrderRefundFromFulfilled(t *testing.T) {
	now := newTestNow()
	order := createFullOrder(t, now)
	order.Fulfil(now)

	if err := order.Refund(now); err == nil {
		t.Error("expected error when refunding fulfilled order")
	}
}

func TestOrderExpireFromDraft(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)

	if err := order.Expire(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != OrderStatusExpired {
		t.Errorf("expected expired, got %s", order.Status)
	}
}

func TestOrderExpireFromPaid(t *testing.T) {
	now := newTestNow()
	order := createFullOrder(t, now)

	if err := order.Expire(now); err == nil {
		t.Error("expected error when expiring paid order")
	}
}

func TestOrderDisputeFromPaid(t *testing.T) {
	now := newTestNow()
	order := createFullOrder(t, now)

	if err := order.Dispute(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != OrderStatusDisputed {
		t.Errorf("expected disputed, got %s", order.Status)
	}
}

func TestOrderDisputeFromConfirmed(t *testing.T) {
	now := newTestNow()
	order := createFullOrder(t, now)

	if err := order.Dispute(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != OrderStatusDisputed {
		t.Errorf("expected disputed, got %s", order.Status)
	}
}

func TestOrderDisputeFromFulfilled(t *testing.T) {
	now := newTestNow()
	order := createFullOrder(t, now)
	order.Fulfil(now)

	if err := order.Dispute(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != OrderStatusDisputed {
		t.Errorf("expected disputed, got %s", order.Status)
	}
}

func TestOrderDisputeFromDraft(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)

	if err := order.Dispute(now); err == nil {
		t.Error("expected error when disputing draft order")
	}
}

func TestOrderDisputeFromCompleted(t *testing.T) {
	now := newTestNow()
	order := createFullOrder(t, now)
	order.Complete(now)

	if err := order.Dispute(now); err == nil {
		t.Error("expected error when disputing completed order")
	}
}

func TestOrderIsTerminal(t *testing.T) {
	tests := []struct {
		status   OrderStatus
		expected bool
	}{
		{OrderStatusDraft, false},
		{OrderStatusPendingPayment, false},
		{OrderStatusPaid, false},
		{OrderStatusConfirmed, false},
		{OrderStatusProcessing, false},
		{OrderStatusReadyForFulfillment, false},
		{OrderStatusFulfilled, false},
		{OrderStatusCompleted, true},
		{OrderStatusCancelled, true},
		{OrderStatusRefunded, true},
		{OrderStatusDisputed, true},
		{OrderStatusExpired, true},
	}

	for _, tt := range tests {
		order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", newTestNow())
		order.Status = tt.status

		if order.IsTerminal() != tt.expected {
			t.Errorf("IsTerminal() for %s: expected %v, got %v", tt.status, tt.expected, order.IsTerminal())
		}
	}
}

func TestOrderIsPaid(t *testing.T) {
	tests := []struct {
		status   OrderStatus
		expected bool
	}{
		{OrderStatusDraft, false},
		{OrderStatusPendingPayment, false},
		{OrderStatusPaid, true},
		{OrderStatusConfirmed, true},
		{OrderStatusProcessing, true},
		{OrderStatusReadyForFulfillment, true},
		{OrderStatusFulfilled, true},
		{OrderStatusCompleted, true},
		{OrderStatusCancelled, false},
		{OrderStatusRefunded, false},
		{OrderStatusDisputed, false},
		{OrderStatusExpired, false},
	}

	for _, tt := range tests {
		order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", newTestNow())
		order.Status = tt.status

		if order.IsPaid() != tt.expected {
			t.Errorf("IsPaid() for %s: expected %v, got %v", tt.status, tt.expected, order.IsPaid())
		}
	}
}

func TestOrderValidateTransition(t *testing.T) {
	tests := []struct {
		from      OrderStatus
		to        OrderStatus
		expectErr bool
	}{
		{OrderStatusDraft, OrderStatusPendingPayment, false},
		{OrderStatusPendingPayment, OrderStatusPaid, false},
		{OrderStatusPaid, OrderStatusConfirmed, false},
		{OrderStatusConfirmed, OrderStatusProcessing, false},
		{OrderStatusProcessing, OrderStatusReadyForFulfillment, false},
		{OrderStatusReadyForFulfillment, OrderStatusFulfilled, false},
		{OrderStatusFulfilled, OrderStatusCompleted, false},
		{OrderStatusDraft, OrderStatusCancelled, false},
		{OrderStatusPendingPayment, OrderStatusCancelled, false},
		{OrderStatusPaid, OrderStatusRefunded, false},
		{OrderStatusConfirmed, OrderStatusRefunded, false},
		{OrderStatusDraft, OrderStatusExpired, false},
		{OrderStatusPendingPayment, OrderStatusExpired, false},
		{OrderStatusPaid, OrderStatusDisputed, false},
		{OrderStatusConfirmed, OrderStatusDisputed, false},
		{OrderStatusFulfilled, OrderStatusDisputed, false},
		{OrderStatusDraft, OrderStatusPaid, true},
		{OrderStatusDraft, OrderStatusConfirmed, true},
		{OrderStatusPaid, OrderStatusCancelled, true},
		{OrderStatusCompleted, OrderStatusCancelled, true},
		{OrderStatusFulfilled, OrderStatusRefunded, true},
		{OrderStatusProcessing, OrderStatusCompleted, true},
	}

	for _, tt := range tests {
		order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", newTestNow())
		order.Status = tt.from

		err := order.ValidateTransition(tt.to)
		if (err != nil) != tt.expectErr {
			t.Errorf("ValidateTransition(%s -> %s): expected error=%v, got error=%v", tt.from, tt.to, tt.expectErr, err != nil)
		}
	}
}

func TestOrderValidateItems(t *testing.T) {
	now := newTestNow()
	t.Run("valid items", func(t *testing.T) {
		items := []*OrderItem{
			NewOrderItem("order-1", "listing-1", "Product", 1, NewMoney(5000, "USD"), now),
		}
		order := NewOrder("org-1", "seller-1", "customer-1", "store-1", items, "USD", now)
		if err := order.ValidateItems(); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("empty items", func(t *testing.T) {
		order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)
		if err := order.ValidateItems(); err == nil {
			t.Error("expected error for empty items")
		}
	})

	t.Run("negative quantity", func(t *testing.T) {
		now := newTestNow()
		item := NewOrderItem("order-1", "listing-1", "Product", -1, NewMoney(5000, "USD"), now)
		if err := item.Validate(); err == nil {
			t.Error("expected error for negative quantity")
		}
	})

	t.Run("zero quantity", func(t *testing.T) {
		now := newTestNow()
		item := NewOrderItem("order-1", "listing-1", "Product", 0, NewMoney(5000, "USD"), now)
		if err := item.Validate(); err == nil {
			t.Error("expected error for zero quantity")
		}
	})
}

func TestOrderItemTotalCalculation(t *testing.T) {
	now := newTestNow()
	item := NewOrderItem("order-1", "listing-1", "Product", 3, NewMoney(1500, "EUR"), now)

	if item.TotalPrice.AmountMinor != 4500 {
		t.Errorf("expected total 4500, got %d", item.TotalPrice.AmountMinor)
	}
	if item.TotalPrice.Currency != "EUR" {
		t.Errorf("expected EUR, got %s", item.TotalPrice.Currency)
	}
}

func TestPricingSnapshot(t *testing.T) {
	now := newTestNow()
	items := []OrderItem{
		*NewOrderItem("order-1", "listing-1", "Product A", 2, NewMoney(1000, "USD"), now),
		*NewOrderItem("order-1", "listing-2", "Product B", 1, NewMoney(500, "USD"), now),
	}
	discount := NewMoney(200, "USD")
	tax := NewMoney(260, "USD")
	snapshot := NewPricingSnapshot(items, discount, tax, now)

	expectedTotal := int64(2000 + 500 - 200 + 260)
	if snapshot.TotalAmount.AmountMinor != expectedTotal {
		t.Errorf("expected total %d, got %d", expectedTotal, snapshot.TotalAmount.AmountMinor)
	}
	if snapshot.Currency != "USD" {
		t.Errorf("expected USD, got %s", snapshot.Currency)
	}
	if len(snapshot.ItemizedBreakdown) != 2 {
		t.Errorf("expected 2 items, got %d", len(snapshot.ItemizedBreakdown))
	}
	if snapshot.SnapshotAt != now {
		t.Errorf("expected snapshot_at %d, got %d", now, snapshot.SnapshotAt)
	}
}

func TestMoneyAdd(t *testing.T) {
	m1 := NewMoney(5000, "USD")
	m2 := NewMoney(3000, "USD")
	result, err := m1.Add(m2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AmountMinor != 8000 {
		t.Errorf("expected 8000, got %d", result.AmountMinor)
	}
}

func TestMoneyAddDifferentCurrency(t *testing.T) {
	m1 := NewMoney(5000, "USD")
	m2 := NewMoney(3000, "EUR")
	_, err := m1.Add(m2)
	if err == nil {
		t.Error("expected error adding different currencies")
	}
}

func TestMoneySubtract(t *testing.T) {
	m1 := NewMoney(5000, "USD")
	m2 := NewMoney(2000, "USD")
	result, err := m1.Subtract(m2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AmountMinor != 3000 {
		t.Errorf("expected 3000, got %d", result.AmountMinor)
	}
}

func TestMoneySubtractDifferentCurrency(t *testing.T) {
	m1 := NewMoney(5000, "USD")
	m2 := NewMoney(2000, "EUR")
	_, err := m1.Subtract(m2)
	if err == nil {
		t.Error("expected error subtracting different currencies")
	}
}

func TestCustomerReference(t *testing.T) {
	ref := NewCustomerReference("cust-1", "+243999999999", "John Doe")
	if ref.ID != "cust-1" {
		t.Errorf("expected cust-1, got %s", ref.ID)
	}
	if ref.Phone != "+243999999999" {
		t.Errorf("expected phone, got %s", ref.Phone)
	}
	if ref.DisplayName != "John Doe" {
		t.Errorf("expected John Doe, got %s", ref.DisplayName)
	}
}

func TestSellerReference(t *testing.T) {
	ref := NewSellerReference("seller-1", "org-1", "Tech Store")
	if ref.ID != "seller-1" {
		t.Errorf("expected seller-1, got %s", ref.ID)
	}
	if ref.OrganizationID != "org-1" {
		t.Errorf("expected org-1, got %s", ref.OrganizationID)
	}
	if ref.DisplayName != "Tech Store" {
		t.Errorf("expected Tech Store, got %s", ref.DisplayName)
	}
}

func TestPaymentReference(t *testing.T) {
	ref := NewPaymentReference("stripe", "pi-123", NewMoney(10000, "USD"))
	if ref.Provider != "stripe" {
		t.Errorf("expected stripe, got %s", ref.Provider)
	}
	if ref.ProviderID != "pi-123" {
		t.Errorf("expected pi-123, got %s", ref.ProviderID)
	}
	if ref.Status != "pending" {
		t.Errorf("expected pending, got %s", ref.Status)
	}
	if ref.Amount.AmountMinor != 10000 {
		t.Errorf("expected 10000, got %d", ref.Amount.AmountMinor)
	}
}

func TestDeliveryReference(t *testing.T) {
	ref := NewDeliveryReference("fedex", "fed-123")
	if ref.Provider != "fedex" {
		t.Errorf("expected fedex, got %s", ref.Provider)
	}
	if ref.ProviderID != "fed-123" {
		t.Errorf("expected fed-123, got %s", ref.ProviderID)
	}
	if ref.Status != "unassigned" {
		t.Errorf("expected unassigned, got %s", ref.Status)
	}
}

func TestOrderVersionIncrement(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)
	initialVersion := order.Version

	order.Submit()
	if order.Version != initialVersion+1 {
		t.Errorf("expected version %d, got %d", initialVersion+1, order.Version)
	}

	order.ConfirmPayment("stripe", "pi-1", NewMoney(10000, "USD"), now)
	if order.Version != initialVersion+2 {
		t.Errorf("expected version %d, got %d", initialVersion+2, order.Version)
	}
}

func TestOrderRecalculateTotal(t *testing.T) {
	now := newTestNow()
	items := []*OrderItem{
		NewOrderItem("order-1", "listing-1", "A", 2, NewMoney(1000, "USD"), now),
		NewOrderItem("order-1", "listing-2", "B", 3, NewMoney(500, "USD"), now),
	}
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", items, "USD", now)

	if order.TotalAmount.AmountMinor != 3500 {
		t.Errorf("expected total 3500, got %d", order.TotalAmount.AmountMinor)
	}
}

func TestOrderSetCustomer(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)
	ref := NewCustomerReference("cust-2", "+243888888888", "Jane Doe")
	order.SetCustomer(ref)

	if order.Customer.ID != "cust-2" {
		t.Errorf("expected cust-2, got %s", order.Customer.ID)
	}
}

func TestOrderSetSeller(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)
	ref := NewSellerReference("seller-2", "org-2", "Another Store")
	order.SetSeller(ref)

	if order.Seller.ID != "seller-2" {
		t.Errorf("expected seller-2, got %s", order.Seller.ID)
	}
}

func TestOrderSetDelivery(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)
	ref := NewDeliveryReference("dhl", "dhl-1")
	order.SetDelivery(ref)

	if order.DeliveryReference.Provider != "dhl" {
		t.Errorf("expected dhl, got %s", order.DeliveryReference.Provider)
	}
}

func TestOrderValidateTransitionFromTerminal(t *testing.T) {
	now := newTestNow()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)
	order.Status = OrderStatusCompleted

	if err := order.ValidateTransition(OrderStatusCancelled); err == nil {
		t.Error("expected error: cannot transition from terminal status")
	}
}

// Helper to create an order progressed to a specific status.
func createOrderToState(t *testing.T, now int64, target OrderStatus) *Order {
	t.Helper()
	order := NewOrder("org-1", "seller-1", "customer-1", "store-1", nil, "USD", now)
	order.Submit()
	order.ConfirmPayment("stripe", "pi-123", NewMoney(10000, "USD"), now)
	if target == OrderStatusPaid {
		return order
	}
	order.Confirm(now)
	if target == OrderStatusConfirmed {
		return order
	}
	order.Process(now)
	if target == OrderStatusProcessing {
		return order
	}
	order.ReadyForFulfillment(now)
	if target == OrderStatusReadyForFulfillment {
		return order
	}
	order.Fulfil(now)
	if target == OrderStatusFulfilled {
		return order
	}
	order.Complete(now)
	return order
}

// Helper to create an order that has progressed through the full lifecycle.
func createFullOrder(t *testing.T, now int64) *Order {
	return createOrderToState(t, now, OrderStatusFulfilled)
}
