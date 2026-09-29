package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// orderRow represents a row from the orders table.
type orderRow struct {
	ID                    string
	OrganizationID        string
	SellerID              string
	CustomerID            string
	StoreID               string
	ItemsJSON             []byte
	TotalAmount           int64
	Currency              string
	Status                string
	PricingSnapshotJSON   []byte
	PaymentReferenceJSON  []byte
	DeliveryReferenceJSON []byte
	CustomerReferenceJSON []byte
	SellerReferenceJSON   []byte
	Version               int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// OrderRepository implements domain.OrderRepository using PostgreSQL.
type OrderRepository struct {
	db *sql.DB
}

// NewOrderRepository creates a new OrderRepository.
func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// Create inserts a new order into the database.
func (r *OrderRepository) Create(ctx context.Context, order *domain.Order) error {
	itemsJSON, err := json.Marshal(order.Items)
	if err != nil {
		return fmt.Errorf("failed to marshal order items: %w", err)
	}

	pricingJSON, err := json.Marshal(order.PricingSnapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal pricing snapshot: %w", err)
	}

	paymentJSON, err := json.Marshal(order.PaymentReference)
	if err != nil {
		return fmt.Errorf("failed to marshal payment reference: %w", err)
	}

	deliveryJSON, err := json.Marshal(order.DeliveryReference)
	if err != nil {
		return fmt.Errorf("failed to marshal delivery reference: %w", err)
	}

	customerJSON, err := json.Marshal(order.Customer)
	if err != nil {
		return fmt.Errorf("failed to marshal customer reference: %w", err)
	}

	sellerJSON, err := json.Marshal(order.Seller)
	if err != nil {
		return fmt.Errorf("failed to marshal seller reference: %w", err)
	}

	query := `INSERT INTO orders (id, organization_id, seller_id, customer_id, store_id, items, total_amount, currency, status, pricing_snapshot, payment_reference, delivery_reference, customer_reference, seller_reference, version, created_at, updated_at, idempotency_key)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`

	var idem sql.NullString
	if order.IdempotencyKey != "" {
		idem = sql.NullString{String: order.IdempotencyKey, Valid: true}
	}

	_, err = r.db.ExecContext(ctx, query,
		order.ID, order.OrganizationID, order.SellerID, order.CustomerID, order.StoreID,
		string(itemsJSON), order.TotalAmount.AmountMinor, order.Currency, string(order.Status),
		string(pricingJSON), string(paymentJSON), string(deliveryJSON), string(customerJSON), string(sellerJSON),
		order.Version, toTime(order.CreatedAt), toTime(order.UpdatedAt), idem,
	)
	if err != nil {
		if isUniqueViolation(err) && order.IdempotencyKey != "" {
			return domain.ErrIdempotencyConflict
		}
		return fmt.Errorf("failed to create order: %w", err)
	}
	return nil
}

// FindByCustomerIdempotencyKey retrieves the order a customer created with the given key.
func (r *OrderRepository) FindByCustomerIdempotencyKey(ctx context.Context, customerID, idempotencyKey string) (*domain.Order, error) {
	query := `SELECT id, organization_id, seller_id, customer_id, store_id, items, total_amount, currency, status, pricing_snapshot, payment_reference, delivery_reference, customer_reference, seller_reference, version, created_at, updated_at
	          FROM orders WHERE customer_id = $1 AND idempotency_key = $2`
	var u orderRow
	err := r.db.QueryRowContext(ctx, query, customerID, idempotencyKey).Scan(
		&u.ID, &u.OrganizationID, &u.SellerID, &u.CustomerID, &u.StoreID,
		&u.ItemsJSON, &u.TotalAmount, &u.Currency, &u.Status,
		&u.PricingSnapshotJSON, &u.PaymentReferenceJSON, &u.DeliveryReferenceJSON,
		&u.CustomerReferenceJSON, &u.SellerReferenceJSON,
		&u.Version, &u.CreatedAt, &u.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find order by idempotency key: %w", err)
	}
	order := unmarshalOrder(u)
	order.IdempotencyKey = idempotencyKey
	return order, nil
}

// FindByID retrieves an order by ID.
func (r *OrderRepository) FindByID(ctx context.Context, orderID string) (*domain.Order, error) {
	query := `SELECT id, organization_id, seller_id, customer_id, store_id, items, total_amount, currency, status, pricing_snapshot, payment_reference, delivery_reference, customer_reference, seller_reference, version, created_at, updated_at
	          FROM orders WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, orderID)

	var u orderRow
	err := row.Scan(
		&u.ID, &u.OrganizationID, &u.SellerID, &u.CustomerID, &u.StoreID,
		&u.ItemsJSON, &u.TotalAmount, &u.Currency, &u.Status,
		&u.PricingSnapshotJSON, &u.PaymentReferenceJSON, &u.DeliveryReferenceJSON,
		&u.CustomerReferenceJSON, &u.SellerReferenceJSON,
		&u.Version, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("order not found: %s", orderID)
		}
		return nil, fmt.Errorf("failed to find order: %w", err)
	}

	return unmarshalOrder(u), nil
}

// FindByIDForUpdate retrieves an order by ID with row-level lock.
func (r *OrderRepository) FindByIDForUpdate(ctx context.Context, orderID string) (*domain.Order, error) {
	query := `SELECT id, organization_id, seller_id, customer_id, store_id, items, total_amount, currency, status, pricing_snapshot, payment_reference, delivery_reference, customer_reference, seller_reference, version, created_at, updated_at
	          FROM orders WHERE id = $1 FOR UPDATE`
	row := r.db.QueryRowContext(ctx, query, orderID)

	var u orderRow
	err := row.Scan(
		&u.ID, &u.OrganizationID, &u.SellerID, &u.CustomerID, &u.StoreID,
		&u.ItemsJSON, &u.TotalAmount, &u.Currency, &u.Status,
		&u.PricingSnapshotJSON, &u.PaymentReferenceJSON, &u.DeliveryReferenceJSON,
		&u.CustomerReferenceJSON, &u.SellerReferenceJSON,
		&u.Version, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("order not found: %s", orderID)
		}
		return nil, fmt.Errorf("failed to find order for update: %w", err)
	}

	return unmarshalOrder(u), nil
}

// FindByIDAndOrg retrieves an order by ID, scoped to the given organization.
// The tenant filter enforces tenant isolation at the DB level (IDOR protection).
func (r *OrderRepository) FindByIDAndOrg(ctx context.Context, orgID, orderID string) (*domain.Order, error) {
	query := `SELECT id, organization_id, seller_id, customer_id, store_id, items, total_amount, currency, status, pricing_snapshot, payment_reference, delivery_reference, customer_reference, seller_reference, version, created_at, updated_at
	          FROM orders WHERE id = $1 AND organization_id = $2`
	row := r.db.QueryRowContext(ctx, query, orderID, orgID)

	var u orderRow
	err := row.Scan(
		&u.ID, &u.OrganizationID, &u.SellerID, &u.CustomerID, &u.StoreID,
		&u.ItemsJSON, &u.TotalAmount, &u.Currency, &u.Status,
		&u.PricingSnapshotJSON, &u.PaymentReferenceJSON, &u.DeliveryReferenceJSON,
		&u.CustomerReferenceJSON, &u.SellerReferenceJSON,
		&u.Version, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("order not found: %s", orderID)
		}
		return nil, fmt.Errorf("failed to find order: %w", err)
	}

	return unmarshalOrder(u), nil
}

// Update updates an existing order in the database.
func (r *OrderRepository) Update(ctx context.Context, order *domain.Order) error {
	itemsJSON, err := json.Marshal(order.Items)
	if err != nil {
		return fmt.Errorf("failed to marshal order items: %w", err)
	}

	pricingJSON, err := json.Marshal(order.PricingSnapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal pricing snapshot: %w", err)
	}

	paymentJSON, err := json.Marshal(order.PaymentReference)
	if err != nil {
		return fmt.Errorf("failed to marshal payment reference: %w", err)
	}

	deliveryJSON, err := json.Marshal(order.DeliveryReference)
	if err != nil {
		return fmt.Errorf("failed to marshal delivery reference: %w", err)
	}

	customerJSON, err := json.Marshal(order.Customer)
	if err != nil {
		return fmt.Errorf("failed to marshal customer reference: %w", err)
	}

	sellerJSON, err := json.Marshal(order.Seller)
	if err != nil {
		return fmt.Errorf("failed to marshal seller reference: %w", err)
	}

	query := `UPDATE orders SET
	          items = $1, total_amount = $2, currency = $3, status = $4, pricing_snapshot = $5,
	          payment_reference = $6, delivery_reference = $7, customer_reference = $8,
	          seller_reference = $9, version = $10, updated_at = $11
	          WHERE id = $12 AND version = $13`

	result, err := r.db.ExecContext(ctx, query,
		string(itemsJSON), order.TotalAmount.AmountMinor, order.Currency, string(order.Status),
		string(pricingJSON), string(paymentJSON), string(deliveryJSON), string(customerJSON), string(sellerJSON),
		order.Version, toTime(order.UpdatedAt), order.ID, order.Version-1,
	)
	if err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		var exists bool
		if err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM orders WHERE id = $1)`, order.ID).Scan(&exists); err == nil && exists {
			return domain.ErrOptimisticLock
		}
		return fmt.Errorf("order not found: %s", order.ID)
	}

	return nil
}

func unmarshalOrder(u orderRow) *domain.Order {
	order := &domain.Order{
		ID:             u.ID,
		OrganizationID: u.OrganizationID,
		SellerID:       u.SellerID,
		CustomerID:     u.CustomerID,
		StoreID:        u.StoreID,
		TotalAmount:    domain.Money{AmountMinor: u.TotalAmount, Currency: u.Currency},
		Currency:       u.Currency,
		Status:         domain.OrderStatus(u.Status),
		Version:        u.Version,
		CreatedAt:      toUnix(u.CreatedAt),
		UpdatedAt:      toUnix(u.UpdatedAt),
	}

	if len(u.ItemsJSON) > 0 && string(u.ItemsJSON) != "null" {
		var items []*domain.OrderItem
		if err := json.Unmarshal(u.ItemsJSON, &items); err == nil {
			order.Items = items
		}
	}

	if len(u.PricingSnapshotJSON) > 0 && string(u.PricingSnapshotJSON) != "null" {
		var ps domain.PricingSnapshot
		if err := json.Unmarshal(u.PricingSnapshotJSON, &ps); err == nil {
			order.PricingSnapshot = &ps
		}
	}

	if len(u.PaymentReferenceJSON) > 0 && string(u.PaymentReferenceJSON) != "null" {
		var pr domain.PaymentReference
		if err := json.Unmarshal(u.PaymentReferenceJSON, &pr); err == nil {
			order.PaymentReference = pr
		}
	}

	if len(u.DeliveryReferenceJSON) > 0 && string(u.DeliveryReferenceJSON) != "null" {
		var dr domain.DeliveryReference
		if err := json.Unmarshal(u.DeliveryReferenceJSON, &dr); err == nil {
			order.DeliveryReference = dr
		}
	}

	if len(u.CustomerReferenceJSON) > 0 && string(u.CustomerReferenceJSON) != "null" {
		var cr domain.CustomerReference
		if err := json.Unmarshal(u.CustomerReferenceJSON, &cr); err == nil {
			order.Customer = cr
		}
	}

	if len(u.SellerReferenceJSON) > 0 && string(u.SellerReferenceJSON) != "null" {
		var sr domain.SellerReference
		if err := json.Unmarshal(u.SellerReferenceJSON, &sr); err == nil {
			order.Seller = sr
		}
	}

	if order.Items == nil {
		order.Items = []*domain.OrderItem{}
	}

	return order
}

// FindByCustomerID lists a customer's orders, newest first.
func (r *OrderRepository) FindByCustomerID(ctx context.Context, customerID string, limit, offset int) ([]*domain.Order, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, organization_id, seller_id, customer_id, store_id, items, total_amount, currency, status, pricing_snapshot, payment_reference, delivery_reference, customer_reference, seller_reference, version, created_at, updated_at
	          FROM orders WHERE customer_id = $1 ORDER BY created_at DESC, id LIMIT $2 OFFSET $3`, customerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list orders: %w", err)
	}
	defer rows.Close()
	var out []*domain.Order
	for rows.Next() {
		var u orderRow
		if err := rows.Scan(
			&u.ID, &u.OrganizationID, &u.SellerID, &u.CustomerID, &u.StoreID,
			&u.ItemsJSON, &u.TotalAmount, &u.Currency, &u.Status,
			&u.PricingSnapshotJSON, &u.PaymentReferenceJSON, &u.DeliveryReferenceJSON,
			&u.CustomerReferenceJSON, &u.SellerReferenceJSON,
			&u.Version, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		out = append(out, unmarshalOrder(u))
	}
	return out, rows.Err()
}

// FindByOrganizationID lists an organization's orders, newest first.
func (r *OrderRepository) FindByOrganizationID(ctx context.Context, organizationID string, limit, offset int) ([]*domain.Order, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, organization_id, seller_id, customer_id, store_id, items, total_amount, currency, status, pricing_snapshot, payment_reference, delivery_reference, customer_reference, seller_reference, version, created_at, updated_at
	          FROM orders WHERE organization_id = $1 ORDER BY created_at DESC, id LIMIT $2 OFFSET $3`, organizationID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list organization orders: %w", err)
	}
	defer rows.Close()
	var out []*domain.Order
	for rows.Next() {
		var u orderRow
		if err := rows.Scan(
			&u.ID, &u.OrganizationID, &u.SellerID, &u.CustomerID, &u.StoreID,
			&u.ItemsJSON, &u.TotalAmount, &u.Currency, &u.Status,
			&u.PricingSnapshotJSON, &u.PaymentReferenceJSON, &u.DeliveryReferenceJSON,
			&u.CustomerReferenceJSON, &u.SellerReferenceJSON,
			&u.Version, &u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		out = append(out, unmarshalOrder(u))
	}
	return out, rows.Err()
}
