package domain

import (
	"fmt"
	"time"
)

// OrderStatus represents the lifecycle state of an order.
type OrderStatus string

const (
	OrderStatusDraft               OrderStatus = "draft"
	OrderStatusPendingPayment      OrderStatus = "pending_payment"
	OrderStatusPaid                OrderStatus = "paid"
	OrderStatusConfirmed           OrderStatus = "confirmed"
	OrderStatusProcessing          OrderStatus = "processing"
	OrderStatusReadyForFulfillment OrderStatus = "ready_for_fulfillment"
	OrderStatusFulfilled           OrderStatus = "fulfilled"
	OrderStatusCompleted           OrderStatus = "completed"
	OrderStatusCancelled           OrderStatus = "cancelled"
	OrderStatusRefunded            OrderStatus = "refunded"
	OrderStatusDisputed            OrderStatus = "disputed"
	OrderStatusExpired             OrderStatus = "expired"
)

// Money represents an amount in minor currency units with explicit currency.
type Money struct {
	AmountMinor int64
	Currency    string
}

// NewMoney creates a new Money value.
func NewMoney(amountMinor int64, currency string) Money {
	return Money{
		AmountMinor: amountMinor,
		Currency:    currency,
	}
}

// Add returns the sum of two Money values in the same currency.
func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("cannot add money with different currencies: %s + %s", m.Currency, other.Currency)
	}
	return Money{
		AmountMinor: m.AmountMinor + other.AmountMinor,
		Currency:    m.Currency,
	}, nil
}

// Subtract returns the difference of two Money values in the same currency.
func (m Money) Subtract(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("cannot subtract money with different currencies: %s - %s", m.Currency, other.Currency)
	}
	return Money{
		AmountMinor: m.AmountMinor - other.AmountMinor,
		Currency:    m.Currency,
	}, nil
}

// OrderItem represents a single line item in an order.
type OrderItem struct {
	ID         string
	OrderID    string
	ListingID  string
	Title      string
	Quantity   int
	UnitPrice  Money
	TotalPrice Money
	SnapshotAt int64
}

// NewOrderItem creates a new OrderItem with calculated total price.
func NewOrderItem(orderID, listingID, title string, quantity int, unitPrice Money, now int64) *OrderItem {
	totalAmount := unitPrice.AmountMinor * int64(quantity)
	return &OrderItem{
		ID:         "",
		OrderID:    orderID,
		ListingID:  listingID,
		Title:      title,
		Quantity:   quantity,
		UnitPrice:  unitPrice,
		TotalPrice: Money{AmountMinor: totalAmount, Currency: unitPrice.Currency},
		SnapshotAt: now,
	}
}

// Validate returns an error if the item is invalid.
func (o *OrderItem) Validate() error {
	if o.Quantity <= 0 {
		return fmt.Errorf("order item quantity must be positive")
	}
	if o.UnitPrice.AmountMinor < 0 {
		return fmt.Errorf("order item unit price must not be negative")
	}
	if o.UnitPrice.Currency == "" {
		return fmt.Errorf("order item unit price currency is required")
	}
	return nil
}

// PricingSnapshot represents a frozen view of pricing at order time.
type PricingSnapshot struct {
	AmountMinor       int64
	Currency          string
	ItemizedBreakdown []OrderItem
	DiscountAmount    Money
	TaxAmount         Money
	TotalAmount       Money
	SnapshotAt        int64
}

// NewPricingSnapshot creates a pricing snapshot from items.
func NewPricingSnapshot(items []OrderItem, discount, tax Money, now int64) *PricingSnapshot {
	var subtotal int64
	currency := ""
	for _, item := range items {
		subtotal += item.TotalPrice.AmountMinor
		if currency == "" {
			currency = item.TotalPrice.Currency
		}
	}
	total := subtotal - discount.AmountMinor + tax.AmountMinor
	if total < 0 {
		total = 0
	}
	return &PricingSnapshot{
		AmountMinor:       total,
		Currency:          currency,
		ItemizedBreakdown: items,
		DiscountAmount:    discount,
		TaxAmount:         tax,
		TotalAmount: Money{
			AmountMinor: total,
			Currency:    currency,
		},
		SnapshotAt: now,
	}
}

// CustomerReference identifies the customer who placed the order.
type CustomerReference struct {
	ID          string
	Phone       string
	DisplayName string
}

// NewCustomerReference creates a customer reference.
func NewCustomerReference(id, phone, displayName string) CustomerReference {
	return CustomerReference{
		ID:          id,
		Phone:       phone,
		DisplayName: displayName,
	}
}

// SellerReference identifies the seller whose listing was ordered.
type SellerReference struct {
	ID             string
	OrganizationID string
	DisplayName    string
}

// NewSellerReference creates a seller reference.
func NewSellerReference(id, organizationID, displayName string) SellerReference {
	return SellerReference{
		ID:             id,
		OrganizationID: organizationID,
		DisplayName:    displayName,
	}
}

// PaymentReference holds payment-related information for an order.
type PaymentReference struct {
	Provider     string
	ProviderID   string
	Status       string
	AuthorizedAt *int64
	CapturedAt   *int64
	Amount       Money
}

// NewPaymentReference creates a payment reference.
func NewPaymentReference(provider, providerID string, amount Money) PaymentReference {
	return PaymentReference{
		Provider:   provider,
		ProviderID: providerID,
		Status:     "pending",
		Amount:     amount,
	}
}

// DeliveryReference holds delivery-related information for an order.
type DeliveryReference struct {
	Provider    string
	ProviderID  string
	Status      string
	TrackingNo  string
	EstimatedAt *int64
	DeliveredAt *int64
}

// NewDeliveryReference creates a delivery reference.
func NewDeliveryReference(provider, providerID string) DeliveryReference {
	return DeliveryReference{
		Provider:   provider,
		ProviderID: providerID,
		Status:     "unassigned",
	}
}

// Order represents a purchase/reservation/booking intent accepted by Trezutil.
type Order struct {
	ID                string
	OrganizationID    string
	SellerID          string
	CustomerID        string
	StoreID           string
	Items             []*OrderItem
	TotalAmount       Money
	Currency          string
	Status            OrderStatus
	PricingSnapshot   *PricingSnapshot
	PaymentReference  PaymentReference
	DeliveryReference DeliveryReference
	Customer          CustomerReference
	Seller            SellerReference
	CreatedAt         int64
	UpdatedAt         int64
	Version           int64
}

// NewOrder creates a new Order in DRAFT status.
func NewOrder(organizationID, sellerID, customerID, storeID string, items []*OrderItem, currency string, now int64) *Order {
	order := &Order{
		ID:             "",
		OrganizationID: organizationID,
		SellerID:       sellerID,
		CustomerID:     customerID,
		StoreID:        storeID,
		Items:          items,
		Currency:       currency,
		Status:         OrderStatusDraft,
		CreatedAt:      now,
		UpdatedAt:      now,
		Version:        1,
	}
	order.recalculateTotal()
	return order
}

// recalculateTotal recalculates the order total from items.
func (o *Order) recalculateTotal() {
	var total int64
	for _, item := range o.Items {
		total += item.TotalPrice.AmountMinor
	}
	o.TotalAmount = Money{AmountMinor: total, Currency: o.Currency}
}

// IsTerminal returns true if the order is in a terminal state.
func (o *Order) IsTerminal() bool {
	switch o.Status {
	case OrderStatusCancelled, OrderStatusRefunded, OrderStatusDisputed, OrderStatusExpired, OrderStatusCompleted:
		return true
	default:
		return false
	}
}

// IsPaid returns true if the order has been paid.
func (o *Order) IsPaid() bool {
	return o.Status == OrderStatusPaid || o.Status == OrderStatusConfirmed ||
		o.Status == OrderStatusProcessing || o.Status == OrderStatusReadyForFulfillment ||
		o.Status == OrderStatusFulfilled || o.Status == OrderStatusCompleted
}

// Submit transitions the order from DRAFT to PENDING_PAYMENT.
// Valid from: DRAFT only.
func (o *Order) Submit() error {
	if o.Status != OrderStatusDraft {
		return fmt.Errorf("cannot submit order from status %s: only draft orders can be submitted", o.Status)
	}
	now := time.Now().Unix()
	o.Status = OrderStatusPendingPayment
	o.UpdatedAt = now
	o.Version++
	return nil
}

// ConfirmPayment transitions the order from PENDING_PAYMENT to PAID.
// Valid from: PENDING_PAYMENT only.
func (o *Order) ConfirmPayment(provider, providerID string, amount Money, now int64) error {
	if o.Status != OrderStatusPendingPayment {
		return fmt.Errorf("cannot confirm payment from status %s: only pending_payment orders can be paid", o.Status)
	}
	o.PaymentReference = NewPaymentReference(provider, providerID, amount)
	o.PaymentReference.Status = "authorized"
	o.PaymentReference.AuthorizedAt = &now
	o.Status = OrderStatusPaid
	o.UpdatedAt = now
	o.Version++
	return nil
}

// Confirm transitions the order from PAID to CONFIRMED.
// Valid from: PAID only.
func (o *Order) Confirm(now int64) error {
	if o.Status != OrderStatusPaid {
		return fmt.Errorf("cannot confirm order from status %s: only paid orders can be confirmed", o.Status)
	}
	o.Status = OrderStatusConfirmed
	o.PaymentReference.Status = "captured"
	if o.PaymentReference.CapturedAt == nil {
		o.PaymentReference.CapturedAt = &now
	}
	o.UpdatedAt = now
	o.Version++
	return nil
}

// Process transitions the order from CONFIRMED to PROCESSING.
// Valid from: CONFIRMED only.
func (o *Order) Process(now int64) error {
	if o.Status != OrderStatusConfirmed {
		return fmt.Errorf("cannot process order from status %s: only confirmed orders can be processed", o.Status)
	}
	o.Status = OrderStatusProcessing
	o.UpdatedAt = now
	o.Version++
	return nil
}

// ReadyForFulfillment transitions the order from PROCESSING to READY_FOR_FULFILLMENT.
// Valid from: PROCESSING only.
func (o *Order) ReadyForFulfillment(now int64) error {
	if o.Status != OrderStatusProcessing {
		return fmt.Errorf("cannot set ready for fulfillment from status %s: only processing orders can be ready", o.Status)
	}
	o.Status = OrderStatusReadyForFulfillment
	o.UpdatedAt = now
	o.Version++
	return nil
}

// Fulfil transitions the order from READY_FOR_FULFILLMENT to FULFILLED.
// Valid from: READY_FOR_FULFILLMENT only.
func (o *Order) Fulfil(now int64) error {
	if o.Status != OrderStatusReadyForFulfillment {
		return fmt.Errorf("cannot fulfil order from status %s: only ready_for_fulfillment orders can be fulfilled", o.Status)
	}
	o.Status = OrderStatusFulfilled
	o.UpdatedAt = now
	o.Version++
	return nil
}

// Complete transitions the order from FULFILLED to COMPLETED.
// Valid from: FULFILLED only.
func (o *Order) Complete(now int64) error {
	if o.Status != OrderStatusFulfilled {
		return fmt.Errorf("cannot complete order from status %s: only fulfilled orders can be completed", o.Status)
	}
	o.Status = OrderStatusCompleted
	o.UpdatedAt = now
	o.Version++
	return nil
}

// Cancel transitions the order to CANCELLED.
// Valid from: DRAFT, PENDING_PAYMENT.
func (o *Order) Cancel(now int64) error {
	switch o.Status {
	case OrderStatusDraft, OrderStatusPendingPayment:
		o.Status = OrderStatusCancelled
	case OrderStatusPaid, OrderStatusConfirmed, OrderStatusProcessing,
		OrderStatusReadyForFulfillment, OrderStatusFulfilled, OrderStatusCompleted:
		return fmt.Errorf("cannot cancel order in status %s: paid or advanced orders cannot be cancelled; use refund instead", o.Status)
	case OrderStatusCancelled, OrderStatusRefunded, OrderStatusDisputed, OrderStatusExpired:
		return fmt.Errorf("cannot cancel order in terminal status %s", o.Status)
	default:
		return fmt.Errorf("cannot cancel order from status %s: invalid source state", o.Status)
	}
	o.UpdatedAt = now
	o.Version++
	return nil
}

// Refund transitions the order to REFUNDED.
// Valid from: PAID, CONFIRMED.
func (o *Order) Refund(now int64) error {
	switch o.Status {
	case OrderStatusPaid, OrderStatusConfirmed:
		o.Status = OrderStatusRefunded
	case OrderStatusDraft, OrderStatusPendingPayment:
		return fmt.Errorf("cannot refund order in status %s: order has not been paid", o.Status)
	case OrderStatusProcessing, OrderStatusReadyForFulfillment, OrderStatusFulfilled, OrderStatusCompleted:
		return fmt.Errorf("cannot refund order in status %s: order has been fulfilled; use dispute instead", o.Status)
	case OrderStatusCancelled, OrderStatusRefunded, OrderStatusDisputed, OrderStatusExpired:
		return fmt.Errorf("cannot refund order in terminal status %s", o.Status)
	default:
		return fmt.Errorf("cannot refund order from status %s: invalid source state", o.Status)
	}
	o.UpdatedAt = now
	o.Version++
	return nil
}

// Expire transitions the order to EXPIRED.
// Valid from: DRAFT, PENDING_PAYMENT.
func (o *Order) Expire(now int64) error {
	switch o.Status {
	case OrderStatusDraft, OrderStatusPendingPayment:
		o.Status = OrderStatusExpired
	case OrderStatusPaid, OrderStatusConfirmed, OrderStatusProcessing,
		OrderStatusReadyForFulfillment, OrderStatusFulfilled, OrderStatusCompleted,
		OrderStatusCancelled, OrderStatusRefunded, OrderStatusDisputed:
		return fmt.Errorf("cannot expire order in status %s", o.Status)
	default:
		return fmt.Errorf("cannot expire order from status %s: invalid source state", o.Status)
	}
	o.UpdatedAt = now
	o.Version++
	return nil
}

// Dispute transitions the order to DISPUTED.
// Valid from: PAID, CONFIRMED, FULFILLED.
func (o *Order) Dispute(now int64) error {
	switch o.Status {
	case OrderStatusPaid, OrderStatusConfirmed, OrderStatusFulfilled:
		o.Status = OrderStatusDisputed
	case OrderStatusDraft, OrderStatusPendingPayment:
		return fmt.Errorf("cannot dispute order in status %s: order has not been paid", o.Status)
	case OrderStatusProcessing, OrderStatusReadyForFulfillment:
		return fmt.Errorf("cannot dispute order in status %s: order is still in progress", o.Status)
	case OrderStatusCompleted, OrderStatusCancelled, OrderStatusRefunded, OrderStatusExpired:
		return fmt.Errorf("cannot dispute order in terminal status %s", o.Status)
	default:
		return fmt.Errorf("cannot dispute order from status %s: invalid source state", o.Status)
	}
	o.UpdatedAt = now
	o.Version++
	return nil
}

// ValidateTransition validates a status transition without applying it.
func (o *Order) ValidateTransition(newStatus OrderStatus) error {
	current := o.Status

	if current == newStatus {
		return nil
	}

	if o.IsTerminal() {
		return fmt.Errorf("cannot transition from terminal status %s", current)
	}

	switch newStatus {
	case OrderStatusPendingPayment:
		if current != OrderStatusDraft {
			return fmt.Errorf("only draft orders can be submitted")
		}
	case OrderStatusPaid:
		if current != OrderStatusPendingPayment {
			return fmt.Errorf("only pending_payment orders can be paid")
		}
	case OrderStatusConfirmed:
		if current != OrderStatusPaid {
			return fmt.Errorf("only paid orders can be confirmed")
		}
	case OrderStatusProcessing:
		if current != OrderStatusConfirmed {
			return fmt.Errorf("only confirmed orders can be processed")
		}
	case OrderStatusReadyForFulfillment:
		if current != OrderStatusProcessing {
			return fmt.Errorf("only processing orders can be ready for fulfillment")
		}
	case OrderStatusFulfilled:
		if current != OrderStatusReadyForFulfillment {
			return fmt.Errorf("only ready_for_fulfillment orders can be fulfilled")
		}
	case OrderStatusCompleted:
		if current != OrderStatusFulfilled {
			return fmt.Errorf("only fulfilled orders can be completed")
		}
	case OrderStatusCancelled:
		if current != OrderStatusDraft && current != OrderStatusPendingPayment {
			return fmt.Errorf("only draft or pending_payment orders can be cancelled")
		}
	case OrderStatusRefunded:
		if current != OrderStatusPaid && current != OrderStatusConfirmed {
			return fmt.Errorf("only paid or confirmed orders can be refunded")
		}
	case OrderStatusExpired:
		if current != OrderStatusDraft && current != OrderStatusPendingPayment {
			return fmt.Errorf("only draft or pending_payment orders can expire")
		}
	case OrderStatusDisputed:
		if current != OrderStatusPaid && current != OrderStatusConfirmed && current != OrderStatusFulfilled {
			return fmt.Errorf("only paid, confirmed, or fulfilled orders can be disputed")
		}
	default:
		return fmt.Errorf("unknown target status: %s", newStatus)
	}

	return nil
}

// ValidateItems validates all order items.
func (o *Order) ValidateItems() error {
	if len(o.Items) == 0 {
		return fmt.Errorf("order must have at least one item")
	}
	for _, item := range o.Items {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("invalid order item: %w", err)
		}
	}
	return nil
}

// SetCustomer sets the customer reference.
func (o *Order) SetCustomer(ref CustomerReference) {
	o.Customer = ref
}

// SetSeller sets the seller reference.
func (o *Order) SetSeller(ref SellerReference) {
	o.Seller = ref
}

// SetDelivery sets the delivery reference.
func (o *Order) SetDelivery(ref DeliveryReference) {
	o.DeliveryReference = ref
}
