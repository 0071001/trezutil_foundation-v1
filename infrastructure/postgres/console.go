package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/trezutil/trezutil/internal/domain"
)

/*
 * ConsoleRepository: the read models behind the seller workspace's money, customers, fulfilment,
 * availability, usage, integrations and team pages, and behind the administration console's listings,
 * payments, operations, providers, channels, security, AI and monetization pages (domain.ConsoleReader).
 *
 * Everything is read from the tables as they are, every filter is a bound parameter, and no query returns a
 * customer's phone number or email address. Provider rows never carry configuration or secret references.
 */

// ConsoleRepository implements domain.ConsoleReader.
type ConsoleRepository struct {
	db *sql.DB
}

// NewConsoleRepository creates a ConsoleRepository.
func NewConsoleRepository(db *sql.DB) *ConsoleRepository {
	return &ConsoleRepository{db: db}
}

var _ domain.ConsoleReader = (*ConsoleRepository)(nil)

func unixOrZero(t sql.NullTime) int64 {
	if !t.Valid {
		return 0
	}
	return t.Time.Unix()
}

// MaskPhone keeps the country code and the last two digits: "+243 ••• ••• 12".
func MaskPhone(phone string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)
	if len(digits) < 4 {
		return "•••"
	}
	prefix := ""
	if strings.HasPrefix(strings.TrimSpace(phone), "+") && len(digits) > 9 {
		prefix = "+" + digits[:len(digits)-9] + " "
	}
	return prefix + "••• ••• " + digits[len(digits)-2:]
}

// --- payments ----------------------------------------------------------------------------------------------

var paymentStatuses = map[string]bool{"created": true, "pending": true, "authorized": true, "captured": true, "failed": true, "cancelled": true, "expired": true, "refunded": true, "partially_refunded": true}

// Payments returns one page of payments, newest first: one business's, or the whole platform's.
func (r *ConsoleRepository) Payments(ctx context.Context, f domain.PaymentFilter, p domain.Page) ([]domain.PaymentRow, int, error) {
	status := f.Status
	if status != "" && !paymentStatuses[status] {
		return nil, 0, fmt.Errorf("unknown payment status")
	}
	q := strings.TrimSpace(f.Query)
	const from = `FROM payments p
		JOIN organizations org ON org.id = p.organization_id
		LEFT JOIN providers pr ON pr.id = p.provider_id`
	where := `WHERE ($1 = '' OR p.organization_id::text = $1) AND ($2 = '' OR p.status = $2)
		AND ($3 = '' OR p.id::text ILIKE $4 ESCAPE '\' OR p.order_id::text ILIKE $4 ESCAPE '\' OR org.display_name ILIKE $5 ESCAPE '\')`
	prefix := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
	args := []interface{}{f.OrganizationID, status, q, prefix, likePattern(q)}

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) `+from+` `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count payments: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.order_id, p.organization_id, org.display_name, p.amount_minor, p.currency, p.payment_method_type,
		       COALESCE(pr.code, ''), p.status, p.refunded_amount_minor, COALESCE(p.failure_reason, ''), p.created_at, p.captured_at
		`+from+` `+where+`
		ORDER BY p.created_at DESC, p.id
		LIMIT $6 OFFSET $7`, append(args, p.Limit, p.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list payments: %w", err)
	}
	defer rows.Close()
	out := []domain.PaymentRow{}
	for rows.Next() {
		var x domain.PaymentRow
		var created time.Time
		var captured sql.NullTime
		if err := rows.Scan(&x.ID, &x.OrderID, &x.OrganizationID, &x.OrganizationName, &x.AmountMinor, &x.Currency, &x.Method,
			&x.ProviderCode, &x.Status, &x.RefundedMinor, &x.FailureReason, &created, &captured); err != nil {
			return nil, 0, err
		}
		x.CreatedAt, x.CapturedAt = created.Unix(), unixOrZero(captured)
		out = append(out, x)
	}
	return out, total, rows.Err()
}

// MoneyTotals sums, per currency, the payments of a business (or the platform, organizationID "") created in
// [from, to) and the value of its orders that count as revenue in the same window.
func (r *ConsoleRepository) MoneyTotals(ctx context.Context, organizationID string, from, to int64) ([]domain.MoneyTotals, error) {
	byCur := map[string]*domain.MoneyTotals{}
	get := func(c string) *domain.MoneyTotals {
		if byCur[c] == nil {
			byCur[c] = &domain.MoneyTotals{Currency: c}
		}
		return byCur[c]
	}
	start, end := time.Unix(from, 0), time.Unix(to, 0)
	rows, err := r.db.QueryContext(ctx, `
		SELECT currency,
		       COALESCE(sum(amount_minor) FILTER (WHERE status IN ('captured', 'refunded', 'partially_refunded')), 0),
		       COALESCE(sum(refunded_amount_minor), 0),
		       count(*) FILTER (WHERE status IN ('captured', 'refunded', 'partially_refunded')),
		       count(*) FILTER (WHERE status = 'failed')
		FROM payments
		WHERE created_at >= $1 AND created_at < $2 AND ($3 = '' OR organization_id::text = $3)
		GROUP BY currency`, start, end, organizationID)
	if err != nil {
		return nil, fmt.Errorf("payment totals: %w", err)
	}
	for rows.Next() {
		var c string
		var captured, refunded int64
		var n, failed int
		if err := rows.Scan(&c, &captured, &refunded, &n, &failed); err != nil {
			rows.Close()
			return nil, err
		}
		m := get(strings.TrimSpace(c))
		m.CapturedMinor, m.RefundedMinor, m.Payments, m.Failed = captured, refunded, n, failed
		m.NetMinor = captured - refunded
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = r.db.QueryContext(ctx, `
		SELECT currency, COALESCE(sum(total_amount), 0), count(*)
		FROM orders
		WHERE created_at >= $1 AND created_at < $2 AND ($3 = '' OR organization_id::text = $3) AND status = ANY($4)
		GROUP BY currency`, start, end, organizationID, pq.Array(domain.EarningOrderStatuses))
	if err != nil {
		return nil, fmt.Errorf("earned totals: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var c string
		var earned int64
		var n int
		if err := rows.Scan(&c, &earned, &n); err != nil {
			return nil, err
		}
		m := get(strings.TrimSpace(c))
		m.EarnedMinor, m.Orders = earned, n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]domain.MoneyTotals, 0, len(byCur))
	for _, m := range byCur {
		out = append(out, *m)
	}
	sortTotals(out)
	return out, nil
}

func sortTotals(t []domain.MoneyTotals) {
	for i := 1; i < len(t); i++ {
		for j := i; j > 0 && t[j].Currency < t[j-1].Currency; j-- {
			t[j], t[j-1] = t[j-1], t[j]
		}
	}
}

// --- customers ---------------------------------------------------------------------------------------------

// Customers returns the people who ordered or booked from a business, most recent first.
func (r *ConsoleRepository) Customers(ctx context.Context, organizationID, query string, p domain.Page) ([]domain.CustomerRow, int, error) {
	q := strings.TrimSpace(query)
	const base = `
		WITH activity AS (
			SELECT customer_id, created_at, 1 AS is_order, 0 AS is_booking FROM orders WHERE organization_id = $1 AND status <> 'draft'
			UNION ALL
			SELECT customer_id, created_at, 0, 1 FROM reservations WHERE organization_id = $1
		), people AS (
			SELECT a.customer_id, sum(a.is_order) AS orders, sum(a.is_booking) AS bookings, min(a.created_at) AS first_at, max(a.created_at) AS last_at
			FROM activity a GROUP BY a.customer_id
		)
		SELECT pe.customer_id, COALESCE(u.display_name, ''), pe.orders, pe.bookings, pe.first_at, pe.last_at
		FROM people pe LEFT JOIN users u ON u.id = pe.customer_id
		WHERE ($2 = '' OR u.display_name ILIKE $3 ESCAPE '\' OR pe.customer_id::text ILIKE $4 ESCAPE '\')`
	prefix := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
	args := []interface{}{organizationID, q, likePattern(q), prefix}

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM (`+base+`) x`, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count customers: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, base+` ORDER BY pe.last_at DESC, pe.customer_id LIMIT $5 OFFSET $6`, append(args, p.Limit, p.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list customers: %w", err)
	}
	out := []domain.CustomerRow{}
	ids := []string{}
	for rows.Next() {
		var c domain.CustomerRow
		var first, last time.Time
		if err := rows.Scan(&c.CustomerID, &c.DisplayName, &c.Orders, &c.Reservations, &first, &last); err != nil {
			rows.Close()
			return nil, 0, err
		}
		c.FirstAt, c.LastAt, c.Spent = first.Unix(), last.Unix(), map[string]int64{}
		out = append(out, c)
		ids = append(ids, c.CustomerID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(ids) == 0 {
		return out, total, nil
	}
	spent, err := r.db.QueryContext(ctx, `
		SELECT customer_id, currency, sum(total_amount) FROM orders
		WHERE organization_id = $1 AND customer_id::text = ANY($2) AND status = ANY($3)
		GROUP BY customer_id, currency`, organizationID, pq.Array(ids), pq.Array(domain.EarningOrderStatuses))
	if err != nil {
		return nil, 0, fmt.Errorf("customer spend: %w", err)
	}
	defer spent.Close()
	index := map[string]int{}
	for i, c := range out {
		index[c.CustomerID] = i
	}
	for spent.Next() {
		var id, cur string
		var sum int64
		if err := spent.Scan(&id, &cur, &sum); err != nil {
			return nil, 0, err
		}
		out[index[id]].Spent[strings.TrimSpace(cur)] = sum
	}
	return out, total, spent.Err()
}

// --- fulfilment --------------------------------------------------------------------------------------------

var fulfillmentStatuses = map[string]bool{"PENDING": true, "ACCEPTED": true, "IN_PROGRESS": true, "READY": true, "COMPLETED": true, "CANCELLED": true, "FAILED": true}

// Fulfillments returns one page of a business's fulfilments, open ones first, newest first within.
func (r *ConsoleRepository) Fulfillments(ctx context.Context, organizationID, status string, p domain.Page) ([]domain.FulfillmentRow, int, error) {
	if status != "" && status != "open" && !fulfillmentStatuses[status] {
		return nil, 0, fmt.Errorf("unknown fulfilment status")
	}
	const from = `FROM fulfillments f
		LEFT JOIN orders o ON o.id = f.order_id
		LEFT JOIN reservations rv ON rv.id = f.reservation_id
		LEFT JOIN listings l ON l.id = rv.listing_id
		LEFT JOIN users c ON c.id = COALESCE(o.customer_id, rv.customer_id)
		LEFT JOIN users a ON a.id = f.assigned_to`
	where := `WHERE f.tenant_id = $1 AND ($2 = '' OR f.status = $2 OR ($2 = 'open' AND f.status NOT IN ('COMPLETED', 'CANCELLED', 'FAILED')))`
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) `+from+` `+where, organizationID, status).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count fulfillments: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT f.id, f.reference, f.type, f.status, COALESCE(f.order_id::text, ''), COALESCE(f.reservation_id::text, ''),
		       COALESCE(o.items->0->>'Title', l.title, ''), COALESCE(o.status, rv.status, ''), COALESCE(c.display_name, ''),
		       COALESCE(f.tracking_ref, ''), COALESCE(f.assigned_to::text, ''), COALESCE(a.display_name, ''), f.created_at, f.updated_at
		`+from+` `+where+`
		ORDER BY (f.status IN ('COMPLETED', 'CANCELLED', 'FAILED')), f.created_at DESC, f.id
		LIMIT $3 OFFSET $4`, organizationID, status, p.Limit, p.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list fulfillments: %w", err)
	}
	defer rows.Close()
	out := []domain.FulfillmentRow{}
	for rows.Next() {
		var x domain.FulfillmentRow
		var created, updated time.Time
		if err := rows.Scan(&x.ID, &x.Reference, &x.Type, &x.Status, &x.OrderID, &x.ReservationID, &x.Title, &x.OrderStatus, &x.CustomerName,
			&x.TrackingRef, &x.AssignedTo, &x.AssignedName, &created, &updated); err != nil {
			return nil, 0, err
		}
		x.CreatedAt, x.UpdatedAt = created.Unix(), updated.Unix()
		out = append(out, x)
	}
	return out, total, rows.Err()
}

// --- availability ------------------------------------------------------------------------------------------

// Capacity returns every stock or capacity record of a business with its windows overlapping [from, to).
func (r *ConsoleRepository) Capacity(ctx context.Context, organizationID string, from, to int64) ([]domain.CapacityResource, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT i.id, i.listing_id, COALESCE(l.title, ''), COALESCE(l.listing_type, ''), i.resource_type, i.total, i.reserved, i.consumed
		FROM inventory_resources i LEFT JOIN listings l ON l.id = i.listing_id
		WHERE i.tenant_id = $1
		ORDER BY l.title, i.id`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list resources: %w", err)
	}
	out := []domain.CapacityResource{}
	index := map[string]int{}
	for rows.Next() {
		var x domain.CapacityResource
		if err := rows.Scan(&x.InventoryID, &x.ListingID, &x.ListingTitle, &x.ListingType, &x.ResourceType, &x.Total, &x.Reserved, &x.Consumed); err != nil {
			rows.Close()
			return nil, err
		}
		x.Windows = []domain.CapacitySlot{}
		index[x.InventoryID] = len(out)
		out = append(out, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	w, err := r.db.QueryContext(ctx, `
		SELECT cu.id, cu.inventory_id, cu.start_at, cu.end_at, cu.capacity, cu.reserved
		FROM capacity_units cu JOIN inventory_resources i ON i.id = cu.inventory_id
		WHERE i.tenant_id = $1 AND cu.end_at > $2 AND cu.start_at < $3
		ORDER BY cu.start_at
		LIMIT 5000`, organizationID, time.Unix(from, 0), time.Unix(to, 0))
	if err != nil {
		return nil, fmt.Errorf("list windows: %w", err)
	}
	defer w.Close()
	for w.Next() {
		var c domain.CapacitySlot
		var inv string
		var start, end time.Time
		if err := w.Scan(&c.ID, &inv, &start, &end, &c.Capacity, &c.Reserved); err != nil {
			return nil, err
		}
		c.StartAt, c.EndAt = start.Unix(), end.Unix()
		if i, ok := index[inv]; ok {
			out[i].Windows = append(out[i].Windows, c)
		}
	}
	return out, w.Err()
}

// --- usage, plans, providers, channels ---------------------------------------------------------------------

func (r *ConsoleRepository) policies(ctx context.Context) ([]domain.UsagePolicyRow, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT action_type, channel, actor_type, cost, active FROM usage_policies ORDER BY action_type, channel, actor_type`)
	if err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}
	defer rows.Close()
	out := []domain.UsagePolicyRow{}
	for rows.Next() {
		var p domain.UsagePolicyRow
		if err := rows.Scan(&p.ActionType, &p.Channel, &p.ActorType, &p.Cost, &p.Active); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

const subscriptionColumns = `s.id, s.tenant_id, COALESCE(org.display_name, ''), s.owner_type, s.status, s.current_period_id, s.cancel_at_period_end, s.started_at,
	p.id, p.name, p.slug, p.price_minor, p.currency, p.billing_cycle, p.quota_amount`

func scanSubscription(row interface{ Scan(...interface{}) error }) (*domain.SubscriptionRow, error) {
	var s domain.SubscriptionRow
	var started time.Time
	if err := row.Scan(&s.ID, &s.OrganizationID, &s.OrganizationName, &s.OwnerType, &s.Status, &s.PeriodID, &s.CancelAtPeriodEnd, &started,
		&s.PlanID, &s.PlanName, &s.PlanSlug, &s.PriceMinor, &s.Currency, &s.BillingCycle, &s.QuotaAmount); err != nil {
		return nil, err
	}
	s.StartedAt = started.Unix()
	s.Currency = strings.TrimSpace(s.Currency)
	return &s, nil
}

// Usage returns a business's credit accounts (most recent period first), its latest 50 ledger lines, its
// subscription if it has one, and the cost of each action.
func (r *ConsoleRepository) Usage(ctx context.Context, organizationID string) (*domain.UsageSummary, error) {
	out := &domain.UsageSummary{Accounts: []domain.CreditAccountRow{}, Ledger: []domain.UsageEntry{}}
	rows, err := r.db.QueryContext(ctx, `
		SELECT account_id, owner_type, owner_id, period_id, COALESCE(plan_id, ''), granted, reserved, consumed, updated_at
		FROM credit_accounts WHERE tenant_id = $1 ORDER BY period_id DESC, updated_at DESC LIMIT 24`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	for rows.Next() {
		var a domain.CreditAccountRow
		var updated time.Time
		if err := rows.Scan(&a.AccountID, &a.OwnerType, &a.OwnerID, &a.PeriodID, &a.PlanID, &a.Granted, &a.Reserved, &a.Consumed, &updated); err != nil {
			rows.Close()
			return nil, err
		}
		a.UpdatedAt = updated.Unix()
		out.Accounts = append(out.Accounts, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = r.db.QueryContext(ctx, `
		SELECT id, COALESCE(action_type, ''), transaction_type, amount, balance_after, period_id, created_at
		FROM usage_ledger WHERE tenant_id = $1 ORDER BY created_at DESC, id LIMIT 50`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list ledger: %w", err)
	}
	for rows.Next() {
		var e domain.UsageEntry
		var created time.Time
		if err := rows.Scan(&e.ID, &e.ActionType, &e.TransactionType, &e.Amount, &e.BalanceAfter, &e.PeriodID, &created); err != nil {
			rows.Close()
			return nil, err
		}
		e.CreatedAt = created.Unix()
		out.Ledger = append(out.Ledger, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	s, err := scanSubscription(r.db.QueryRowContext(ctx, `
		SELECT `+subscriptionColumns+`
		FROM subscriptions s JOIN plans p ON p.id = s.plan_id LEFT JOIN organizations org ON org.id = s.tenant_id
		WHERE s.tenant_id = $1 AND s.owner_type = 'organization'
		ORDER BY s.updated_at DESC LIMIT 1`, organizationID))
	switch {
	case err == sql.ErrNoRows:
	case err != nil:
		return nil, fmt.Errorf("read subscription: %w", err)
	default:
		out.Subscription = s
	}
	if out.Policies, err = r.policies(ctx); err != nil {
		return nil, err
	}
	return out, nil
}

// Providers returns the provider connections of a business (or every business, organizationID ""), with
// their executions since the given time. Configuration and secret references are never read.
func (r *ConsoleRepository) Providers(ctx context.Context, organizationID string, since int64) ([]domain.ProviderRow, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT pr.id, pr.organization_id, COALESCE(org.display_name, ''), pr.code, pr.name, pr.type, pr.status, pr.health_status,
		       pr.capabilities, pr.priority, pr.updated_at,
		       count(e.id), count(e.id) FILTER (WHERE e.status = 'FAILED'),
		       COALESCE(percentile_disc(0.5) WITHIN GROUP (ORDER BY e.duration_ms), 0),
		       COALESCE(percentile_disc(0.95) WITHIN GROUP (ORDER BY e.duration_ms), 0),
		       max(e.started_at) FILTER (WHERE e.status = 'FAILED')
		FROM providers pr
		LEFT JOIN organizations org ON org.id = pr.organization_id
		LEFT JOIN provider_executions e ON e.provider_id = pr.id AND e.started_at >= $2
		WHERE ($1 = '' OR pr.organization_id::text = $1)
		GROUP BY pr.id, org.display_name
		ORDER BY org.display_name, pr.priority, pr.name`, organizationID, time.Unix(since, 0))
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	out := []domain.ProviderRow{}
	index := map[string]int{}
	for rows.Next() {
		var p domain.ProviderRow
		var updated time.Time
		var lastFail sql.NullTime
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.OrganizationName, &p.Code, &p.Name, &p.Type, &p.Status, &p.HealthStatus,
			pq.Array(&p.Capabilities), &p.Priority, &updated, &p.Attempts, &p.Failures, &p.P50Ms, &p.P95Ms, &lastFail); err != nil {
			rows.Close()
			return nil, err
		}
		p.UpdatedAt, p.LastFailure = updated.Unix(), unixOrZero(lastFail)
		if p.Capabilities == nil {
			p.Capabilities = []string{}
		}
		index[p.ID] = len(out)
		out = append(out, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// The last error code of each provider that failed in the window.
	errs, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT ON (e.provider_id) e.provider_id, COALESCE(e.error_code, '')
		FROM provider_executions e JOIN providers pr ON pr.id = e.provider_id
		WHERE e.status = 'FAILED' AND e.started_at >= $2 AND ($1 = '' OR pr.organization_id::text = $1)
		ORDER BY e.provider_id, e.started_at DESC`, organizationID, time.Unix(since, 0))
	if err != nil {
		return nil, fmt.Errorf("provider errors: %w", err)
	}
	defer errs.Close()
	for errs.Next() {
		var id, code string
		if err := errs.Scan(&id, &code); err != nil {
			return nil, err
		}
		if i, ok := index[id]; ok {
			out[i].LastError = code
		}
	}
	return out, errs.Err()
}

func maskExternal(id string) string {
	if len(id) <= 4 {
		return "••••"
	}
	return "••••" + id[len(id)-4:]
}

// ChannelAccounts returns the messaging channels linked to a business, external ids masked.
func (r *ConsoleRepository) ChannelAccounts(ctx context.Context, organizationID string) ([]domain.ChannelAccountRow, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, channel, external_id, status, created_at FROM channel_accounts WHERE organization_id = $1 ORDER BY channel, created_at`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list channel accounts: %w", err)
	}
	defer rows.Close()
	out := []domain.ChannelAccountRow{}
	for rows.Next() {
		var c domain.ChannelAccountRow
		var created time.Time
		if err := rows.Scan(&c.ID, &c.Channel, &c.ExternalID, &c.Status, &created); err != nil {
			return nil, err
		}
		c.ExternalID, c.CreatedAt = maskExternal(c.ExternalID), created.Unix()
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetProviderStatus moves a provider from one status to another only if it is still in the first one, and
// returns its business.
func (r *ConsoleRepository) SetProviderStatus(ctx context.Context, providerID, from, to string) (string, error) {
	var org string
	err := r.db.QueryRowContext(ctx, `
		UPDATE providers SET status = $3, version = version + 1, updated_at = NOW()
		WHERE id::text = $1 AND status = $2
		RETURNING organization_id`, providerID, from, to).Scan(&org)
	if err == sql.ErrNoRows {
		return "", domain.ErrAccountState
	}
	return org, err
}

// --- team --------------------------------------------------------------------------------------------------

// Team returns the members of a business, owners first.
func (r *ConsoleRepository) Team(ctx context.Context, organizationID string) ([]domain.TeamMember, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT m.user_id, COALESCE(u.display_name, ''), COALESCE(u.phone, ''), m.membership_role, m.joined_at, m.status
		FROM organization_members m JOIN users u ON u.id = m.user_id
		WHERE m.organization_id = $1
		ORDER BY CASE m.membership_role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 WHEN 'manager' THEN 2 ELSE 3 END, m.joined_at`, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list team: %w", err)
	}
	defer rows.Close()
	out := []domain.TeamMember{}
	for rows.Next() {
		var m domain.TeamMember
		var phone string
		var joined time.Time
		if err := rows.Scan(&m.UserID, &m.DisplayName, &phone, &m.Role, &joined, &m.Status); err != nil {
			return nil, err
		}
		m.MaskedPhone, m.JoinedAt = MaskPhone(phone), joined.Unix()
		out = append(out, m)
	}
	return out, rows.Err()
}

// --- administration ----------------------------------------------------------------------------------------

var adminListingStatuses = map[string]bool{"draft": true, "pending_review": true, "published": true, "paused": true, "sold": true, "closed": true, "expired": true, "suspended": true}

// AdminListings returns one page of listings across businesses, newest change first, with quality flags.
func (r *ConsoleRepository) AdminListings(ctx context.Context, f domain.AdminListingFilter, p domain.Page) ([]domain.AdminListing, int, error) {
	if f.Status != "" && !adminListingStatuses[f.Status] {
		return nil, 0, fmt.Errorf("unknown listing status")
	}
	if f.Flag != "" && f.Flag != "no_image" && f.Flag != "no_price" {
		return nil, 0, fmt.Errorf("unknown flag")
	}
	q := strings.TrimSpace(f.Query)
	const from = `FROM listings l
		JOIN organizations org ON org.id = l.organization_id
		LEFT JOIN categories cat ON cat.id = l.category_id
		LEFT JOIN LATERAL (SELECT count(*) AS n FROM listing_media m WHERE m.listing_id = l.id) media ON true`
	where := `WHERE ($1 = '' OR l.id::text ILIKE $2 ESCAPE '\' OR l.title ILIKE $3 ESCAPE '\' OR org.display_name ILIKE $3 ESCAPE '\')
		AND ($4 = '' OR l.status = $4) AND ($5 = '' OR l.vertical = $5)
		AND ($6 = '' OR ($6 = 'no_image' AND media.n = 0) OR ($6 = 'no_price' AND l.price_amount <= 0))`
	prefix := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
	args := []interface{}{q, prefix, likePattern(q), f.Status, f.Vertical, f.Flag}
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) `+from+` `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count listings: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT l.id, l.title, l.status, COALESCE(l.vertical, ''), l.listing_type, l.price_amount, l.price_currency,
		       l.organization_id, org.display_name, COALESCE(cat.name, ''), media.n, l.created_at, l.updated_at
		`+from+` `+where+`
		ORDER BY l.updated_at DESC, l.id
		LIMIT $7 OFFSET $8`, append(args, p.Limit, p.Offset)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list listings: %w", err)
	}
	defer rows.Close()
	out := []domain.AdminListing{}
	for rows.Next() {
		var x domain.AdminListing
		var created, updated time.Time
		if err := rows.Scan(&x.ID, &x.Title, &x.Status, &x.Vertical, &x.ListingType, &x.PriceAmount, &x.Currency,
			&x.OrganizationID, &x.OrganizationName, &x.CategoryName, &x.Images, &created, &updated); err != nil {
			return nil, 0, err
		}
		x.CreatedAt, x.UpdatedAt, x.Currency = created.Unix(), updated.Unix(), strings.TrimSpace(x.Currency)
		x.Flags = []string{}
		if x.Images == 0 {
			x.Flags = append(x.Flags, "no_image")
		}
		if x.PriceAmount <= 0 {
			x.Flags = append(x.Flags, "no_price")
		}
		out = append(out, x)
	}
	return out, total, rows.Err()
}

// Exceptions gathers what went wrong since the given time: failed payments, webhooks that failed or never
// finished (WhatsApp and payments), and orders paid before stuckBefore that nobody confirmed.
func (r *ConsoleRepository) Exceptions(ctx context.Context, since, stuckBefore int64, limit int) (*domain.Exceptions, error) {
	out := &domain.Exceptions{Counts: map[string]int{}, Recent: []domain.ExceptionRow{}}
	s, stuck := time.Unix(since, 0), time.Unix(stuckBefore, 0)
	type source struct {
		kind  string
		count string
		rows  string
		args  []interface{}
	}
	sources := []source{
		{"failed_payment",
			`SELECT count(*) FROM payments WHERE status = 'failed' AND COALESCE(failed_at, updated_at) >= $1`,
			`SELECT p.id, p.order_id::text, p.status, COALESCE(p.failure_reason, ''), org.display_name, COALESCE(p.failed_at, p.updated_at)
			 FROM payments p JOIN organizations org ON org.id = p.organization_id
			 WHERE p.status = 'failed' AND COALESCE(p.failed_at, p.updated_at) >= $1 ORDER BY 6 DESC LIMIT $2`,
			[]interface{}{s}},
		{"failed_webhook",
			`SELECT count(*) FROM webhook_events WHERE (status = 'failed' OR (status IN ('pending', 'processing') AND created_at < NOW() - interval '15 minutes')) AND created_at >= $1`,
			`SELECT id, channel, status, COALESCE(error_message, ''), '', created_at FROM webhook_events
			 WHERE (status = 'failed' OR (status IN ('pending', 'processing') AND created_at < NOW() - interval '15 minutes')) AND created_at >= $1
			 ORDER BY created_at DESC LIMIT $2`,
			[]interface{}{s}},
		{"failed_payment_webhook",
			`SELECT count(*) FROM payment_webhook_events WHERE (status = 'failed' OR (status IN ('pending', 'processing') AND created_at < NOW() - interval '15 minutes')) AND created_at >= $1`,
			`SELECT w.id, w.provider_code || ' · ' || w.event_type, w.status, COALESCE(w.error_message, ''), COALESCE(org.display_name, ''), w.created_at
			 FROM payment_webhook_events w LEFT JOIN organizations org ON org.id = w.organization_id
			 WHERE (w.status = 'failed' OR (w.status IN ('pending', 'processing') AND w.created_at < NOW() - interval '15 minutes')) AND w.created_at >= $1
			 ORDER BY w.created_at DESC LIMIT $2`,
			[]interface{}{s}},
		{"unconfirmed_order",
			`SELECT count(*) FROM orders WHERE status = 'paid' AND updated_at < $1`,
			`SELECT o.id, COALESCE(o.items->0->>'Title', ''), o.status, '', org.display_name, o.updated_at
			 FROM orders o JOIN organizations org ON org.id = o.organization_id
			 WHERE o.status = 'paid' AND o.updated_at < $1 ORDER BY o.updated_at LIMIT $2`,
			[]interface{}{stuck}},
	}
	for _, src := range sources {
		var n int
		if err := r.db.QueryRowContext(ctx, src.count, src.args...).Scan(&n); err != nil {
			return nil, fmt.Errorf("count %s: %w", src.kind, err)
		}
		out.Counts[src.kind] = n
		rows, err := r.db.QueryContext(ctx, src.rows, append(src.args, limit)...)
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", src.kind, err)
		}
		for rows.Next() {
			x := domain.ExceptionRow{Kind: src.kind}
			var at time.Time
			if err := rows.Scan(&x.ID, &x.Reference, &x.Status, &x.Detail, &x.Business, &at); err != nil {
				rows.Close()
				return nil, err
			}
			if len(x.Detail) > 300 {
				x.Detail = x.Detail[:300]
			}
			x.At = at.Unix()
			out.Recent = append(out.Recent, x)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ChannelHealth reports, per inbound channel (WhatsApp and the other conversation channels, and each payment
// provider's webhooks), the events of the last 24 hours.
func (r *ConsoleRepository) ChannelHealth(ctx context.Context, now int64) ([]domain.ChannelHealth, error) {
	since := time.Unix(now, 0).Add(-24 * time.Hour)
	week := time.Unix(now, 0).Add(-7 * 24 * time.Hour)
	out := []domain.ChannelHealth{}
	rows, err := r.db.QueryContext(ctx, `
		WITH channels AS (
			SELECT channel FROM webhook_events UNION SELECT channel FROM channel_accounts UNION SELECT channel FROM conversations
		)
		SELECT c.channel,
		       (SELECT count(*) FROM webhook_events w WHERE w.channel = c.channel AND w.created_at >= $1),
		       (SELECT count(*) FROM webhook_events w WHERE w.channel = c.channel AND w.created_at >= $1 AND w.status = 'failed'),
		       (SELECT count(*) FROM webhook_events w WHERE w.channel = c.channel AND w.status IN ('pending', 'processing')),
		       (SELECT max(w.created_at) FROM webhook_events w WHERE w.channel = c.channel),
		       (SELECT max(w.created_at) FROM webhook_events w WHERE w.channel = c.channel AND w.status = 'failed'),
		       COALESCE((SELECT w.error_message FROM webhook_events w WHERE w.channel = c.channel AND w.status = 'failed' ORDER BY w.created_at DESC LIMIT 1), ''),
		       (SELECT count(*) FROM channel_accounts a WHERE a.channel = c.channel),
		       (SELECT count(*) FROM conversations v WHERE v.channel = c.channel AND v.created_at >= $2)
		FROM channels c ORDER BY c.channel`, since, week)
	if err != nil {
		return nil, fmt.Errorf("channel health: %w", err)
	}
	for rows.Next() {
		var h domain.ChannelHealth
		var last, lastFail sql.NullTime
		if err := rows.Scan(&h.Channel, &h.Events24h, &h.Failed24h, &h.Pending, &last, &lastFail, &h.LastError, &h.Accounts, &h.Conversations7d); err != nil {
			rows.Close()
			return nil, err
		}
		h.LastEventAt, h.LastFailureAt = unixOrZero(last), unixOrZero(lastFail)
		if len(h.LastError) > 200 {
			h.LastError = h.LastError[:200]
		}
		out = append(out, h)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = r.db.QueryContext(ctx, `
		SELECT 'payments:' || provider_code,
		       count(*) FILTER (WHERE created_at >= $1),
		       count(*) FILTER (WHERE created_at >= $1 AND status = 'failed'),
		       count(*) FILTER (WHERE status IN ('pending', 'processing')),
		       max(created_at), max(created_at) FILTER (WHERE status = 'failed')
		FROM payment_webhook_events GROUP BY provider_code ORDER BY provider_code`, since)
	if err != nil {
		return nil, fmt.Errorf("payment webhook health: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var h domain.ChannelHealth
		var last, lastFail sql.NullTime
		if err := rows.Scan(&h.Channel, &h.Events24h, &h.Failed24h, &h.Pending, &last, &lastFail); err != nil {
			return nil, err
		}
		h.LastEventAt, h.LastFailureAt = unixOrZero(last), unixOrZero(lastFail)
		out = append(out, h)
	}
	return out, rows.Err()
}

// StaffAccounts lists the accounts holding a staff role, with their sign-in state.
func (r *ConsoleRepository) StaffAccounts(ctx context.Context) ([]domain.StaffAccount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT u.id, COALESCE(u.display_name, ''), COALESCE(u.email, ''), array_agg(r.name ORDER BY r.name), u.status, u.last_login_at,
		       u.failed_login_attempts, u.locked_until, u.password_changed_at
		FROM users u JOIN user_roles ur ON ur.user_id = u.id JOIN roles r ON r.id = ur.role_id
		WHERE u.id IN (SELECT ur2.user_id FROM user_roles ur2 JOIN roles r2 ON r2.id = ur2.role_id WHERE r2.name IN ('admin', 'operator', 'moderator'))
		GROUP BY u.id
		ORDER BY u.last_login_at DESC NULLS LAST, u.created_at`)
	if err != nil {
		return nil, fmt.Errorf("list staff: %w", err)
	}
	defer rows.Close()
	out := []domain.StaffAccount{}
	for rows.Next() {
		var s domain.StaffAccount
		var last, locked, pw sql.NullTime
		if err := rows.Scan(&s.UserID, &s.DisplayName, &s.Email, pq.Array(&s.Roles), &s.Status, &last, &s.FailedAttempts, &locked, &pw); err != nil {
			return nil, err
		}
		s.LastLoginAt, s.LockedUntil, s.PasswordSetAt = unixOrZero(last), unixOrZero(locked), unixOrZero(pw)
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *ConsoleRepository) countBy(ctx context.Context, query string, args ...interface{}) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		out[k] = n
	}
	return out, rows.Err()
}

// ConversationSummary is the assistant's activity over the last days (Africa/Kinshasa calendar days).
func (r *ConsoleRepository) ConversationSummary(ctx context.Context, now int64, days int) (*domain.ConversationSummary, error) {
	since := time.Unix(now, 0).Add(-time.Duration(days) * 24 * time.Hour)
	out := &domain.ConversationSummary{Days: days, PerDay: []domain.DayCount{}}
	var err error
	if err = r.db.QueryRowContext(ctx, `SELECT count(*), count(DISTINCT customer_id) FROM conversations WHERE created_at >= $1`, since).Scan(&out.Conversations, &out.ActiveCustomers); err != nil {
		return nil, fmt.Errorf("count conversations: %w", err)
	}
	if out.ByChannel, err = r.countBy(ctx, `SELECT channel, count(*) FROM conversations WHERE created_at >= $1 GROUP BY 1`, since); err != nil {
		return nil, fmt.Errorf("conversations by channel: %w", err)
	}
	if out.ByState, err = r.countBy(ctx, `SELECT state, count(*) FROM conversations WHERE created_at >= $1 GROUP BY 1`, since); err != nil {
		return nil, fmt.Errorf("conversations by state: %w", err)
	}
	dir, err := r.countBy(ctx, `SELECT lower(direction), count(*) FROM messages WHERE created_at >= $1 GROUP BY 1`, since)
	if err != nil {
		return nil, fmt.Errorf("messages: %w", err)
	}
	out.MessagesIn, out.MessagesOut = dir["inbound"]+dir["in"], dir["outbound"]+dir["out"]
	if out.Confirmations, err = r.countBy(ctx, `SELECT state, count(*) FROM conversation_confirmations WHERE created_at >= $1 GROUP BY 1`, since); err != nil {
		return nil, fmt.Errorf("confirmations: %w", err)
	}
	if out.ResultsByType, err = r.countBy(ctx, `SELECT ref_type, count(*) FROM conversation_result_items WHERE created_at >= $1 GROUP BY 1`, since); err != nil {
		return nil, fmt.Errorf("results: %w", err)
	}
	if err = r.db.QueryRowContext(ctx, `SELECT count(*) FROM ai_execution_records`).Scan(&out.AIExecutions); err != nil {
		return nil, fmt.Errorf("ai executions: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+dayExpr("created_at")+`, count(*) FROM conversations WHERE created_at >= $1 GROUP BY 1 ORDER BY 1`, since)
	if err != nil {
		return nil, fmt.Errorf("conversations per day: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var d domain.DayCount
		if err := rows.Scan(&d.Date, &d.Count); err != nil {
			return nil, err
		}
		out.PerDay = append(out.PerDay, d)
	}
	return out, rows.Err()
}

// Monetization returns the plans, the subscriptions and what the credit ledger recorded since the given time.
func (r *ConsoleRepository) Monetization(ctx context.Context, since int64) (*domain.MonetizationSummary, error) {
	out := &domain.MonetizationSummary{Plans: []domain.PlanRow{}, Subscriptions: []domain.SubscriptionRow{}, LedgerByType: map[string]int64{}}
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.tenant_id, p.name, p.slug, p.price_minor, p.currency, p.billing_cycle, p.quota_amount, p.credit_pack, p.status,
		       (SELECT count(*) FROM subscriptions s WHERE s.plan_id = p.id AND s.status = 'active')
		FROM plans p ORDER BY p.status, p.price_minor, p.name`)
	if err != nil {
		return nil, fmt.Errorf("list plans: %w", err)
	}
	for rows.Next() {
		var p domain.PlanRow
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.Name, &p.Slug, &p.PriceMinor, &p.Currency, &p.BillingCycle, &p.QuotaAmount, &p.CreditPack, &p.Status, &p.Subscribers); err != nil {
			rows.Close()
			return nil, err
		}
		p.Currency = strings.TrimSpace(p.Currency)
		out.Plans = append(out.Plans, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out.SubscriptionsByStatus, err = r.countBy(ctx, `SELECT status, count(*) FROM subscriptions GROUP BY 1`); err != nil {
		return nil, fmt.Errorf("subscriptions by status: %w", err)
	}
	subs, err := r.db.QueryContext(ctx, `
		SELECT `+subscriptionColumns+`
		FROM subscriptions s JOIN plans p ON p.id = s.plan_id LEFT JOIN organizations org ON org.id = s.tenant_id
		ORDER BY s.updated_at DESC LIMIT 50`)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	for subs.Next() {
		s, err := scanSubscription(subs)
		if err != nil {
			subs.Close()
			return nil, err
		}
		out.Subscriptions = append(out.Subscriptions, *s)
	}
	subs.Close()
	if err := subs.Err(); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT count(*), COALESCE(sum(granted), 0), COALESCE(sum(consumed), 0), COALESCE(sum(reserved), 0) FROM credit_accounts`).
		Scan(&out.Accounts, &out.Granted, &out.Consumed, &out.Reserved); err != nil {
		return nil, fmt.Errorf("credit totals: %w", err)
	}
	ledger, err := r.db.QueryContext(ctx, `SELECT transaction_type, COALESCE(sum(amount), 0) FROM usage_ledger WHERE created_at >= $1 GROUP BY 1`, time.Unix(since, 0))
	if err != nil {
		return nil, fmt.Errorf("ledger totals: %w", err)
	}
	defer ledger.Close()
	for ledger.Next() {
		var k string
		var v int64
		if err := ledger.Scan(&k, &v); err != nil {
			return nil, err
		}
		out.LedgerByType[k] = v
	}
	if err := ledger.Err(); err != nil {
		return nil, err
	}
	if out.Policies, err = r.policies(ctx); err != nil {
		return nil, err
	}
	return out, nil
}
