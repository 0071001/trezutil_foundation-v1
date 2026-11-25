package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/lib/pq"
	"github.com/trezutil/trezutil/internal/application/adminmanage"
	"github.com/trezutil/trezutil/internal/domain"
)

// AdminManageStore implements adminmanage.Store: plans, platform subscriptions, credit grants, pricing rules,
// categories and the staff role count.
type AdminManageStore struct {
	db       *sql.DB
	personal *PersonalSpaceStore
	accounts *CreditAccountRepository
	cats     *CategoryRepository
}

// NewAdminManageStore creates the store.
func NewAdminManageStore(db *sql.DB) *AdminManageStore {
	return &AdminManageStore{db: db, personal: NewPersonalSpaceStore(db), accounts: NewCreditAccountRepository(db), cats: NewCategoryRepository(db)}
}

var _ adminmanage.Store = (*AdminManageStore)(nil)

// PlatformOrganization is the Trezutil organisation that owns the platform's plans: PLATFORM_ORGANIZATION_ID when
// set, otherwise the organisation scripts/ops/set-saspay.sh created ("Trezutil").
func (s *AdminManageStore) PlatformOrganization(ctx context.Context) (string, error) {
	if id := strings.TrimSpace(os.Getenv("PLATFORM_ORGANIZATION_ID")); id != "" {
		return id, nil
	}
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM organizations WHERE legal_name = 'Trezutil' AND display_name = 'Trezutil' ORDER BY created_at LIMIT 1`).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", adminmanage.ErrNoPlatformOrg
	}
	return id, err
}

func uniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}

// --- plans ---

const planCols = `id, name, slug, price_minor, currency, billing_cycle, quota_amount, credit_pack, status`

func scanAdminPlan(row interface{ Scan(...any) error }) (*adminmanage.Plan, error) {
	var p adminmanage.Plan
	err := row.Scan(&p.ID, &p.Name, &p.Slug, &p.PriceMinor, &p.Currency, &p.BillingCycle, &p.IncludedCredits, &p.CreditPack, &p.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, adminmanage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	p.Currency = strings.TrimSpace(p.Currency)
	return &p, nil
}

func (s *AdminManageStore) CreatePlan(ctx context.Context, tenantID string, p *adminmanage.Plan) error {
	err := s.db.QueryRowContext(ctx, `INSERT INTO plans (tenant_id, name, slug, price_minor, currency, billing_cycle, quota_amount, credit_pack, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		tenantID, p.Name, p.Slug, p.PriceMinor, p.Currency, p.BillingCycle, p.IncludedCredits, p.CreditPack, p.Status).Scan(&p.ID)
	if uniqueViolation(err) {
		return adminmanage.ErrConflict
	}
	return err
}

func (s *AdminManageStore) UpdatePlan(ctx context.Context, p *adminmanage.Plan) error {
	res, err := s.db.ExecContext(ctx, `UPDATE plans SET name = $2, slug = $3, price_minor = $4, currency = $5, billing_cycle = $6,
		quota_amount = $7, credit_pack = $8, status = $9, updated_at = NOW() WHERE id = $1`,
		p.ID, p.Name, p.Slug, p.PriceMinor, p.Currency, p.BillingCycle, p.IncludedCredits, p.CreditPack, p.Status)
	if uniqueViolation(err) {
		return adminmanage.ErrConflict
	}
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return adminmanage.ErrNotFound
	}
	return nil
}

func (s *AdminManageStore) FindPlan(ctx context.Context, id string) (*adminmanage.Plan, error) {
	return scanAdminPlan(s.db.QueryRowContext(ctx, `SELECT `+planCols+` FROM plans WHERE id::text = $1`, id))
}

func (s *AdminManageStore) PlanInUse(ctx context.Context, id string) (bool, error) {
	var used bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM subscriptions WHERE plan_id = $1)
		OR EXISTS (SELECT 1 FROM credit_purchases WHERE plan_id = $1)
		OR EXISTS (SELECT 1 FROM billing_periods WHERE plan_id = $1)`, id).Scan(&used)
	return used, err
}

func (s *AdminManageStore) DeletePlan(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM plans WHERE id = $1`, id)
	return err
}

// --- subscriptions (held by a person's account: owner_type 'customer', owner_id = user id) ---

const subCols = `id, plan_id, owner_id, status, current_period_id`

func scanAdminSub(row interface{ Scan(...any) error }) (*adminmanage.Subscription, error) {
	var sub adminmanage.Subscription
	err := row.Scan(&sub.ID, &sub.PlanID, &sub.UserID, &sub.Status, &sub.PeriodID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *AdminManageStore) FindSubscription(ctx context.Context, id string) (*adminmanage.Subscription, error) {
	return scanAdminSub(s.db.QueryRowContext(ctx, `SELECT `+subCols+` FROM subscriptions WHERE id::text = $1`, id))
}

func (s *AdminManageStore) FindSubscriptionByUser(ctx context.Context, tenantID, userID string) (*adminmanage.Subscription, error) {
	return scanAdminSub(s.db.QueryRowContext(ctx, `SELECT `+subCols+` FROM subscriptions WHERE tenant_id = $1 AND owner_type = 'customer' AND owner_id = $2`, tenantID, userID))
}

func (s *AdminManageStore) CreateSubscription(ctx context.Context, tenantID string, sub *adminmanage.Subscription) error {
	return s.db.QueryRowContext(ctx, `INSERT INTO subscriptions (tenant_id, plan_id, owner_type, owner_id, status, current_period_id)
		VALUES ($1, $2, 'customer', $3, $4, $5) RETURNING id`, tenantID, sub.PlanID, sub.UserID, sub.Status, sub.PeriodID).Scan(&sub.ID)
}

func (s *AdminManageStore) UpdateSubscription(ctx context.Context, sub *adminmanage.Subscription) error {
	_, err := s.db.ExecContext(ctx, `UPDATE subscriptions SET plan_id = $2, status = $3, current_period_id = $4, cancel_at_period_end = false, updated_at = NOW() WHERE id = $1`,
		sub.ID, sub.PlanID, sub.Status, sub.PeriodID)
	return err
}

func (s *AdminManageStore) ActiveSubscriptions(ctx context.Context, tenantID string) ([]adminmanage.Subscription, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+subCols+` FROM subscriptions WHERE tenant_id = $1 AND owner_type = 'customer' AND status = 'active'`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []adminmanage.Subscription{}
	for rows.Next() {
		var sub adminmanage.Subscription
		if err := rows.Scan(&sub.ID, &sub.PlanID, &sub.UserID, &sub.Status, &sub.PeriodID); err != nil {
			return nil, err
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

// --- credits ---

// UserCreditAccount resolves the account a person's metered actions are charged to: their personal space's account
// for the period (as UsageService.FindOrCreateCreditAccountByOwner does for a customer acting for themselves).
func (s *AdminManageStore) UserCreditAccount(ctx context.Context, userID, planID, periodID string) (string, string, error) {
	space, err := s.personal.Ensure(ctx, userID)
	if err != nil {
		return "", "", fmt.Errorf("personal space: %w", err)
	}
	acct, err := s.accounts.FindOrCreateByOwnerAndPeriod(ctx, domain.ActorTypeCustomer, userID, space, planID, periodID)
	if err != nil {
		return "", "", err
	}
	return acct.AccountID, space, nil
}

func (s *AdminManageStore) Grant(ctx context.Context, accountID, key string, source domain.CreditGrantSource, amount int64, tenantID, periodID, reason string) (bool, error) {
	granted, _, err := s.accounts.GrantCreditsTx(ctx, accountID, key, source, amount, tenantID, periodID, nil, reason)
	return granted, err
}

// --- pricing rules ---

const policyCols = `id, action_type, channel, actor_type, cost, active`

func scanPolicy(row interface{ Scan(...any) error }) (*adminmanage.Policy, error) {
	var p adminmanage.Policy
	err := row.Scan(&p.ID, &p.ActionType, &p.Channel, &p.ActorType, &p.Cost, &p.Active)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *AdminManageStore) FindPolicy(ctx context.Context, id string) (*adminmanage.Policy, error) {
	return scanPolicy(s.db.QueryRowContext(ctx, `SELECT `+policyCols+` FROM usage_policies WHERE id::text = $1`, id))
}

func (s *AdminManageStore) ActivePolicyFor(ctx context.Context, actionType string) (*adminmanage.Policy, error) {
	return scanPolicy(s.db.QueryRowContext(ctx, `SELECT `+policyCols+` FROM usage_policies WHERE action_type = $1 AND active = true LIMIT 1`, actionType))
}

func (s *AdminManageStore) CreatePolicy(ctx context.Context, p *adminmanage.Policy) error {
	return s.db.QueryRowContext(ctx, `INSERT INTO usage_policies (action_type, channel, actor_type, cost, active) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		p.ActionType, p.Channel, p.ActorType, p.Cost, p.Active).Scan(&p.ID)
}

func (s *AdminManageStore) UpdatePolicy(ctx context.Context, p *adminmanage.Policy) error {
	_, err := s.db.ExecContext(ctx, `UPDATE usage_policies SET action_type = $2, channel = $3, actor_type = $4, cost = $5, active = $6, updated_at = NOW() WHERE id = $1`,
		p.ID, p.ActionType, p.Channel, p.ActorType, p.Cost, p.Active)
	return err
}

func (s *AdminManageStore) DeletePolicy(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM usage_policies WHERE id = $1`, id)
	return err
}

// --- categories ---

func (s *AdminManageStore) FindCategory(ctx context.Context, id string) (*adminmanage.Category, error) {
	c, err := s.cats.FindByID(ctx, id)
	if err != nil || c == nil {
		return nil, adminmanage.ErrNotFound
	}
	return &adminmanage.Category{ID: c.ID, Name: c.Name, Slug: c.Slug, ParentID: c.ParentID}, nil
}

func (s *AdminManageStore) CategoryUsage(ctx context.Context, id string) (int, int, error) {
	var listings, children int
	err := s.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM listings WHERE category_id = $1), (SELECT COUNT(*) FROM categories WHERE parent_id = $1)`, id).Scan(&listings, &children)
	return listings, children, err
}

func (s *AdminManageStore) UpdateCategory(ctx context.Context, c *adminmanage.Category) error {
	_, err := s.db.ExecContext(ctx, `UPDATE categories SET name = $2, slug = $3, parent_id = $4, updated_at = NOW() WHERE id = $1`, c.ID, c.Name, c.Slug, c.ParentID)
	if uniqueViolation(err) {
		return adminmanage.ErrConflict
	}
	return err
}

func (s *AdminManageStore) DeleteCategory(ctx context.Context, id string) error {
	return s.cats.Delete(ctx, id)
}

// --- roles ---

func (s *AdminManageStore) CountRole(ctx context.Context, role domain.Role) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM user_roles ur JOIN roles r ON r.id = ur.role_id JOIN users u ON u.id = ur.user_id
		WHERE r.name = $1 AND u.status = 'active'`, string(role)).Scan(&n)
	return n, err
}

// ListSubscriptions lists the platform's subscriptions with the account and plan names, newest first.
func (s *AdminManageStore) ListSubscriptions(ctx context.Context, tenantID string) ([]adminmanage.SubscriptionRow, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT s.id, s.plan_id, p.name, s.owner_id, COALESCE(u.display_name, ''), COALESCE(u.email, ''), COALESCE(u.phone, ''),
			s.status, s.current_period_id, s.started_at
		FROM subscriptions s JOIN plans p ON p.id = s.plan_id LEFT JOIN users u ON u.id::text = s.owner_id
		WHERE s.tenant_id = $1 AND s.owner_type = 'customer'
		ORDER BY s.created_at DESC LIMIT 500`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []adminmanage.SubscriptionRow{}
	for rows.Next() {
		var r adminmanage.SubscriptionRow
		if err := rows.Scan(&r.ID, &r.PlanID, &r.PlanName, &r.UserID, &r.UserName, &r.UserEmail, &r.UserPhone, &r.Status, &r.PeriodID, &r.StartedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListPolicies lists every pricing rule with its id.
func (s *AdminManageStore) ListPolicies(ctx context.Context) ([]adminmanage.Policy, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+policyCols+` FROM usage_policies ORDER BY action_type, active DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []adminmanage.Policy{}
	for rows.Next() {
		var p adminmanage.Policy
		if err := rows.Scan(&p.ID, &p.ActionType, &p.Channel, &p.ActorType, &p.Cost, &p.Active); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
