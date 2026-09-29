package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// PostgresPlanRepository implements domain.PlanRepository for the per-tenant plan catalog.
type PostgresPlanRepository struct {
	db *sql.DB
}

func NewPlanRepository(db *sql.DB) *PostgresPlanRepository {
	return &PostgresPlanRepository{db: db}
}

func (r *PostgresPlanRepository) scanPlan(row interface{ Scan(...any) error }) (*domain.Plan, error) {
	var p domain.Plan
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&p.ID, &p.TenantID, &p.Name, &p.Slug, &p.PriceMinor, &p.Currency,
		&p.BillingCycle, &p.QuotaAmount, &p.CreditPack, &p.Status, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrMonetizationPlanNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan plan: %w", err)
	}
	p.CreatedAt = toUnix(createdAt)
	p.UpdatedAt = toUnix(updatedAt)
	return &p, nil
}

func (r *PostgresPlanRepository) FindActive(ctx context.Context) (*domain.Plan, error) {
	// Legacy single-plan semantic. No production caller exists yet; return the most
	// recently created active plan. Monetization callers use ListByTenant/FindByID.
	const q = `
		SELECT id, tenant_id, name, slug, price_minor, currency, billing_cycle, quota_amount, credit_pack, status, created_at, updated_at
		FROM plans
		WHERE status = 'active'
		ORDER BY created_at DESC
		LIMIT 1
	`
	return r.scanPlan(r.db.QueryRowContext(ctx, q))
}

func (r *PostgresPlanRepository) FindByID(ctx context.Context, id string) (*domain.Plan, error) {
	const q = `
		SELECT id, tenant_id, name, slug, price_minor, currency, billing_cycle, quota_amount, credit_pack, status, created_at, updated_at
		FROM plans
		WHERE id = $1
	`
	return r.scanPlan(r.db.QueryRowContext(ctx, q, id))
}

func (r *PostgresPlanRepository) ListByTenant(ctx context.Context, tenantID string) ([]*domain.Plan, error) {
	const q = `
		SELECT id, tenant_id, name, slug, price_minor, currency, billing_cycle, quota_amount, credit_pack, status, created_at, updated_at
		FROM plans
		WHERE tenant_id = $1 AND status = 'active'
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, q, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to list plans: %w", err)
	}
	defer rows.Close()

	var out []*domain.Plan
	for rows.Next() {
		var p domain.Plan
		var createdAt, updatedAt time.Time
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.Name, &p.Slug, &p.PriceMinor, &p.Currency,
			&p.BillingCycle, &p.QuotaAmount, &p.CreditPack, &p.Status, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan plan: %w", err)
		}
		p.CreatedAt = toUnix(createdAt)
		p.UpdatedAt = toUnix(updatedAt)
		out = append(out, &p)
	}
	return out, rows.Err()
}

func (r *PostgresPlanRepository) FindByTenantAndSlug(ctx context.Context, tenantID, slug string) (*domain.Plan, error) {
	const q = `
		SELECT id, tenant_id, name, slug, price_minor, currency, billing_cycle, quota_amount, credit_pack, status, created_at, updated_at
		FROM plans
		WHERE tenant_id = $1 AND slug = $2
	`
	return r.scanPlan(r.db.QueryRowContext(ctx, q, tenantID, slug))
}

// PostgresSubscriptionRepository implements domain.SubscriptionRepository.
type PostgresSubscriptionRepository struct {
	db *sql.DB
}

func NewSubscriptionRepository(db *sql.DB) *PostgresSubscriptionRepository {
	return &PostgresSubscriptionRepository{db: db}
}

func (r *PostgresSubscriptionRepository) Create(ctx context.Context, sub *domain.Subscription) error {
	const q = `
		INSERT INTO subscriptions (tenant_id, plan_id, owner_type, owner_id, status, current_period_id, cancel_at_period_end, started_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`
	var id string
	if err := r.db.QueryRowContext(ctx, q,
		sub.TenantID, sub.PlanID, sub.OwnerType, sub.OwnerID, string(sub.Status),
		sub.CurrentPeriodID, sub.CancelAtPeriodEnd, toTime(sub.StartedAt), toTime(sub.CreatedAt), toTime(sub.UpdatedAt),
	).Scan(&id); err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}
	sub.ID = id
	return nil
}

func (r *PostgresSubscriptionRepository) FindByID(ctx context.Context, id string) (*domain.Subscription, error) {
	const q = `
		SELECT id, tenant_id, plan_id, owner_type, owner_id, status, current_period_id, cancel_at_period_end, started_at, created_at, updated_at
		FROM subscriptions
		WHERE id = $1
	`
	return r.scanSubscription(r.db.QueryRowContext(ctx, q, id))
}

func (r *PostgresSubscriptionRepository) FindByOwner(ctx context.Context, tenantID, ownerType, ownerID string) (*domain.Subscription, error) {
	const q = `
		SELECT id, tenant_id, plan_id, owner_type, owner_id, status, current_period_id, cancel_at_period_end, started_at, created_at, updated_at
		FROM subscriptions
		WHERE tenant_id = $1 AND owner_type = $2 AND owner_id = $3
		ORDER BY created_at DESC
		LIMIT 1
	`
	return r.scanSubscription(r.db.QueryRowContext(ctx, q, tenantID, ownerType, ownerID))
}

func (r *PostgresSubscriptionRepository) Update(ctx context.Context, sub *domain.Subscription) error {
	const q = `
		UPDATE subscriptions
		SET status = $2, current_period_id = $3, cancel_at_period_end = $4, updated_at = $5
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, q, sub.ID, string(sub.Status), sub.CurrentPeriodID, sub.CancelAtPeriodEnd, toTime(sub.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to update subscription: %w", err)
	}
	return nil
}

func (r *PostgresSubscriptionRepository) scanSubscription(row interface{ Scan(...any) error }) (*domain.Subscription, error) {
	var s domain.Subscription
	var createdAt, updatedAt, startedAt time.Time
	var cancelAt bool
	err := row.Scan(
		&s.ID, &s.TenantID, &s.PlanID, &s.OwnerType, &s.OwnerID, &s.Status,
		&s.CurrentPeriodID, &cancelAt, &startedAt, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrSubscriptionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan subscription: %w", err)
	}
	s.CancelAtPeriodEnd = cancelAt
	s.StartedAt = toUnix(startedAt)
	s.CreatedAt = toUnix(createdAt)
	s.UpdatedAt = toUnix(updatedAt)
	return &s, nil
}

// PostgresBillingPeriodRepository implements domain.BillingPeriodRepository.
type PostgresBillingPeriodRepository struct {
	db *sql.DB
}

func NewBillingPeriodRepository(db *sql.DB) *PostgresBillingPeriodRepository {
	return &PostgresBillingPeriodRepository{db: db}
}

func (r *PostgresBillingPeriodRepository) Create(ctx context.Context, period *domain.BillingPeriod) error {
	var planID interface{}
	if period.PlanID != "" {
		planID = period.PlanID
	}
	const q = `
		INSERT INTO billing_periods (id, tenant_id, subscription_id, plan_id, starts_at, ends_at, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.ExecContext(ctx, q,
		period.ID, period.TenantID, period.SubscriptionID, planID,
		toTime(period.StartsAt), toTime(period.EndsAt), string(period.Status), toTime(period.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to create billing period: %w", err)
	}
	return nil
}

func (r *PostgresBillingPeriodRepository) FindByID(ctx context.Context, id, tenantID string) (*domain.BillingPeriod, error) {
	const q = `
		SELECT id, tenant_id, subscription_id, plan_id, starts_at, ends_at, status, created_at
		FROM billing_periods
		WHERE id = $1 AND tenant_id = $2
	`
	return r.scanBillingPeriod(r.db.QueryRowContext(ctx, q, id, tenantID))
}

func (r *PostgresBillingPeriodRepository) FindBySubscription(ctx context.Context, subscriptionID string) ([]*domain.BillingPeriod, error) {
	const q = `
		SELECT id, tenant_id, subscription_id, plan_id, starts_at, ends_at, status, created_at
		FROM billing_periods
		WHERE subscription_id = $1
		ORDER BY starts_at DESC
	`
	rows, err := r.db.QueryContext(ctx, q, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list billing periods: %w", err)
	}
	defer rows.Close()

	var out []*domain.BillingPeriod
	for rows.Next() {
		var p *domain.BillingPeriod
		p, err = r.scanBillingPeriod(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *PostgresBillingPeriodRepository) Update(ctx context.Context, period *domain.BillingPeriod) error {
	const q = `UPDATE billing_periods SET status = $2 WHERE id = $1 AND tenant_id = $3`
	_, err := r.db.ExecContext(ctx, q, period.ID, string(period.Status), period.TenantID)
	if err != nil {
		return fmt.Errorf("failed to update billing period: %w", err)
	}
	return nil
}

func (r *PostgresBillingPeriodRepository) scanBillingPeriod(row interface{ Scan(...any) error }) (*domain.BillingPeriod, error) {
	var p domain.BillingPeriod
	var planID sql.NullString
	var startsAt, endsAt, createdAt time.Time
	err := row.Scan(
		&p.ID, &p.TenantID, &p.SubscriptionID, &planID, &startsAt, &endsAt, &p.Status, &createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrBillingPeriodNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan billing period: %w", err)
	}
	if planID.Valid {
		p.PlanID = planID.String
	}
	p.StartsAt = toUnix(startsAt)
	p.EndsAt = toUnix(endsAt)
	p.CreatedAt = toUnix(createdAt)
	return &p, nil
}

// PostgresCreditPurchaseRepository implements domain.CreditPurchaseRepository.
type PostgresCreditPurchaseRepository struct {
	db *sql.DB
}

func NewCreditPurchaseRepository(db *sql.DB) *PostgresCreditPurchaseRepository {
	return &PostgresCreditPurchaseRepository{db: db}
}

func (r *PostgresCreditPurchaseRepository) Create(ctx context.Context, purchase *domain.CreditPurchase) error {
	const q = `
		INSERT INTO credit_purchases (tenant_id, owner_type, owner_id, plan_id, amount_credits, payment_id, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`
	var id string
	if err := r.db.QueryRowContext(ctx, q,
		purchase.TenantID, purchase.OwnerType, purchase.OwnerID, purchase.PlanID,
		purchase.AmountCredits, purchase.PaymentID, string(purchase.Status),
		toTime(purchase.CreatedAt), toTime(purchase.UpdatedAt),
	).Scan(&id); err != nil {
		return fmt.Errorf("failed to create credit purchase: %w", err)
	}
	purchase.ID = id
	return nil
}

func (r *PostgresCreditPurchaseRepository) FindByID(ctx context.Context, id string) (*domain.CreditPurchase, error) {
	const q = `
		SELECT id, tenant_id, owner_type, owner_id, plan_id, amount_credits, payment_id, status, created_at, updated_at
		FROM credit_purchases
		WHERE id = $1
	`
	return r.scanCreditPurchase(r.db.QueryRowContext(ctx, q, id))
}

func (r *PostgresCreditPurchaseRepository) FindByPaymentID(ctx context.Context, paymentID string) (*domain.CreditPurchase, error) {
	const q = `
		SELECT id, tenant_id, owner_type, owner_id, plan_id, amount_credits, payment_id, status, created_at, updated_at
		FROM credit_purchases
		WHERE payment_id = $1
	`
	return r.scanCreditPurchase(r.db.QueryRowContext(ctx, q, paymentID))
}

func (r *PostgresCreditPurchaseRepository) Update(ctx context.Context, purchase *domain.CreditPurchase) error {
	const q = `UPDATE credit_purchases SET status = $2, updated_at = $3 WHERE id = $1`
	_, err := r.db.ExecContext(ctx, q, purchase.ID, string(purchase.Status), toTime(purchase.UpdatedAt))
	if err != nil {
		return fmt.Errorf("failed to update credit purchase: %w", err)
	}
	return nil
}

// MarkCaptured upserts a captured credit purchase. Idempotent: replay with the same
// payment_id is a no-op (ON CONFLICT DO UPDATE keeps it captured).
func (r *PostgresCreditPurchaseRepository) MarkCaptured(ctx context.Context, paymentID, tenantID, planID, ownerType, ownerID string, amountCredits, now int64) error {
	const q = `
		INSERT INTO credit_purchases (id, tenant_id, owner_type, owner_id, plan_id, amount_credits, payment_id, status, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, 'captured', NOW(), NOW())
		ON CONFLICT (payment_id) DO UPDATE SET status = 'captured', updated_at = NOW()
	`
	_, err := r.db.ExecContext(ctx, q, tenantID, ownerType, ownerID, planID, amountCredits, paymentID)
	if err != nil {
		return fmt.Errorf("failed to mark credit purchase captured: %w", err)
	}
	return nil
}

func (r *PostgresCreditPurchaseRepository) scanCreditPurchase(row interface{ Scan(...any) error }) (*domain.CreditPurchase, error) {
	var p domain.CreditPurchase
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&p.ID, &p.TenantID, &p.OwnerType, &p.OwnerID, &p.PlanID,
		&p.AmountCredits, &p.PaymentID, &p.Status, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrCreditPurchaseNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan credit purchase: %w", err)
	}
	p.CreatedAt = toUnix(createdAt)
	p.UpdatedAt = toUnix(updatedAt)
	return &p, nil
}

// PostgresUsageReservationRepository implements domain.UsageReservationRepository.
type PostgresUsageReservationRepository struct {
	db *sql.DB
}

func NewUsageReservationRepository(db *sql.DB) *PostgresUsageReservationRepository {
	return &PostgresUsageReservationRepository{db: db}
}

func (r *PostgresUsageReservationRepository) Record(ctx context.Context, reservation *domain.UsageReservation) error {
	var tenantParam interface{}
	if reservation.TenantID != "" {
		tenantParam = reservation.TenantID
	}
	const q = `
		INSERT INTO usage_reservations (reservation_id, account_id, action_type, amount, status, reserved_at, consumed_at, released_at, idempotency_key, tenant_id)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, to_timestamp($5), NULL, NULL, $6,
			CASE WHEN $7::text = '' THEN NULL ELSE $7::uuid END
		)
		RETURNING reservation_id
	`
	var id string
	err := r.db.QueryRowContext(ctx, q,
		reservation.AccountID, string(reservation.ActionType),
		reservation.Amount, string(reservation.Status), reservation.ReservedAt,
		reservation.IdempotencyKey, tenantParam,
	).Scan(&id)
	if err != nil {
		return fmt.Errorf("failed to record usage reservation: %w", err)
	}
	reservation.ReservationID = id
	return nil
}

func (r *PostgresUsageReservationRepository) FindByID(ctx context.Context, reservationID string) (*domain.UsageReservation, error) {
	const q = `
		SELECT reservation_id, account_id, action_type, amount, status, reserved_at, consumed_at, released_at, idempotency_key, tenant_id
		FROM usage_reservations
		WHERE reservation_id = $1
	`
	return r.scanReservation(r.db.QueryRowContext(ctx, q, reservationID))
}

func (r *PostgresUsageReservationRepository) FindByAccount(ctx context.Context, accountID string) ([]domain.UsageReservation, error) {
	const q = `
		SELECT reservation_id, account_id, action_type, amount, status, reserved_at, consumed_at, released_at, idempotency_key, tenant_id
		FROM usage_reservations
		WHERE account_id = $1
		ORDER BY reserved_at DESC
	`
	rows, err := r.db.QueryContext(ctx, q, accountID)
	if err != nil {
		return nil, fmt.Errorf("failed to list usage reservations: %w", err)
	}
	defer rows.Close()

	var out []domain.UsageReservation
	for rows.Next() {
		var res domain.UsageReservation
		res, err = r.scanReservationRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, rows.Err()
}

func (r *PostgresUsageReservationRepository) UpdateStatus(ctx context.Context, reservationID string, status domain.UsageReservationStatus) error {
	const q = `UPDATE usage_reservations SET status = $1 WHERE reservation_id = $2`
	_, err := r.db.ExecContext(ctx, q, string(status), reservationID)
	if err != nil {
		return fmt.Errorf("failed to update reservation status: %w", err)
	}
	return nil
}

// ReserveCreditsTx atomically reserves credits on the credit_accounts table and creates
// a durable reservation row in one Postgres transaction. The idempotency_key column
// (unique index from migration 000017) prevents duplicate reservations for the same
// logical action (F2/F5). The balance-guard UPDATE clause `AND (granted - reserved -
// consumed) >= cost` prevents the account from going negative under concurrent
// reservation attempts (F4/F6).
func (r *PostgresUsageReservationRepository) ReserveCreditsTx(ctx context.Context, accountID string, actionType domain.CommerceActionType, cost int64, idempotencyKey, tenantID string, now int64) (reservation *domain.UsageReservation, err error) {
	// An empty key would make every caller share one reservation row.
	if idempotencyKey == "" {
		return nil, fmt.Errorf("reserve: idempotency key is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("reserve begin: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var existing domain.UsageReservation
	var reservedAt time.Time
	var existingTenant sql.NullString
	err = tx.QueryRowContext(ctx, `
		SELECT reservation_id, account_id, action_type, amount, status, reserved_at, idempotency_key, tenant_id
		FROM usage_reservations
		WHERE idempotency_key = $1 AND account_id = $2
	`, idempotencyKey, accountID).Scan(
		&existing.ReservationID, &existing.AccountID, &existing.ActionType, &existing.Amount,
		&existing.Status, &reservedAt, &existing.IdempotencyKey, &existingTenant,
	)
	if err == nil {
		existing.TenantID = existingTenant.String
		existing.ReservedAt = toUnix(reservedAt)
		_ = tx.Commit()
		return &existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("reserve idempotency check failed: %w", err)
	}

	// Serialize on the account row to prevent deadlocks between the reservation
	// INSERT (FK share lock on credit_accounts) and the balance-guard UPDATE below,
	// and to make over-draw prevention robust under concurrent reservations.
	var accountExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM credit_accounts WHERE account_id = $1 FOR UPDATE)`, accountID).Scan(&accountExists); err != nil {
		return nil, fmt.Errorf("reserve account lock failed: %w", err)
	}
	if !accountExists {
		return nil, domain.ErrUsageAccountNotFound
	}
	var tenantParam interface{}
	if tenantID != "" {
		tenantParam = tenantID
	}

	var reservationID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO usage_reservations (account_id, action_type, amount, status, reserved_at, idempotency_key, tenant_id)
		VALUES ($1, $2, $3, 'active', to_timestamp($4), $5,
			CASE WHEN $6::text = '' THEN NULL ELSE $6::uuid END
		)
		RETURNING reservation_id
	`, accountID, string(actionType), cost, now, idempotencyKey, tenantParam).Scan(&reservationID)
	if err != nil {
		return nil, fmt.Errorf("reserve insert failed: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
		UPDATE credit_accounts
		SET reserved = reserved + $1, updated_at = NOW()
		WHERE account_id = $2 AND (granted - reserved - consumed) >= $1
	`, cost, accountID)
	if err != nil {
		return nil, fmt.Errorf("reserve account update failed: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return nil, domain.ErrInsufficientCredits
	}

	var granted, reserved, consumed int64
	var periodID, ownerID, acctTenantID string
	err = tx.QueryRowContext(ctx, `
		SELECT granted, reserved, consumed, period_id, owner_id, tenant_id
		FROM credit_accounts
		WHERE account_id = $1
	`, accountID).Scan(&granted, &reserved, &consumed, &periodID, &ownerID, &acctTenantID)
	if err != nil {
		return nil, fmt.Errorf("reserve balance read failed: %w", err)
	}

	balanceBefore := granted - (reserved - cost) - consumed
	balanceAfter := granted - reserved - consumed

	metaJSON, _ := json.Marshal(map[string]interface{}{
		"reservation_id":  reservationID,
		"idempotency_key": idempotencyKey,
	})
	_, err = tx.ExecContext(ctx, `
		INSERT INTO usage_ledger (id, account_id, action_type, actor_id, tenant_id, transaction_type, amount, balance_before, balance_after, period_id, created_at, metadata)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, 'RESERVE', $5, $6, $7, $8, to_timestamp($9), $10)
	`, accountID, string(actionType), ownerID, tenantParam, cost, balanceBefore, balanceAfter, periodID, now, metaJSON)
	if err != nil {
		return nil, fmt.Errorf("reserve ledger insert failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("reserve commit failed: %w", err)
	}

	return &domain.UsageReservation{
		ReservationID:  reservationID,
		AccountID:      accountID,
		ActionType:     actionType,
		Amount:         cost,
		ReservedAt:     now,
		Status:         domain.UsageReservationActive,
		IdempotencyKey: idempotencyKey,
		TenantID:       tenantID,
	}, nil
}

// ConsumeReservation atomically marks a reservation consumed, moves the reserved
// credits to consumed, and appends a ledger event. Returns domain errors for
// missing reservations, double-consume, or consume-after-release.
func (r *PostgresUsageReservationRepository) ConsumeReservation(ctx context.Context, reservationID string, now int64) (err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("consume begin: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var res domain.UsageReservation
	var accountID string
	err = tx.QueryRowContext(ctx, `
		SELECT reservation_id, account_id, action_type, amount, status
		FROM usage_reservations
		WHERE reservation_id = $1
	`, reservationID).Scan(
		&res.ReservationID, &accountID, &res.ActionType, &res.Amount, &res.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrUsageReservationNotFound
	}
	if err != nil {
		return fmt.Errorf("consume reservation lookup failed: %w", err)
	}

	if res.Status != domain.UsageReservationActive {
		if res.Status == domain.UsageReservationConsumed {
			return domain.ErrUsageAlreadyConsumed
		}
		if res.Status == domain.UsageReservationReleased {
			return domain.ErrUsageReservationConflict
		}
		return domain.ErrUsageReservationConflict
	}

	res.ReservedAt = 0
	_, err = tx.ExecContext(ctx, `
		UPDATE credit_accounts
		SET reserved = reserved - $1, consumed = consumed + $1, updated_at = NOW()
		WHERE account_id = $2 AND reserved >= $1
	`, res.Amount, accountID)
	if err != nil {
		return fmt.Errorf("consume account update failed: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE usage_reservations
		SET status = 'consumed', consumed_at = to_timestamp($1)
		WHERE reservation_id = $2
	`, now, reservationID)
	if err != nil {
		return fmt.Errorf("consume reservation update failed: %w", err)
	}

	var granted, reserved, consumed int64
	var periodID, ownerID string
	var tenantID interface{}
	err = tx.QueryRowContext(ctx, `
		SELECT granted, reserved, consumed, period_id, owner_id, tenant_id
		FROM credit_accounts
		WHERE account_id = $1
	`, accountID).Scan(&granted, &reserved, &consumed, &periodID, &ownerID, &tenantID)
	if err != nil {
		return fmt.Errorf("consume balance read failed: %w", err)
	}

	balanceBefore := granted - reserved - consumed + res.Amount
	balanceAfter := granted - reserved - consumed

	metaJSON, _ := json.Marshal(map[string]interface{}{
		"reservation_id": reservationID,
	})
	_, err = tx.ExecContext(ctx, `
		INSERT INTO usage_ledger (id, account_id, action_type, actor_id, tenant_id, transaction_type, amount, balance_before, balance_after, period_id, created_at, metadata)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, 'CONSUME', $5, $6, $7, $8, to_timestamp($9), $10)
	`, accountID, string(res.ActionType), ownerID, tenantID, res.Amount, balanceBefore, balanceAfter, periodID, now, metaJSON)
	if err != nil {
		return fmt.Errorf("consume ledger insert failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("consume commit failed: %w", err)
	}
	return nil
}

// ReleaseReservation atomically marks a reservation released, returns the reserved
// credits to the account, and appends a ledger event.
func (r *PostgresUsageReservationRepository) ReleaseReservation(ctx context.Context, reservationID string, now int64) (err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("release begin: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var res domain.UsageReservation
	var accountID string
	err = tx.QueryRowContext(ctx, `
		SELECT reservation_id, account_id, action_type, amount, status
		FROM usage_reservations
		WHERE reservation_id = $1
	`, reservationID).Scan(
		&res.ReservationID, &accountID, &res.ActionType, &res.Amount, &res.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrUsageReservationNotFound
	}
	if err != nil {
		return fmt.Errorf("release reservation lookup failed: %w", err)
	}

	if res.Status != domain.UsageReservationActive {
		if res.Status == domain.UsageReservationReleased {
			return domain.ErrUsageAlreadyReleased
		}
		if res.Status == domain.UsageReservationConsumed {
			return domain.ErrUsageReservationConflict
		}
		return domain.ErrUsageReservationConflict
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE credit_accounts
		SET reserved = reserved - $1, updated_at = NOW()
		WHERE account_id = $2 AND reserved >= $1
	`, res.Amount, accountID)
	if err != nil {
		return fmt.Errorf("release account update failed: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE usage_reservations
		SET status = 'released', released_at = to_timestamp($1)
		WHERE reservation_id = $2
	`, now, reservationID)
	if err != nil {
		return fmt.Errorf("release reservation update failed: %w", err)
	}

	var granted, reserved, consumed int64
	var periodID, ownerID string
	var tenantID interface{}
	err = tx.QueryRowContext(ctx, `
		SELECT granted, reserved, consumed, period_id, owner_id, tenant_id
		FROM credit_accounts
		WHERE account_id = $1
	`, accountID).Scan(&granted, &reserved, &consumed, &periodID, &ownerID, &tenantID)
	if err != nil {
		return fmt.Errorf("release balance read failed: %w", err)
	}

	balanceBefore := granted - (reserved + res.Amount) - consumed
	balanceAfter := granted - reserved - consumed

	metaJSON, _ := json.Marshal(map[string]interface{}{
		"reservation_id": reservationID,
	})
	_, err = tx.ExecContext(ctx, `
		INSERT INTO usage_ledger (id, account_id, action_type, actor_id, tenant_id, transaction_type, amount, balance_before, balance_after, period_id, created_at, metadata)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, 'RELEASE', $5, $6, $7, $8, to_timestamp($9), $10)
	`, accountID, string(res.ActionType), ownerID, tenantID, res.Amount, balanceBefore, balanceAfter, periodID, now, metaJSON)
	if err != nil {
		return fmt.Errorf("release ledger insert failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("release commit failed: %w", err)
	}
	return nil
}

func (r *PostgresUsageReservationRepository) scanReservation(row interface{ Scan(...any) error }) (*domain.UsageReservation, error) {
	res, err := r.scanReservationRow(row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrUsageReservationNotFound
	}
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (r *PostgresUsageReservationRepository) scanReservationRow(row interface{ Scan(...any) error }) (domain.UsageReservation, error) {
	var res domain.UsageReservation
	var consumedAt, releasedAt, reservedAt sql.NullTime
	var idempotencyKey sql.NullString
	var tenantID sql.NullString
	var actionType string
	err := row.Scan(&res.ReservationID, &res.AccountID, &actionType, &res.Amount, &res.Status, &reservedAt, &consumedAt, &releasedAt, &idempotencyKey, &tenantID)
	if err != nil {
		return res, err
	}
	res.ActionType = domain.CommerceActionType(actionType)
	if reservedAt.Valid {
		res.ReservedAt = toUnix(reservedAt.Time)
	}
	if idempotencyKey.Valid {
		res.IdempotencyKey = idempotencyKey.String
	}
	if tenantID.Valid {
		res.TenantID = tenantID.String
	}
	return res, nil
}

// Compile-time assertions that the postgres repositories satisfy the domain interfaces.
var (
	_ domain.PlanRepository             = (*PostgresPlanRepository)(nil)
	_ domain.SubscriptionRepository     = (*PostgresSubscriptionRepository)(nil)
	_ domain.BillingPeriodRepository    = (*PostgresBillingPeriodRepository)(nil)
	_ domain.CreditPurchaseRepository   = (*PostgresCreditPurchaseRepository)(nil)
	_ domain.UsageReservationRepository = (*PostgresUsageReservationRepository)(nil)
	_ domain.CreditGranter              = (*CreditAccountRepository)(nil)
	_ domain.CreditAccountByOwnerStore  = (*CreditAccountRepository)(nil)
	_ domain.CreditAccountCreator       = (*CreditAccountRepository)(nil)
)

// ReleaseStaleReservations releases active credit reservations older than
// maxAge. Reservations are consumed or released within the request that
// created them, so an old active one was orphaned by a crash or timeout and
// would otherwise hold the customer's credits forever. Each release uses the
// atomic ReleaseReservation primitive; a reservation completed concurrently is
// skipped. Returns the number released.
func (r *PostgresUsageReservationRepository) ReleaseStaleReservations(ctx context.Context, maxAge time.Duration, now int64) (int, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT reservation_id FROM usage_reservations
		WHERE status = 'active' AND reserved_at < to_timestamp($1)
		ORDER BY reserved_at
		LIMIT 500`, now-int64(maxAge/time.Second))
	if err != nil {
		return 0, fmt.Errorf("list stale reservations: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	released := 0
	for _, id := range ids {
		if err := r.ReleaseReservation(ctx, id, now); err == nil {
			released++
		}
	}
	return released, nil
}
