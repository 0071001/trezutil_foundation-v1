package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/trezutil/trezutil/internal/application/negotiation"
)

// NegotiationStore implements negotiation.Store and negotiation.Outbox (migration 000037).
type NegotiationStore struct {
	db *sql.DB
}

// NewNegotiationStore creates the store.
func NewNegotiationStore(db *sql.DB) *NegotiationStore { return &NegotiationStore{db: db} }

var _ negotiation.Store = (*NegotiationStore)(nil)
var _ negotiation.Outbox = (*NegotiationStore)(nil)

type execer interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

func nullUUID(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func (s *NegotiationStore) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Create writes a new negotiation with its mandate, participants and events.
func (s *NegotiationStore) Create(ctx context.Context, snap *negotiation.Snapshot) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		n := &snap.Negotiation
		if err := tx.QueryRowContext(ctx, `INSERT INTO negotiations (buyer_id, conversation_id, title, quantity, currency, status, deadline, version, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, 1, $8, $8) RETURNING id`,
			n.BuyerID, n.ConversationID, n.Title, n.Quantity, n.Currency, string(n.Status), n.Deadline, n.CreatedAt).Scan(&n.ID); err != nil {
			return fmt.Errorf("create negotiation: %w", err)
		}
		m := snap.Mandate
		if _, err := tx.ExecContext(ctx, `INSERT INTO negotiation_mandates (negotiation_id, version, target_unit_minor, max_unit_minor, max_rounds, auto_counter, granted_by, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, n.ID, m.Version, m.TargetUnitMinor, m.MaxUnitMinor, m.MaxRounds, m.AutoCounter, m.GrantedBy, m.CreatedAt); err != nil {
			return fmt.Errorf("create mandate: %w", err)
		}
		for i := range snap.Participants {
			p := &snap.Participants[i]
			p.NegotiationID = n.ID
			if err := tx.QueryRowContext(ctx, `INSERT INTO negotiation_participants (negotiation_id, organization_id, listing_id, listing_title, list_unit_minor, status, round, last_activity_at, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8) RETURNING id`,
				n.ID, p.OrganizationID, p.ListingID, p.ListingTitle, p.ListUnitMinor, string(p.Status), p.Round, p.LastActivityAt).Scan(&p.ID); err != nil {
				return fmt.Errorf("create participant: %w", err)
			}
		}
		for _, e := range snap.Events {
			e.NegotiationID = n.ID
			if err := insertEvent(ctx, tx, e); err != nil {
				return err
			}
		}
		return nil
	})
}

func insertEvent(ctx context.Context, tx execer, e negotiation.Event) error {
	data := e.Data
	if data == nil {
		data = map[string]interface{}{}
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO negotiation_events (negotiation_id, participant_id, actor, actor_id, kind, data, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, e.NegotiationID, nullUUID(e.ParticipantID), e.Actor, nullUUID(e.ActorID), e.Kind, raw, e.CreatedAt)
	if err != nil {
		return fmt.Errorf("record negotiation event: %w", err)
	}
	return nil
}

// Apply writes a change if the negotiation is still at the expected version.
func (s *NegotiationStore) Apply(ctx context.Context, c negotiation.Change) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE negotiations SET version = version + 1, updated_at = now(),
				status = COALESCE(NULLIF($3, ''), status),
				accepted_offer_id = CASE WHEN $6::boolean THEN NULL ELSE COALESCE($4::uuid, accepted_offer_id) END,
				order_id = COALESCE($5::uuid, order_id)
			WHERE id = $1 AND version = $2`, c.NegotiationID, c.ExpectedVersion, string(c.Status), nullUUID(c.AcceptedOfferID), nullUUID(c.OrderID), c.ClearAccepted)
		if err != nil {
			return fmt.Errorf("update negotiation: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return negotiation.ErrConflict
		}
		for _, u := range c.Participants {
			if _, err := tx.ExecContext(ctx, `UPDATE negotiation_participants SET status = $3, round = $4,
					reminded_at = COALESCE($5, reminded_at), last_activity_at = now()
				WHERE id = $1 AND negotiation_id = $2`, u.ID, c.NegotiationID, string(u.Status), u.Round, u.RemindedAt); err != nil {
				return fmt.Errorf("update participant: %w", err)
			}
		}
		for id, st := range c.OfferStatuses {
			if _, err := tx.ExecContext(ctx, `UPDATE negotiation_offers SET status = $3 WHERE id = $1 AND negotiation_id = $2`, id, c.NegotiationID, st); err != nil {
				return fmt.Errorf("update offer: %w", err)
			}
		}
		for _, o := range c.NewOffers {
			if _, err := tx.ExecContext(ctx, `INSERT INTO negotiation_offers (negotiation_id, participant_id, round, author, source, unit_minor, quantity, delivery_days, valid_until, note, status, created_by, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
				c.NegotiationID, o.ParticipantID, o.Round, o.Author, o.Source, o.UnitMinor, o.Quantity, o.DeliveryDays, o.ValidUntil, o.Note, o.Status, nullUUID(o.CreatedBy), o.CreatedAt); err != nil {
				return fmt.Errorf("record offer: %w", err)
			}
		}
		for _, e := range c.Events {
			e.NegotiationID = c.NegotiationID
			if err := insertEvent(ctx, tx, e); err != nil {
				return err
			}
		}
		for _, m := range c.Messages {
			// A portal message is there as soon as it is written; others are sent later by the outbox.
			var sentAt interface{}
			if m.State == "delivered" {
				sentAt = m.CreatedAt
			}
			// The same notification twice (a retried change) is kept once.
			if _, err := tx.ExecContext(ctx, `INSERT INTO negotiation_messages (negotiation_id, participant_id, channel, direction, body, recipient, idempotency_key, state, last_error, created_at, sent_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
				ON CONFLICT (idempotency_key) DO NOTHING`,
				c.NegotiationID, m.ParticipantID, m.Channel, m.Direction, m.Body, m.Recipient, m.IdempotencyKey, m.State, m.LastError, m.CreatedAt, sentAt); err != nil {
				return fmt.Errorf("record message: %w", err)
			}
		}
		return nil
	})
}

const negotiationColumns = `n.id, n.buyer_id, n.conversation_id, n.title, n.quantity, n.currency, n.status, n.deadline,
	COALESCE(n.accepted_offer_id::text, ''), COALESCE(n.order_id::text, ''), n.version, n.created_at, n.updated_at`

func scanNegotiation(row interface{ Scan(...interface{}) error }) (negotiation.Negotiation, error) {
	var n negotiation.Negotiation
	var status string
	err := row.Scan(&n.ID, &n.BuyerID, &n.ConversationID, &n.Title, &n.Quantity, &n.Currency, &status, &n.Deadline,
		&n.AcceptedOfferID, &n.OrderID, &n.Version, &n.CreatedAt, &n.UpdatedAt)
	n.Status = negotiation.Status(status)
	return n, err
}

// Load returns a negotiation with its latest mandate, participants, offers, journal and messages.
func (s *NegotiationStore) Load(ctx context.Context, id string) (*negotiation.Snapshot, error) {
	n, err := scanNegotiation(s.db.QueryRowContext(ctx, `SELECT `+negotiationColumns+` FROM negotiations n WHERE n.id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, negotiation.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load negotiation: %w", err)
	}
	snap := &negotiation.Snapshot{Negotiation: n}
	if err := s.db.QueryRowContext(ctx, `SELECT version, target_unit_minor, max_unit_minor, max_rounds, auto_counter, granted_by, created_at
		FROM negotiation_mandates WHERE negotiation_id = $1 ORDER BY version DESC LIMIT 1`, id).
		Scan(&snap.Mandate.Version, &snap.Mandate.TargetUnitMinor, &snap.Mandate.MaxUnitMinor, &snap.Mandate.MaxRounds, &snap.Mandate.AutoCounter, &snap.Mandate.GrantedBy, &snap.Mandate.CreatedAt); err != nil {
		return nil, fmt.Errorf("load mandate: %w", err)
	}
	if snap.Participants, err = s.participants(ctx, `p.negotiation_id = $1`, id); err != nil {
		return nil, err
	}
	if snap.Offers, err = s.offers(ctx, `negotiation_id = $1`, id); err != nil {
		return nil, err
	}
	if snap.Events, err = s.events(ctx, id); err != nil {
		return nil, err
	}
	if snap.Messages, err = s.messages(ctx, `negotiation_id = $1 ORDER BY created_at, id`, id); err != nil {
		return nil, err
	}
	return snap, nil
}

func (s *NegotiationStore) participants(ctx context.Context, where string, args ...interface{}) ([]negotiation.Participant, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT p.id, p.negotiation_id, p.organization_id, COALESCE(o.display_name, ''), p.listing_id, p.listing_title,
			p.list_unit_minor, p.status, p.round, p.reminded_at, p.last_activity_at
		FROM negotiation_participants p LEFT JOIN organizations o ON o.id = p.organization_id
		WHERE `+where+` ORDER BY p.created_at, p.id`, args...)
	if err != nil {
		return nil, fmt.Errorf("list participants: %w", err)
	}
	defer rows.Close()
	out := []negotiation.Participant{}
	for rows.Next() {
		var p negotiation.Participant
		var status string
		var reminded sql.NullTime
		if err := rows.Scan(&p.ID, &p.NegotiationID, &p.OrganizationID, &p.SupplierName, &p.ListingID, &p.ListingTitle, &p.ListUnitMinor, &status, &p.Round, &reminded, &p.LastActivityAt); err != nil {
			return nil, err
		}
		p.Status = negotiation.ParticipantStatus(status)
		if reminded.Valid {
			t := reminded.Time
			p.RemindedAt = &t
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *NegotiationStore) offers(ctx context.Context, where string, args ...interface{}) ([]negotiation.Offer, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, negotiation_id, participant_id, round, author, source, unit_minor, quantity, delivery_days, valid_until, note, status,
			COALESCE(created_by::text, ''), created_at
		FROM negotiation_offers WHERE `+where+` ORDER BY created_at, id`, args...)
	if err != nil {
		return nil, fmt.Errorf("list offers: %w", err)
	}
	defer rows.Close()
	out := []negotiation.Offer{}
	for rows.Next() {
		var o negotiation.Offer
		var days sql.NullInt64
		var valid sql.NullTime
		if err := rows.Scan(&o.ID, &o.NegotiationID, &o.ParticipantID, &o.Round, &o.Author, &o.Source, &o.UnitMinor, &o.Quantity, &days, &valid, &o.Note, &o.Status, &o.CreatedBy, &o.CreatedAt); err != nil {
			return nil, err
		}
		if days.Valid {
			d := int(days.Int64)
			o.DeliveryDays = &d
		}
		if valid.Valid {
			t := valid.Time
			o.ValidUntil = &t
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (s *NegotiationStore) events(ctx context.Context, id string) ([]negotiation.Event, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, negotiation_id, COALESCE(participant_id::text, ''), actor, COALESCE(actor_id::text, ''), kind, data, created_at
		FROM negotiation_events WHERE negotiation_id = $1 ORDER BY id`, id)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()
	out := []negotiation.Event{}
	for rows.Next() {
		var e negotiation.Event
		var raw []byte
		if err := rows.Scan(&e.ID, &e.NegotiationID, &e.ParticipantID, &e.Actor, &e.ActorID, &e.Kind, &raw, &e.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(raw, &e.Data)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *NegotiationStore) messages(ctx context.Context, where string, args ...interface{}) ([]negotiation.Message, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, negotiation_id, participant_id, channel, direction, body, recipient, external_id, idempotency_key, state, attempts, last_error, created_at
		FROM negotiation_messages WHERE `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()
	out := []negotiation.Message{}
	for rows.Next() {
		var m negotiation.Message
		if err := rows.Scan(&m.ID, &m.NegotiationID, &m.ParticipantID, &m.Channel, &m.Direction, &m.Body, &m.Recipient, &m.ExternalID, &m.IdempotencyKey, &m.State, &m.Attempts, &m.LastError, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListForBuyer returns a buyer's negotiations, most recent first (each loaded whole: a buyer has few).
func (s *NegotiationStore) ListForBuyer(ctx context.Context, buyerID string, limit int) ([]negotiation.Snapshot, error) {
	return s.loadMany(ctx, `SELECT id FROM negotiations WHERE buyer_id = $1 ORDER BY created_at DESC LIMIT $2`, buyerID, limit)
}

// ListForOrganizations returns the negotiations where these businesses take part, most recent activity first.
func (s *NegotiationStore) ListForOrganizations(ctx context.Context, orgIDs []string, limit int) ([]negotiation.Snapshot, error) {
	return s.loadMany(ctx, `SELECT negotiation_id FROM negotiation_participants WHERE organization_id = ANY($1::uuid[])
		GROUP BY negotiation_id ORDER BY max(last_activity_at) DESC LIMIT $2`, pq.Array(orgIDs), limit)
}

func (s *NegotiationStore) loadMany(ctx context.Context, query string, args ...interface{}) ([]negotiation.Snapshot, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list negotiations: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	out := make([]negotiation.Snapshot, 0, len(ids))
	for _, id := range ids {
		snap, err := s.Load(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *snap)
	}
	return out, nil
}

// ParticipantNegotiation returns the negotiation and business of a participant.
func (s *NegotiationStore) ParticipantNegotiation(ctx context.Context, participantID string) (string, string, error) {
	var nid, org string
	err := s.db.QueryRowContext(ctx, `SELECT negotiation_id, organization_id FROM negotiation_participants WHERE id = $1`, participantID).Scan(&nid, &org)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", negotiation.ErrNotFound
	}
	if err != nil {
		return "", "", fmt.Errorf("find participant: %w", err)
	}
	return nid, org, nil
}

// DueDeadlines lists the open negotiations whose deadline passed (or whose decision week ended).
func (s *NegotiationStore) DueDeadlines(ctx context.Context, now time.Time, limit int) ([]string, error) {
	return s.ids(ctx, `SELECT id FROM negotiations
		WHERE (status IN ('soliciting', 'negotiating') AND deadline <= $1)
		   OR (status = 'awaiting_decision' AND deadline <= $1 - interval '7 days')
		ORDER BY deadline LIMIT $2`, now, limit)
}

// DueReminders lists the open negotiations with a supplier still to answer, halfway to the deadline, not reminded yet.
func (s *NegotiationStore) DueReminders(ctx context.Context, now time.Time, limit int) ([]string, error) {
	return s.ids(ctx, `SELECT DISTINCT n.id FROM negotiations n JOIN negotiation_participants p ON p.negotiation_id = n.id
		WHERE n.status IN ('soliciting', 'negotiating', 'awaiting_decision') AND n.deadline > $1
		  AND p.status IN ('invited', 'countered') AND p.reminded_at IS NULL
		  AND n.created_at + (n.deadline - n.created_at) / 2 <= $1
		LIMIT $2`, now, limit)
}

func (s *NegotiationStore) ids(ctx context.Context, query string, args ...interface{}) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("due negotiations: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// PendingMessages returns the outbox's waiting messages, oldest first.
func (s *NegotiationStore) PendingMessages(ctx context.Context, limit int) ([]negotiation.Message, error) {
	return s.messages(ctx, `state = 'pending' ORDER BY created_at, id LIMIT $1`, limit)
}

// MarkMessage records a delivery attempt.
func (s *NegotiationStore) MarkMessage(ctx context.Context, id, state, lastError, externalID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE negotiation_messages SET state = $2, last_error = $3, attempts = attempts + 1,
			external_id = COALESCE(NULLIF($4, ''), external_id),
			sent_at = CASE WHEN $5::boolean THEN now() ELSE sent_at END
		WHERE id = $1 AND state = 'pending'`, id, state, lastError, externalID, state == "sent" || state == "delivered")
	if err != nil {
		return fmt.Errorf("mark message: %w", err)
	}
	return nil
}
