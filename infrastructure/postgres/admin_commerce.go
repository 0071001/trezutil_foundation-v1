package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

/*
 * Read models of the administration console for commerce: orders, bookings and businesses across the whole
 * platform, newest first, paged, with a text search. They read the tables as they are (nothing is stored for the
 * console), show names and states and never contact data, and every search word is a bound parameter.
 */

// textSearch builds the "matches" condition shared by the lists: the id starts with the word, or one of the
// names involved contains it. args[0] is the raw word, args[1] the contains-pattern.
func textSearch(idColumn string, names ...string) string {
	conds := []string{fmt.Sprintf("%s::text ILIKE $2 ESCAPE '\\'", idColumn)}
	for _, n := range names {
		conds = append(conds, fmt.Sprintf("%s ILIKE $2 ESCAPE '\\'", n))
	}
	return "($1 = '' OR " + strings.Join(conds, " OR ") + ")"
}

// ListOrders returns one page of orders, newest first, and the total that matches.
func (r *AdminRepository) ListOrders(ctx context.Context, f domain.AdminListFilter, limit, offset int) ([]domain.AdminOrder, int, error) {
	q := strings.TrimSpace(f.Query)
	pattern := likePattern(q)
	const from = `FROM orders o
		JOIN organizations org ON org.id = o.organization_id
		JOIN users c ON c.id = o.customer_id`
	where := `WHERE ` + textSearch("o.id", "org.display_name", "c.display_name") + ` AND ($3 = '' OR o.status = $3)`

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) `+from+` `+where, q, pattern, f.Status).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count orders: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT o.id, o.status, o.total_amount, o.currency,
		       COALESCE(o.items->0->>'Title', ''), COALESCE(jsonb_array_length(o.items), 0),
		       o.organization_id, org.display_name, o.customer_id, c.display_name, o.created_at, o.updated_at
		`+from+` `+where+`
		ORDER BY o.created_at DESC, o.id
		LIMIT $4 OFFSET $5`, q, pattern, f.Status, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()

	out := []domain.AdminOrder{}
	for rows.Next() {
		var o domain.AdminOrder
		var created, updated time.Time
		if err := rows.Scan(&o.ID, &o.Status, &o.TotalMinor, &o.Currency, &o.Title, &o.ItemCount,
			&o.OrganizationID, &o.OrganizationName, &o.CustomerID, &o.CustomerName, &created, &updated); err != nil {
			return nil, 0, err
		}
		o.CreatedAt, o.UpdatedAt = toUnix(created), toUnix(updated)
		out = append(out, o)
	}
	return out, total, rows.Err()
}

// ListReservations returns one page of bookings, newest first, and the total that matches.
func (r *AdminRepository) ListReservations(ctx context.Context, f domain.AdminListFilter, limit, offset int) ([]domain.AdminReservation, int, error) {
	q := strings.TrimSpace(f.Query)
	pattern := likePattern(q)
	const from = `FROM reservations r
		JOIN organizations org ON org.id = r.organization_id
		JOIN users c ON c.id = r.customer_id
		JOIN listings l ON l.id = r.listing_id`
	where := `WHERE ` + textSearch("r.id", "org.display_name", "c.display_name", "l.title") + ` AND ($3 = '' OR r.status = $3)`

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) `+from+` `+where, q, pattern, f.Status).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count reservations: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.id, r.status, l.title, r.listing_id, r.organization_id, org.display_name, r.customer_id, c.display_name,
		       r.start_at, r.end_at, r.quantity, r.party_size, r.created_at
		`+from+` `+where+`
		ORDER BY r.created_at DESC, r.id
		LIMIT $4 OFFSET $5`, q, pattern, f.Status, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list reservations: %w", err)
	}
	defer rows.Close()

	out := []domain.AdminReservation{}
	for rows.Next() {
		var b domain.AdminReservation
		var start, end, created time.Time
		if err := rows.Scan(&b.ID, &b.Status, &b.Title, &b.ListingID, &b.OrganizationID, &b.OrganizationName, &b.CustomerID, &b.CustomerName,
			&start, &end, &b.Quantity, &b.PartySize, &created); err != nil {
			return nil, 0, err
		}
		b.StartAt, b.EndAt, b.CreatedAt = toUnix(start), toUnix(end), toUnix(created)
		out = append(out, b)
	}
	return out, total, rows.Err()
}

// ListOrganizations returns one page of businesses, newest first, and the total that matches. The counts are
// computed with the page, never stored.
func (r *AdminRepository) ListOrganizations(ctx context.Context, f domain.AdminListFilter, limit, offset int) ([]domain.AdminOrganization, int, error) {
	q := strings.TrimSpace(f.Query)
	pattern := likePattern(q)
	where := `WHERE ` + textSearch("o.id", "o.display_name", "o.legal_name", "COALESCE(o.city, '')") + ` AND ($3 = '' OR o.status = $3)`

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM organizations o `+where, q, pattern, f.Status).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count organizations: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT o.id, o.display_name, o.type, o.status, o.verification_status, COALESCE(o.city, ''), o.country, o.created_at,
		       (SELECT count(*) FROM organization_members m WHERE m.organization_id = o.id),
		       (SELECT count(*) FROM stores s WHERE s.organization_id = o.id),
		       (SELECT count(*) FROM listings l WHERE l.organization_id = o.id AND l.status = 'published'),
		       (SELECT count(*) FROM orders x WHERE x.organization_id = o.id)
		FROM organizations o `+where+`
		ORDER BY o.created_at DESC, o.id
		LIMIT $4 OFFSET $5`, q, pattern, f.Status, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list organizations: %w", err)
	}
	defer rows.Close()

	out := []domain.AdminOrganization{}
	for rows.Next() {
		var o domain.AdminOrganization
		var created time.Time
		if err := rows.Scan(&o.ID, &o.DisplayName, &o.Type, &o.Status, &o.VerificationStatus, &o.City, &o.Country, &created,
			&o.Members, &o.Stores, &o.PublishedListings, &o.Orders); err != nil {
			return nil, 0, err
		}
		o.CreatedAt = toUnix(created)
		out = append(out, o)
	}
	return out, total, rows.Err()
}
