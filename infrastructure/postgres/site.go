package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// SiteContentRepository stores the managed content of the public site, the
// uploaded partner logos, and computes the live figures of the trust strip.
type SiteContentRepository struct {
	db *sql.DB
}

// NewSiteContentRepository creates a SiteContentRepository.
func NewSiteContentRepository(db *sql.DB) *SiteContentRepository {
	return &SiteContentRepository{db: db}
}

// siteKey is the one document the home page reads.
const siteKey = "home"

// Get returns the saved content, or the built-in defaults (Stored=false,
// Version=0) when staff have never saved anything.
func (r *SiteContentRepository) Get(ctx context.Context) (*domain.SiteContent, error) {
	var raw []byte
	var version int
	var updatedBy sql.NullString
	var updatedAt time.Time
	err := r.db.QueryRowContext(ctx, `SELECT value, version, updated_by::text, updated_at FROM site_settings WHERE key = $1`, siteKey).
		Scan(&raw, &version, &updatedBy, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		def := domain.DefaultSiteContent()
		return &def, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read site content: %w", err)
	}
	var c domain.SiteContent
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("decode site content: %w", err)
	}
	// An older document never hides a section or a metric a newer release added.
	c.Normalize()
	c.Version, c.UpdatedAt, c.Stored = version, updatedAt.Unix(), true
	if updatedBy.Valid {
		c.UpdatedBy = updatedBy.String
	}
	return &c, nil
}

// Save replaces the content when the stored version is expectedVersion (0 when
// nothing has been saved yet). It returns domain.ErrSiteContentConflict when
// somebody saved in between.
func (r *SiteContentRepository) Save(ctx context.Context, content *domain.SiteContent, expectedVersion int, actorID string) (*domain.SiteContent, error) {
	raw, err := json.Marshal(content)
	if err != nil {
		return nil, fmt.Errorf("encode site content: %w", err)
	}
	var version int
	var updatedAt time.Time
	if expectedVersion == 0 {
		err = r.db.QueryRowContext(ctx,
			`INSERT INTO site_settings (key, value, version, updated_by) VALUES ($1, $2, 1, $3) ON CONFLICT (key) DO NOTHING RETURNING version, updated_at`,
			siteKey, raw, actorRef(actorID)).Scan(&version, &updatedAt)
	} else {
		err = r.db.QueryRowContext(ctx,
			`UPDATE site_settings SET value = $2, version = version + 1, updated_by = $3, updated_at = NOW() WHERE key = $1 AND version = $4 RETURNING version, updated_at`,
			siteKey, raw, actorRef(actorID), expectedVersion).Scan(&version, &updatedAt)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrSiteContentConflict
	}
	if err != nil {
		return nil, fmt.Errorf("save site content: %w", err)
	}
	saved := *content
	saved.Version, saved.UpdatedAt, saved.UpdatedBy, saved.Stored = version, updatedAt.Unix(), actorRef(actorID).String, true
	return &saved, nil
}

// Stats computes the live value of every trust-strip metric from the platform's own tables.
func (r *SiteContentRepository) Stats(ctx context.Context) (domain.SiteStats, error) {
	var verified, live, deals, cities, bookings, orders, verticals, users, merchants, conversations, connections int
	err := r.db.QueryRowContext(ctx, `
		SELECT
		  (SELECT count(*) FROM organizations WHERE status = 'active' AND verification_status = 'verified'),
		  (SELECT count(*) FROM listings WHERE status = 'published'),
		  (SELECT count(*) FROM listings
		     WHERE status = 'published'
		       AND jsonb_typeof(metadata->'promotion') = 'object'
		       AND COALESCE((metadata->'promotion'->>'compare_at_amount')::numeric, 0) > price_amount),
		  (SELECT count(DISTINCT lower(trim(s.city))) FROM stores s
		     WHERE s.status = 'active' AND COALESCE(trim(s.city), '') <> ''
		       AND EXISTS (SELECT 1 FROM listings l WHERE l.store_id = s.id AND l.status = 'published')),
		  (SELECT count(*) FROM reservations WHERE status IN ('confirmed', 'completed')),
		  (SELECT count(*) FROM orders WHERE status IN ('fulfilled', 'completed')),
		  (SELECT count(DISTINCT vertical) FROM listings WHERE status = 'published' AND vertical IS NOT NULL),
		  (SELECT count(DISTINCT u.id) FROM users u
		     JOIN user_roles ur ON ur.user_id = u.id JOIN roles r ON r.id = ur.role_id
		     WHERE u.status = 'active' AND r.name IN ('customer', 'seller')),
		  (SELECT count(*) FROM organizations o
		     WHERE o.status = 'active' AND EXISTS (SELECT 1 FROM listings l WHERE l.organization_id = o.id AND l.status = 'published')),
		  (SELECT count(*) FROM conversations),
		  (SELECT count(*) FROM (
		     SELECT customer_id, organization_id FROM orders WHERE status IN ('fulfilled', 'completed')
		     UNION
		     SELECT customer_id, organization_id FROM reservations WHERE status IN ('confirmed', 'completed')
		   ) AS pairs)
	`).Scan(&verified, &live, &deals, &cities, &bookings, &orders, &verticals, &users, &merchants, &conversations, &connections)
	if err != nil {
		return nil, fmt.Errorf("site stats: %w", err)
	}
	return domain.SiteStats{
		domain.SiteMetricVerifiedSellers:   verified,
		domain.SiteMetricLiveListings:      live,
		domain.SiteMetricDealsLive:         deals,
		domain.SiteMetricCitiesCovered:     cities,
		domain.SiteMetricBookingsConfirmed: bookings,
		domain.SiteMetricOrdersCompleted:   orders,
		domain.SiteMetricVerticalsLive:     verticals,
		domain.SiteMetricRegisteredUsers:   users,
		domain.SiteMetricMerchants:         merchants,
		domain.SiteMetricTransactions:      orders + bookings,
		domain.SiteMetricConversations:     conversations,
		domain.SiteMetricConnections:       connections,
	}, nil
}

// SaveAsset stores an uploaded logo. The same bytes are stored once; uploading
// them again returns the existing id.
func (r *SiteContentRepository) SaveAsset(ctx context.Context, contentType string, data []byte, actorID string) (string, error) {
	sum := sha256.Sum256(data)
	var id string
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO site_assets (content_type, size_bytes, sha256, data, created_by) VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (sha256) DO UPDATE SET sha256 = EXCLUDED.sha256 RETURNING id::text`,
		contentType, len(data), hex.EncodeToString(sum[:]), data, actorRef(actorID)).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("save site asset: %w", err)
	}
	return id, nil
}

// Asset returns an uploaded logo, or domain.ErrSiteAssetNotFound.
func (r *SiteContentRepository) Asset(ctx context.Context, id string) (string, []byte, error) {
	if !uuidPattern.MatchString(id) {
		return "", nil, domain.ErrSiteAssetNotFound
	}
	var contentType string
	var data []byte
	err := r.db.QueryRowContext(ctx, `SELECT content_type, data FROM site_assets WHERE id = $1`, id).Scan(&contentType, &data)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil, domain.ErrSiteAssetNotFound
	}
	if err != nil {
		return "", nil, fmt.Errorf("read site asset: %w", err)
	}
	return contentType, data, nil
}
