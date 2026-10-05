package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// AdminRepository serves the read-only queries of the administration console.
type AdminRepository struct {
	db *sql.DB
}

// NewAdminRepository creates an AdminRepository.
func NewAdminRepository(db *sql.DB) *AdminRepository { return &AdminRepository{db: db} }

// Overview counts the platform's accounts, listings, orders and conversations.
func (r *AdminRepository) Overview(ctx context.Context, now int64) (*domain.AdminOverview, error) {
	since := toTime(now - 7*24*3600)
	out := &domain.AdminOverview{GeneratedAt: now, UsersByRole: map[string]int{}, ListingsByStatus: map[string]int{}}

	if err := r.db.QueryRowContext(ctx, `SELECT count(*), count(*) FILTER (WHERE status = 'active'), count(*) FILTER (WHERE status = 'suspended'), count(*) FILTER (WHERE created_at >= $1) FROM users`, since).
		Scan(&out.UsersTotal, &out.UsersActive, &out.UsersSuspended, &out.UsersNew7d); err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	if err := r.groupCount(ctx, `SELECT r.name, count(*) FROM user_roles ur JOIN roles r ON r.id = ur.role_id GROUP BY r.name`, out.UsersByRole); err != nil {
		return nil, fmt.Errorf("count roles: %w", err)
	}
	if err := r.groupCount(ctx, `SELECT status, count(*) FROM listings GROUP BY status`, out.ListingsByStatus); err != nil {
		return nil, fmt.Errorf("count listings: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM organizations`).Scan(&out.Organizations); err != nil {
		return nil, fmt.Errorf("count organizations: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, `SELECT count(*), count(*) FILTER (WHERE created_at >= $1) FROM orders`, since).Scan(&out.OrdersTotal, &out.Orders7d); err != nil {
		return nil, fmt.Errorf("count orders: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM reservations`).Scan(&out.ReservationsTotal); err != nil {
		return nil, fmt.Errorf("count reservations: %w", err)
	}
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM conversations WHERE created_at >= $1`, since).Scan(&out.Conversations7d); err != nil {
		return nil, fmt.Errorf("count conversations: %w", err)
	}
	return out, nil
}

func (r *AdminRepository) groupCount(ctx context.Context, query string, into map[string]int) error {
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		var n int
		if err := rows.Scan(&key, &n); err != nil {
			return err
		}
		into[key] = n
	}
	return rows.Err()
}

// likePattern turns a search word into a contains-pattern, escaping the
// characters LIKE treats specially.
func likePattern(q string) string {
	q = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.TrimSpace(q))
	return "%" + q + "%"
}

const adminUserFilter = `($1 = '' OR u.email ILIKE $2 ESCAPE '\' OR u.phone ILIKE $2 ESCAPE '\' OR u.display_name ILIKE $2 ESCAPE '\')`

// ListUsers returns one page of accounts, newest first, and the total that
// matches. query is matched against email, phone and name.
func (r *AdminRepository) ListUsers(ctx context.Context, query string, limit, offset int) ([]domain.AdminUser, int, error) {
	query = strings.TrimSpace(query)
	pattern := likePattern(query)

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM users u WHERE `+adminUserFilter, query, pattern).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT u.id, COALESCE(u.email, ''), COALESCE(u.phone, ''), u.display_name, u.status, u.created_at, u.last_login_at,
		       COALESCE(string_agg(r.name, ',' ORDER BY r.name), '')
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles r ON r.id = ur.role_id
		WHERE `+adminUserFilter+`
		GROUP BY u.id
		ORDER BY u.created_at DESC, u.id
		LIMIT $3 OFFSET $4`, query, pattern, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := []domain.AdminUser{}
	for rows.Next() {
		var u domain.AdminUser
		var created time.Time
		var lastLogin sql.NullTime
		var roles string
		if err := rows.Scan(&u.ID, &u.Email, &u.Phone, &u.DisplayName, &u.Status, &created, &lastLogin, &roles); err != nil {
			return nil, 0, err
		}
		u.CreatedAt = toUnix(created)
		if lastLogin.Valid {
			u.LastLoginAt = toUnix(lastLogin.Time)
		}
		u.Roles = []string{}
		if roles != "" {
			u.Roles = strings.Split(roles, ",")
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}
