package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"

	"github.com/trezutil/trezutil/internal/domain"
)

// AnalyticsRepository computes the figures behind the dashboards. Every number
// is a count or a sum over the platform's own tables; days with no activity are
// real zeros. See internal/domain/analytics.go for the definitions.
type AnalyticsRepository struct {
	db *sql.DB
}

// NewAnalyticsRepository creates an AnalyticsRepository.
func NewAnalyticsRepository(db *sql.DB) *AnalyticsRepository { return &AnalyticsRepository{db: db} }

var analyticsZone = time.FixedZone("WAT", domain.AnalyticsZoneOffsetSeconds)

// dayExpr turns a timestamptz column into its Kinshasa calendar day.
func dayExpr(column string) string {
	return `to_char(` + column + ` AT TIME ZONE '` + domain.AnalyticsTimeZone + `', 'YYYY-MM-DD')`
}

// qb collects the arguments of one query so placeholders are numbered as they are used.
type qb struct{ args []interface{} }

func (q *qb) arg(v interface{}) string {
	q.args = append(q.args, v)
	return fmt.Sprintf("$%d", len(q.args))
}

// scope restricts a query to one organization; the zero value is the whole platform.
type scope struct{ organizationID string }

func (s scope) isPlatform() bool { return s.organizationID == "" }

// and returns " AND column = $n" for an organization scope and "" for the platform.
func (s scope) and(column string, q *qb) string {
	if s.isPlatform() {
		return ""
	}
	return " AND " + column + " = " + q.arg(s.organizationID)
}

// where returns " WHERE column = $n" for an organization scope and "" for the platform.
func (s scope) where(column string, q *qb) string {
	if s.isPlatform() {
		return ""
	}
	return " WHERE " + column + " = " + q.arg(s.organizationID)
}

// window is the current period and the one before it, aligned to Kinshasa midnights.
type window struct {
	days     int
	prevFrom time.Time
	from     time.Time
	to       time.Time // exclusive: the start of tomorrow
	current  []string
	previous []string
}

func newWindow(now int64, days int) window {
	if days < domain.MinAnalyticsDays {
		days = domain.MinAnalyticsDays
	}
	if days > domain.MaxAnalyticsDays {
		days = domain.MaxAnalyticsDays
	}
	local := time.Unix(now, 0).In(analyticsZone)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, analyticsZone)
	w := window{days: days, to: today.AddDate(0, 0, 1)}
	w.from = w.to.AddDate(0, 0, -days)
	w.prevFrom = w.from.AddDate(0, 0, -days)
	for d := w.prevFrom; d.Before(w.from); d = d.AddDate(0, 0, 1) {
		w.previous = append(w.previous, d.Format("2006-01-02"))
	}
	for d := w.from; d.Before(w.to); d = d.AddDate(0, 0, 1) {
		w.current = append(w.current, d.Format("2006-01-02"))
	}
	return w
}

// Platform covers every organization.
func (r *AnalyticsRepository) Platform(ctx context.Context, now int64, days int) (*domain.Analytics, error) {
	return r.compute(ctx, scope{}, now, days)
}

// Organization covers one organization.
func (r *AnalyticsRepository) Organization(ctx context.Context, organizationID string, now int64, days int) (*domain.Analytics, error) {
	if strings.TrimSpace(organizationID) == "" {
		return nil, fmt.Errorf("organization id is required")
	}
	return r.compute(ctx, scope{organizationID: organizationID}, now, days)
}

func (r *AnalyticsRepository) compute(ctx context.Context, sc scope, now int64, days int) (*domain.Analytics, error) {
	w := newWindow(now, days)
	out := &domain.Analytics{GeneratedAt: now, Days: w.days, From: w.current[0], To: w.current[len(w.current)-1]}

	// One query per measure over both periods; the halves are split in Go.
	series := func(table, extra string, scoped bool) (map[string]int, error) {
		q := &qb{}
		from, to := q.arg(w.prevFrom), q.arg(w.to)
		query := `SELECT ` + dayExpr("created_at") + `, count(*) FROM ` + table + ` WHERE created_at >= ` + from + ` AND created_at < ` + to + extra
		if scoped {
			query += sc.and("organization_id", q)
		}
		return r.pairs(ctx, query+` GROUP BY 1`, q.args)
	}
	orders, err := series("orders", ` AND status <> 'draft'`, true)
	if err != nil {
		return nil, fmt.Errorf("orders by day: %w", err)
	}
	reservations, err := series("reservations", "", true)
	if err != nil {
		return nil, fmt.Errorf("reservations by day: %w", err)
	}
	listings, err := series("listings", "", true)
	if err != nil {
		return nil, fmt.Errorf("listings by day: %w", err)
	}
	revenue, err := r.dailyRevenue(ctx, sc, w)
	if err != nil {
		return nil, fmt.Errorf("revenue by day: %w", err)
	}

	out.Orders, out.Current.Orders, out.Previous.Orders = split(w, orders)
	out.Reservations, out.Current.Reservations, out.Previous.Reservations = split(w, reservations)
	out.Listings, out.Current.NewListings, out.Previous.NewListings = split(w, listings)
	out.Revenue, out.Current.Revenue, out.Previous.Revenue = splitRevenue(w, revenue)

	if sc.isPlatform() {
		users, err := series("users", "", false)
		if err != nil {
			return nil, fmt.Errorf("users by day: %w", err)
		}
		conversations, err := series("conversations", "", false)
		if err != nil {
			return nil, fmt.Errorf("conversations by day: %w", err)
		}
		out.Users, out.Current.NewUsers, out.Previous.NewUsers = split(w, users)
		out.Conversations, out.Current.Conversations, out.Previous.Conversations = split(w, conversations)
	}

	if out.Current.Customers, out.Current.Returning, err = r.customers(ctx, sc, w.from, w.to); err != nil {
		return nil, fmt.Errorf("customers: %w", err)
	}
	if out.Previous.Customers, out.Previous.Returning, err = r.customers(ctx, sc, w.prevFrom, w.from); err != nil {
		return nil, fmt.Errorf("previous customers: %w", err)
	}

	// Breakdowns of the current period.
	byStatus := func(table, extra string) (map[string]int, error) {
		q := &qb{}
		from, to := q.arg(w.from), q.arg(w.to)
		return r.pairs(ctx, `SELECT status, count(*) FROM `+table+` WHERE created_at >= `+from+` AND created_at < `+to+extra+sc.and("organization_id", q)+` GROUP BY status`, q.args)
	}
	if out.OrdersByStatus, err = byStatus("orders", ` AND status <> 'draft'`); err != nil {
		return nil, fmt.Errorf("orders by status: %w", err)
	}
	if out.ReservationsByStatus, err = byStatus("reservations", ""); err != nil {
		return nil, fmt.Errorf("reservations by status: %w", err)
	}

	// Snapshots of the catalogue as it is now, not within the window.
	q := &qb{}
	query := `SELECT status, count(*) FROM listings` + sc.where("organization_id", q) + ` GROUP BY status`
	if out.ListingsByStatus, err = r.pairs(ctx, query, q.args); err != nil {
		return nil, fmt.Errorf("listings by status: %w", err)
	}
	q = &qb{}
	query = `SELECT COALESCE(vertical, 'OTHER'), count(*) FROM listings WHERE status = 'published'` + sc.and("organization_id", q) + ` GROUP BY 1`
	if out.ListingsByVertical, err = r.pairs(ctx, query, q.args); err != nil {
		return nil, fmt.Errorf("published listings by vertical: %w", err)
	}

	if sc.isPlatform() {
		if out.UsersByRole, err = r.pairs(ctx, `SELECT r.name, count(*) FROM user_roles ur JOIN roles r ON r.id = ur.role_id GROUP BY r.name`, nil); err != nil {
			return nil, fmt.Errorf("users by role: %w", err)
		}
		if out.TopOrganizations, err = r.topOrganizations(ctx, w); err != nil {
			return nil, fmt.Errorf("top organizations: %w", err)
		}
	} else if out.TopListings, err = r.topListings(ctx, sc, w); err != nil {
		return nil, fmt.Errorf("top listings: %w", err)
	}
	return out, nil
}

// pairs runs a query that returns (key, count) rows.
func (r *AnalyticsRepository) pairs(ctx context.Context, query string, args []interface{}) (map[string]int, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var key string
		var n int
		if err := rows.Scan(&key, &n); err != nil {
			return nil, err
		}
		out[key] = n
	}
	return out, rows.Err()
}

// dailyRevenue returns day -> currency -> minor units of earning orders.
func (r *AnalyticsRepository) dailyRevenue(ctx context.Context, sc scope, w window) (map[string]map[string]int64, error) {
	q := &qb{}
	from, to, statuses := q.arg(w.prevFrom), q.arg(w.to), q.arg(pq.Array(domain.EarningOrderStatuses))
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+dayExpr("created_at")+`, currency, COALESCE(sum(total_amount), 0) FROM orders WHERE created_at >= `+from+` AND created_at < `+to+` AND status = ANY(`+statuses+`)`+sc.and("organization_id", q)+` GROUP BY 1, 2`,
		q.args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]map[string]int64{}
	for rows.Next() {
		var day, currency string
		var amount int64
		if err := rows.Scan(&day, &currency, &amount); err != nil {
			return nil, err
		}
		if out[day] == nil {
			out[day] = map[string]int64{}
		}
		out[day][currency] += amount
	}
	return out, rows.Err()
}

// customers counts the distinct buyers in [from, to) and how many of them had
// already bought in the same scope before from.
func (r *AnalyticsRepository) customers(ctx context.Context, sc scope, from, to time.Time) (total, returning int, err error) {
	q := &qb{}
	f, t := q.arg(from), q.arg(to)
	query := `
		SELECT count(DISTINCT o.customer_id),
		       count(DISTINCT o.customer_id) FILTER (WHERE EXISTS (
		           SELECT 1 FROM orders p WHERE p.customer_id = o.customer_id AND p.created_at < ` + f + ` AND p.status <> 'draft'` + sc.and("p.organization_id", q) + `))
		FROM orders o WHERE o.created_at >= ` + f + ` AND o.created_at < ` + t + ` AND o.status <> 'draft'` + sc.and("o.organization_id", q)
	err = r.db.QueryRowContext(ctx, query, q.args...).Scan(&total, &returning)
	return total, returning, err
}

func (r *AnalyticsRepository) topOrganizations(ctx context.Context, w window) ([]domain.RankedEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT o.organization_id::text, org.display_name, o.currency, count(*),
		       COALESCE(sum(o.total_amount) FILTER (WHERE o.status = ANY($3)), 0)
		FROM orders o JOIN organizations org ON org.id = o.organization_id
		WHERE o.created_at >= $1 AND o.created_at < $2 AND o.status <> 'draft'
		GROUP BY 1, 2, 3`, w.from, w.to, pq.Array(domain.EarningOrderStatuses))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byOrg := map[string]*domain.RankedEntry{}
	for rows.Next() {
		var id, name, currency string
		var n int
		var amount int64
		if err := rows.Scan(&id, &name, &currency, &n, &amount); err != nil {
			return nil, err
		}
		e := byOrg[id]
		if e == nil {
			e = &domain.RankedEntry{ID: id, Name: name, Revenue: map[string]int64{}}
			byOrg[id] = e
		}
		e.Orders += n
		if amount > 0 {
			e.Revenue[currency] += amount
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rank(byOrg), nil
}

// topListings ranks an organization's listings by what its earning orders sold.
func (r *AnalyticsRepository) topListings(ctx context.Context, sc scope, w window) ([]domain.RankedEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT it->>'ListingID', max(it->>'Title'), COALESCE(it->'TotalPrice'->>'Currency', o.currency),
		       count(DISTINCT o.id), COALESCE(sum((it->>'Quantity')::int), 0), COALESCE(sum((it->'TotalPrice'->>'AmountMinor')::bigint), 0)
		FROM orders o, jsonb_array_elements(o.items) it
		WHERE o.created_at >= $1 AND o.created_at < $2 AND o.organization_id = $3 AND o.status = ANY($4)
		GROUP BY 1, 3`, w.from, w.to, sc.organizationID, pq.Array(domain.EarningOrderStatuses))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byListing := map[string]*domain.RankedEntry{}
	for rows.Next() {
		var id, title, currency string
		var orders, units int
		var amount int64
		if err := rows.Scan(&id, &title, &currency, &orders, &units, &amount); err != nil {
			return nil, err
		}
		e := byListing[id]
		if e == nil {
			e = &domain.RankedEntry{ID: id, Name: title, Revenue: map[string]int64{}}
			byListing[id] = e
		}
		e.Orders += orders
		e.Units += units
		e.Revenue[currency] += amount
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return rank(byListing), nil
}

// rank orders entries by orders, then units, then name, and keeps the first five.
func rank(entries map[string]*domain.RankedEntry) []domain.RankedEntry {
	out := make([]domain.RankedEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Orders != out[j].Orders {
			return out[i].Orders > out[j].Orders
		}
		if out[i].Units != out[j].Units {
			return out[i].Units > out[j].Units
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

// split lays a day->count map over the window and totals each half.
func split(w window, byDay map[string]int) (series []domain.DayCount, current, previous int) {
	series = make([]domain.DayCount, 0, len(w.current))
	for _, d := range w.current {
		series = append(series, domain.DayCount{Date: d, Count: byDay[d]})
		current += byDay[d]
	}
	for _, d := range w.previous {
		previous += byDay[d]
	}
	return series, current, previous
}

func splitRevenue(w window, byDay map[string]map[string]int64) (series []domain.DayRevenue, current, previous map[string]int64) {
	current, previous = map[string]int64{}, map[string]int64{}
	series = make([]domain.DayRevenue, 0, len(w.current))
	for _, d := range w.current {
		amounts := map[string]int64{}
		for c, v := range byDay[d] {
			amounts[c] = v
			current[c] += v
		}
		series = append(series, domain.DayRevenue{Date: d, Amounts: amounts})
	}
	for _, d := range w.previous {
		for c, v := range byDay[d] {
			previous[c] += v
		}
	}
	return series, current, previous
}
