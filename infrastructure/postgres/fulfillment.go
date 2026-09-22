package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

type fulfillmentRow struct {
	ID            string
	TenantID      string
	OrderID       *string
	ReservationID *string
	Type          string
	Status        string
	Reference     string
	TrackingRef   *string
	AssignedTo    *string
	MetadataJSON  []byte
	Version       int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type FulfillmentRepository struct {
	db *sql.DB
}

func NewFulfillmentRepository(db *sql.DB) *FulfillmentRepository {
	return &FulfillmentRepository{db: db}
}

func (r *FulfillmentRepository) Create(ctx context.Context, fulfillment *domain.Fulfillment) error {
	metadataJSON, _ := json.Marshal(fulfillment.Metadata)
	if metadataJSON == nil {
		metadataJSON = []byte("{}")
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO fulfillments (id, tenant_id, order_id, reservation_id, type, status, reference, tracking_ref, assigned_to, metadata, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, fulfillment.ID, fulfillment.TenantID, fulfillment.OrderID, fulfillment.ReservationID,
		string(fulfillment.Type), string(fulfillment.Status), fulfillment.Reference,
		fulfillment.TrackingRef, fulfillment.AssignedTo, string(metadataJSON),
		fulfillment.Version, toTime(fulfillment.CreatedAt), toTime(fulfillment.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to create fulfillment: %w", err)
	}
	return nil
}

func (r *FulfillmentRepository) FindByID(ctx context.Context, tenantID, id string) (*domain.Fulfillment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, order_id, reservation_id, type, status, reference, tracking_ref, assigned_to, metadata, version, created_at, updated_at
		FROM fulfillments WHERE id = $1 AND tenant_id = $2
	`, id, tenantID)
	return scanFulfillment(row)
}

// FindByIDForUpdate retrieves a fulfillment by ID with row-level lock.
func (r *FulfillmentRepository) FindByIDForUpdate(ctx context.Context, tenantID, id string) (*domain.Fulfillment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, order_id, reservation_id, type, status, reference, tracking_ref, assigned_to, metadata, version, created_at, updated_at
		FROM fulfillments WHERE id = $1 AND tenant_id = $2 FOR UPDATE
	`, id, tenantID)
	return scanFulfillment(row)
}

func (r *FulfillmentRepository) FindByOrderID(ctx context.Context, tenantID, orderID string) (*domain.Fulfillment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, order_id, reservation_id, type, status, reference, tracking_ref, assigned_to, metadata, version, created_at, updated_at
		FROM fulfillments WHERE order_id = $1 AND tenant_id = $2
	`, orderID, tenantID)
	return scanFulfillment(row)
}

func (r *FulfillmentRepository) FindByReservationID(ctx context.Context, tenantID, reservationID string) (*domain.Fulfillment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, order_id, reservation_id, type, status, reference, tracking_ref, assigned_to, metadata, version, created_at, updated_at
		FROM fulfillments WHERE reservation_id = $1 AND tenant_id = $2
	`, reservationID, tenantID)
	return scanFulfillment(row)
}

func (r *FulfillmentRepository) FindByAssignedTo(ctx context.Context, tenantID, actorID string, status *domain.FulfillmentStatus) ([]*domain.Fulfillment, error) {
	query := `SELECT id, tenant_id, order_id, reservation_id, type, status, reference, tracking_ref, assigned_to, metadata, version, created_at, updated_at
		FROM fulfillments WHERE tenant_id = $1 AND assigned_to = $2`
	args := []interface{}{tenantID, actorID}

	if status != nil {
		query += " AND status = $3"
		args = append(args, string(*status))
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to find fulfillments by assigned: %w", err)
	}
	defer rows.Close()

	var fulfillments []*domain.Fulfillment
	for rows.Next() {
		f, err := scanFulfillmentFromRow(rows)
		if err != nil {
			return nil, err
		}
		fulfillments = append(fulfillments, f)
	}
	if fulfillments == nil {
		fulfillments = []*domain.Fulfillment{}
	}
	return fulfillments, nil
}

func (r *FulfillmentRepository) Update(ctx context.Context, fulfillment *domain.Fulfillment) error {
	metadataJSON, _ := json.Marshal(fulfillment.Metadata)
	if metadataJSON == nil {
		metadataJSON = []byte("{}")
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE fulfillments SET order_id = $1, reservation_id = $2, type = $3, status = $4, reference = $5,
		tracking_ref = $6, assigned_to = $7, metadata = $8, version = $9, updated_at = $10
		WHERE id = $11 AND version = $12
	`, fulfillment.OrderID, fulfillment.ReservationID, string(fulfillment.Type),
		string(fulfillment.Status), fulfillment.Reference, fulfillment.TrackingRef,
		fulfillment.AssignedTo, string(metadataJSON), fulfillment.Version,
		toTime(fulfillment.UpdatedAt), fulfillment.ID, fulfillment.Version-1)
	if err != nil {
		return fmt.Errorf("failed to update fulfillment: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}

func scanFulfillment(row *sql.Row) (*domain.Fulfillment, error) {
	var r fulfillmentRow
	err := row.Scan(&r.ID, &r.TenantID, &r.OrderID, &r.ReservationID,
		&r.Type, &r.Status, &r.Reference, &r.TrackingRef, &r.AssignedTo,
		&r.MetadataJSON, &r.Version, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrFulfillmentNotFound
		}
		return nil, fmt.Errorf("failed to scan fulfillment: %w", err)
	}
	return unmarshalFulfillment(r), nil
}

func scanFulfillmentFromRow(rows *sql.Rows) (*domain.Fulfillment, error) {
	var r fulfillmentRow
	err := rows.Scan(&r.ID, &r.TenantID, &r.OrderID, &r.ReservationID,
		&r.Type, &r.Status, &r.Reference, &r.TrackingRef, &r.AssignedTo,
		&r.MetadataJSON, &r.Version, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to scan fulfillment: %w", err)
	}
	return unmarshalFulfillment(r), nil
}

func unmarshalFulfillment(r fulfillmentRow) *domain.Fulfillment {
	metadata := make(map[string]interface{})
	if len(r.MetadataJSON) > 0 && string(r.MetadataJSON) != "null" {
		json.Unmarshal(r.MetadataJSON, &metadata)
	}

	return &domain.Fulfillment{
		ID:            r.ID,
		TenantID:      r.TenantID,
		OrderID:       r.OrderID,
		ReservationID: r.ReservationID,
		Type:          domain.FulfillmentType(r.Type),
		Status:        domain.FulfillmentStatus(r.Status),
		Reference:     r.Reference,
		TrackingRef:   r.TrackingRef,
		AssignedTo:    r.AssignedTo,
		Metadata:      metadata,
		Version:       r.Version,
		CreatedAt:     toUnix(r.CreatedAt),
		UpdatedAt:     toUnix(r.UpdatedAt),
	}
}
