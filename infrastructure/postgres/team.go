package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// TeamRepository stores team invitations and changes member roles (domain.TeamRepository).
type TeamRepository struct {
	db *sql.DB
}

// NewTeamRepository creates a TeamRepository.
func NewTeamRepository(db *sql.DB) *TeamRepository {
	return &TeamRepository{db: db}
}

var _ domain.TeamRepository = (*TeamRepository)(nil)

// Invite records an open invitation or refreshes the open one for the same phone.
func (r *TeamRepository) Invite(ctx context.Context, organizationID, phone string, role domain.OrganizationRole, invitedBy string, now, expiresAt int64) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO organization_invitations (organization_id, phone, membership_role, invited_by, created_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (organization_id, phone) WHERE accepted_at IS NULL AND revoked_at IS NULL
		DO UPDATE SET membership_role = EXCLUDED.membership_role, invited_by = EXCLUDED.invited_by,
		              created_at = EXCLUDED.created_at, expires_at = EXCLUDED.expires_at
		RETURNING id`, organizationID, phone, string(role), invitedBy, time.Unix(now, 0), time.Unix(expiresAt, 0)).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("invite: %w", err)
	}
	return id, nil
}

// Invitations lists the open, unexpired invitations of a business, newest first. Phones are masked.
func (r *TeamRepository) Invitations(ctx context.Context, organizationID string, now int64) ([]domain.TeamInvitation, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT i.id, i.phone, i.membership_role, COALESCE(i.invited_by::text, ''), COALESCE(u.display_name, ''), i.created_at, i.expires_at
		FROM organization_invitations i LEFT JOIN users u ON u.id = i.invited_by
		WHERE i.organization_id = $1 AND i.accepted_at IS NULL AND i.revoked_at IS NULL AND i.expires_at > $2
		ORDER BY i.created_at DESC`, organizationID, time.Unix(now, 0))
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer rows.Close()
	out := []domain.TeamInvitation{}
	for rows.Next() {
		var x domain.TeamInvitation
		var phone string
		var created, expires time.Time
		if err := rows.Scan(&x.ID, &phone, &x.Role, &x.InvitedBy, &x.InvitedName, &created, &expires); err != nil {
			return nil, err
		}
		x.MaskedPhone, x.CreatedAt, x.ExpiresAt = MaskPhone(phone), created.Unix(), expires.Unix()
		out = append(out, x)
	}
	return out, rows.Err()
}

// Revoke closes an open invitation of the business.
func (r *TeamRepository) Revoke(ctx context.Context, organizationID, invitationID string, now int64) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE organization_invitations SET revoked_at = $3
		WHERE id::text = $2 AND organization_id = $1 AND accepted_at IS NULL AND revoked_at IS NULL`, organizationID, invitationID, time.Unix(now, 0))
	if err != nil {
		return fmt.Errorf("revoke invitation: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrInvitationNotFound
	}
	return nil
}

// AcceptFor turns the open invitations for a phone into memberships, in one transaction.
func (r *TeamRepository) AcceptFor(ctx context.Context, userID, phone string, now int64) ([]string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	at := time.Unix(now, 0)
	rows, err := tx.QueryContext(ctx, `
		UPDATE organization_invitations SET accepted_at = $3, accepted_by = $1
		WHERE phone = $2 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > $3
		RETURNING organization_id, membership_role`, userID, phone, at)
	if err != nil {
		return nil, fmt.Errorf("accept invitations: %w", err)
	}
	type grant struct{ org, role string }
	var grants []grant
	for rows.Next() {
		var g grant
		if err := rows.Scan(&g.org, &g.role); err != nil {
			rows.Close()
			return nil, err
		}
		grants = append(grants, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	joined := []string{}
	for _, g := range grants {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO organization_members (organization_id, user_id, membership_role, status, joined_at)
			VALUES ($1, $2, $3, 'active', $4)
			ON CONFLICT (organization_id, user_id) DO NOTHING`, g.org, userID, g.role, at)
		if err != nil {
			return nil, fmt.Errorf("add member: %w", err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			joined = append(joined, g.org)
		}
	}
	return joined, tx.Commit()
}

// SetMemberRole changes a member's role only from the role the caller saw.
func (r *TeamRepository) SetMemberRole(ctx context.Context, organizationID, userID string, from, to domain.OrganizationRole) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE organization_members SET membership_role = $4, updated_at = NOW()
		WHERE organization_id = $1 AND user_id::text = $2 AND membership_role = $3`, organizationID, userID, string(from), string(to))
	if err != nil {
		return fmt.Errorf("set member role: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrAccountState
	}
	return nil
}
