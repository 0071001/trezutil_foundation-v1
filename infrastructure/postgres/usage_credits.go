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

// CreditAccountRepository implements domain.CreditAccountRepository.
type CreditAccountRepository struct {
	db *sql.DB
}

func NewCreditAccountRepository(db *sql.DB) *CreditAccountRepository {
	return &CreditAccountRepository{db: db}
}

func (r *CreditAccountRepository) Create(ctx context.Context, account *domain.CreditAccount) error {
	const q = `
		INSERT INTO credit_accounts (account_id, owner_type, owner_id, tenant_id, plan_id, period_id, granted, reserved, consumed, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := r.db.ExecContext(ctx, q,
		account.AccountID,
		account.OwnerType,
		account.OwnerID,
		account.TenantID,
		account.PlanID,
		account.PeriodID,
		account.Granted,
		account.Reserved,
		account.Consumed,
		account.CreatedAt,
		account.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create credit account: %w", err)
	}
	return nil
}

func (r *CreditAccountRepository) FindByID(ctx context.Context, accountID string) (*domain.CreditAccount, error) {
	const q = `
		SELECT account_id, owner_type, owner_id, tenant_id, plan_id, period_id, granted, reserved, consumed, created_at, updated_at
		FROM credit_accounts
		WHERE account_id = $1
	`
	var account domain.CreditAccount
	var createdAt, updatedAt time.Time
	err := r.db.QueryRowContext(ctx, q, accountID).Scan(
		&account.AccountID,
		&account.OwnerType,
		&account.OwnerID,
		&account.TenantID,
		&account.PlanID,
		&account.PeriodID,
		&account.Granted,
		&account.Reserved,
		&account.Consumed,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find credit account by ID: %w", err)
	}
	account.CreatedAt = toUnix(createdAt)
	account.UpdatedAt = toUnix(updatedAt)
	return &account, nil
}

func (r *CreditAccountRepository) FindByOwner(ctx context.Context, ownerType domain.ActorType, ownerID string, tenantID string) (*domain.CreditAccount, error) {
	const q = `
		SELECT account_id, owner_type, owner_id, tenant_id, plan_id, period_id, granted, reserved, consumed, created_at, updated_at
		FROM credit_accounts
		WHERE owner_type = $1 AND owner_id = $2 AND tenant_id = $3
	`
	var account domain.CreditAccount
	var createdAt, updatedAt time.Time
	err := r.db.QueryRowContext(ctx, q, ownerType, ownerID, tenantID).Scan(
		&account.AccountID,
		&account.OwnerType,
		&account.OwnerID,
		&account.TenantID,
		&account.PlanID,
		&account.PeriodID,
		&account.Granted,
		&account.Reserved,
		&account.Consumed,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find credit account by owner: %w", err)
	}
	account.CreatedAt = toUnix(createdAt)
	account.UpdatedAt = toUnix(updatedAt)
	return &account, nil
}

func (r *CreditAccountRepository) Update(ctx context.Context, account *domain.CreditAccount) error {
	const q = `
		UPDATE credit_accounts
		SET granted = $2, reserved = $3, consumed = $4, updated_at = $5
		WHERE account_id = $1
	`
	_, err := r.db.ExecContext(ctx, q,
		account.AccountID,
		account.Granted,
		account.Reserved,
		account.Consumed,
		account.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update credit account: %w", err)
	}
	return nil
}

// UsagePolicyRepository implements domain.UsagePolicyRepository.
type UsagePolicyRepository struct {
	db *sql.DB
}

func NewUsagePolicyRepository(db *sql.DB) *UsagePolicyRepository {
	return &UsagePolicyRepository{db: db}
}

func (r *UsagePolicyRepository) FindByAction(ctx context.Context, actionType domain.CommerceActionType) (*domain.UsagePolicy, error) {
	const q = `
		SELECT id, action_type, channel, actor_type, cost, active, created_at, updated_at
		FROM usage_policies
		WHERE action_type = $1 AND active = true
		LIMIT 1
	`
	var policy domain.UsagePolicy
	err := r.db.QueryRowContext(ctx, q, actionType).Scan(
		&policy.ID,
		&policy.ActionType,
		&policy.Channel,
		&policy.ActorType,
		&policy.Cost,
		&policy.Active,
		&policy.CreatedAt,
		&policy.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find usage policy: %w", err)
	}
	return &policy, nil
}

func (r *UsagePolicyRepository) FindAll(ctx context.Context) ([]domain.UsagePolicy, error) {
	const q = `
		SELECT id, action_type, channel, actor_type, cost, active, created_at, updated_at
		FROM usage_policies
	`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("failed to find usage policies: %w", err)
	}
	defer rows.Close()

	var policies []domain.UsagePolicy
	for rows.Next() {
		var policy domain.UsagePolicy
		if err := rows.Scan(
			&policy.ID,
			&policy.ActionType,
			&policy.Channel,
			&policy.ActorType,
			&policy.Cost,
			&policy.Active,
			&policy.CreatedAt,
			&policy.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan usage policy: %w", err)
		}
		policies = append(policies, policy)
	}
	return policies, nil
}

// UsageLedgerRepository implements domain.UsageLedgerRepository.
type UsageLedgerRepository struct {
	db *sql.DB
}

func NewUsageLedgerRepository(db *sql.DB) *UsageLedgerRepository {
	return &UsageLedgerRepository{db: db}
}

func (r *UsageLedgerRepository) Record(ctx context.Context, event *domain.UsageEvent) error {
	const q = `
		INSERT INTO usage_ledger (id, account_id, action_type, actor_id, tenant_id, transaction_type, amount, balance_before, balance_after, period_id, created_at, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := r.db.ExecContext(ctx, q,
		event.ID,
		event.AccountID,
		event.ActionType,
		event.ActorID,
		event.TenantID,
		event.TransactionType,
		event.Amount,
		event.BalanceBefore,
		event.BalanceAfter,
		event.PeriodID,
		event.CreatedAt,
		event.Metadata,
	)
	if err != nil {
		return fmt.Errorf("failed to record usage event: %w", err)
	}
	return nil
}

func (r *UsageLedgerRepository) FindByAccount(ctx context.Context, accountID string, periodID string) ([]domain.UsageEvent, error) {
	const q = `
		SELECT id, account_id, action_type, actor_id, tenant_id, transaction_type, amount, balance_before, balance_after, period_id, created_at, metadata
		FROM usage_ledger
		WHERE account_id = $1 AND period_id = $2
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, q, accountID, periodID)
	if err != nil {
		return nil, fmt.Errorf("failed to find usage events by account: %w", err)
	}
	defer rows.Close()

	var events []domain.UsageEvent
	for rows.Next() {
		var event domain.UsageEvent
		if err := rows.Scan(
			&event.ID,
			&event.AccountID,
			&event.ActionType,
			&event.ActorID,
			&event.TenantID,
			&event.TransactionType,
			&event.Amount,
			&event.BalanceBefore,
			&event.BalanceAfter,
			&event.PeriodID,
			&event.CreatedAt,
			&event.Metadata,
		); err != nil {
			return nil, fmt.Errorf("failed to scan usage event: %w", err)
		}
		events = append(events, event)
	}
	return events, nil
}

func (r *UsageLedgerRepository) FindByAction(ctx context.Context, actionType domain.CommerceActionType, periodID string) ([]domain.UsageEvent, error) {
	const q = `
		SELECT id, account_id, action_type, actor_id, tenant_id, transaction_type, amount, balance_before, balance_after, period_id, created_at, metadata
		FROM usage_ledger
		WHERE action_type = $1 AND period_id = $2
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, q, actionType, periodID)
	if err != nil {
		return nil, fmt.Errorf("failed to find usage events by action: %w", err)
	}
	defer rows.Close()

	var events []domain.UsageEvent
	for rows.Next() {
		var event domain.UsageEvent
		if err := rows.Scan(
			&event.ID,
			&event.AccountID,
			&event.ActionType,
			&event.ActorID,
			&event.TenantID,
			&event.TransactionType,
			&event.Amount,
			&event.BalanceBefore,
			&event.BalanceAfter,
			&event.PeriodID,
			&event.CreatedAt,
			&event.Metadata,
		); err != nil {
			return nil, fmt.Errorf("failed to scan usage event: %w", err)
		}
		events = append(events, event)
	}
	return events, nil
}

// GrantCreditsTx is the atomic, idempotent credit-grant primitive (domain.CreditGranter).
// It inserts a credit_grants row (ON CONFLICT idempotency_key), bumps
// credit_accounts.granted, and appends a usage_ledger event — all in one transaction.
// Returns granted=false (with the current balance) on idempotent replay.
func (r *CreditAccountRepository) GrantCreditsTx(
	ctx context.Context,
	accountID, idempotencyKey string,
	source domain.CreditGrantSource,
	amount int64,
	tenantID, periodID string,
	paymentID *string,
	reason string,
) (granted bool, newBalance int64, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, 0, fmt.Errorf("grant begin: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var txnType string
	switch source {
	case domain.CreditGrantSourcePurchase:
		txnType = string(domain.UsagePurchase)
	default:
		txnType = string(domain.UsageGrant)
	}

	// 1. Insert the grant row idempotently. ON CONFLICT (idempotency_key) DO NOTHING
	//    is the dedup mechanism (invariant: idempotent credit mutations).
	var inserted bool
	var grantID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO credit_grants (account_id, idempotency_key, source, amount, tenant_id, period_id, payment_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING id
	`, accountID, idempotencyKey, string(source), amount, tenantID, periodID, paymentID).Scan(&grantID)
	if err == nil {
		inserted = true
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, 0, fmt.Errorf("grant insert failed: %w", err)
	}

	// 2. If this is a replay (existing grant), return the current balance without mutating.
	if !inserted {
		var bal int64
		if qerr := tx.QueryRowContext(ctx, `SELECT granted FROM credit_accounts WHERE account_id = $1`, accountID).Scan(&bal); qerr != nil {
			_ = tx.Rollback()
			if qerr == sql.ErrNoRows {
				return false, 0, domain.ErrUsageAccountNotFound
			}
			return false, 0, fmt.Errorf("grant replay balance lookup failed: %w", qerr)
		}
		return false, bal, tx.Commit()
	}

	// 3. Bump the account balance atomically and verify the account exists.
	var balanceBefore, balanceAfter int64
	res, err := tx.ExecContext(ctx, `
		UPDATE credit_accounts
		SET granted = granted + $1, updated_at = NOW()
		WHERE account_id = $2
	`, amount, accountID)
	if err != nil {
		return false, 0, fmt.Errorf("grant update account failed: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return false, 0, domain.ErrUsageAccountNotFound
	}
	if err := tx.QueryRowContext(ctx, `SELECT granted FROM credit_accounts WHERE account_id = $1`, accountID).Scan(&balanceAfter); err != nil {
		return false, 0, fmt.Errorf("grant balance read failed: %w", err)
	}
	balanceBefore = balanceAfter - amount

	// 4. Append the ledger event (accounting truth).
	meta := map[string]interface{}{"reason": reason}
	if paymentID != nil && *paymentID != "" {
		meta["payment_id"] = *paymentID
	}
	metaJSON, _ := json.Marshal(meta)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO usage_ledger (id, account_id, action_type, actor_id, tenant_id, transaction_type, amount, balance_before, balance_after, period_id, created_at, metadata)
		VALUES (gen_random_uuid(), $1, NULL, $2, $3, $4, $5, $6, $7, $8, NOW(), $9)
	`, accountID, "system", tenantID, txnType, amount, balanceBefore, balanceAfter, periodID, metaJSON); err != nil {
		return false, 0, fmt.Errorf("grant ledger insert failed: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return false, 0, fmt.Errorf("grant commit failed: %w", err)
	}
	return true, balanceAfter, nil
}

// FindByOwnerAndPeriod resolves the credit account for an owner within a billing period
// (tenant-scoped), fixing the legacy FindByOwner which ignored period_id.
func (r *CreditAccountRepository) FindByOwnerAndPeriod(ctx context.Context, ownerType domain.ActorType, ownerID, tenantID, periodID string) (*domain.CreditAccount, error) {
	const q = `
		SELECT account_id, owner_type, owner_id, tenant_id, plan_id, period_id, granted, reserved, consumed, created_at, updated_at
		FROM credit_accounts
		WHERE owner_type = $1 AND owner_id = $2 AND tenant_id = $3 AND period_id = $4
	`
	var acct domain.CreditAccount
	var createdAt, updatedAt time.Time
	err := r.db.QueryRowContext(ctx, q, ownerType, ownerID, tenantID, periodID).Scan(
		&acct.AccountID, &acct.OwnerType, &acct.OwnerID, &acct.TenantID, &acct.PlanID,
		&acct.PeriodID, &acct.Granted, &acct.Reserved, &acct.Consumed, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrUsageAccountNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find credit account by owner+period: %w", err)
	}
	acct.CreatedAt = toUnix(createdAt)
	acct.UpdatedAt = toUnix(updatedAt)
	return &acct, nil
}

// FindOrCreateByOwnerAndPeriod creates a credit account with a server-generated UUID
// (fixing the legacy string-ID/F1 bug) using an ON CONFLICT upsert (fixing the
// check-then-act race, F4), scoped to the exact period.
func (r *CreditAccountRepository) FindOrCreateByOwnerAndPeriod(ctx context.Context, ownerType domain.ActorType, ownerID, tenantID, planID, periodID string) (*domain.CreditAccount, error) {
	const q = `
		INSERT INTO credit_accounts (account_id, owner_type, owner_id, tenant_id, plan_id, period_id, granted, reserved, consumed, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, 0, 0, 0, NOW(), NOW())
		ON CONFLICT (owner_type, owner_id, tenant_id, period_id) DO UPDATE SET updated_at = NOW()
		RETURNING account_id, owner_type, owner_id, tenant_id, plan_id, period_id, granted, reserved, consumed, created_at, updated_at
	`
	var acct domain.CreditAccount
	var createdAt, updatedAt time.Time
	err := r.db.QueryRowContext(ctx, q, ownerType, ownerID, tenantID, planID, periodID).Scan(
		&acct.AccountID, &acct.OwnerType, &acct.OwnerID, &acct.TenantID, &acct.PlanID,
		&acct.PeriodID, &acct.Granted, &acct.Reserved, &acct.Consumed, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to find-or-create credit account: %w", err)
	}
	acct.CreatedAt = toUnix(createdAt)
	acct.UpdatedAt = toUnix(updatedAt)
	return &acct, nil
}
