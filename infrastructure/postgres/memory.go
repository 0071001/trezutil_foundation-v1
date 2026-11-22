package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// MemoryStore implements domain.MemoryStore (migration 000032). Every statement is scoped by user_id.
type MemoryStore struct {
	db *sql.DB
}

// NewMemoryStore creates a MemoryStore.
func NewMemoryStore(db *sql.DB) *MemoryStore {
	return &MemoryStore{db: db}
}

// List returns the user's items, the most recent first.
func (s *MemoryStore) List(ctx context.Context, userID string) ([]domain.MemoryItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, user_id, kind, text, created_at, updated_at
	                                      FROM commercial_memory_items WHERE user_id = $1
	                                      ORDER BY created_at DESC, id`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list memory: %w", err)
	}
	defer rows.Close()
	out := []domain.MemoryItem{}
	for rows.Next() {
		item, err := scanMemoryItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

// Count returns how many items the user has.
func (s *MemoryStore) Count(ctx context.Context, userID string) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM commercial_memory_items WHERE user_id = $1`, userID).Scan(&n); err != nil {
		return 0, fmt.Errorf("failed to count memory: %w", err)
	}
	return n, nil
}

// Add stores a new item; its id and times are set by the database.
func (s *MemoryStore) Add(ctx context.Context, item *domain.MemoryItem) error {
	row := s.db.QueryRowContext(ctx, `INSERT INTO commercial_memory_items (user_id, kind, text) VALUES ($1, $2, $3)
	                                  RETURNING id, user_id, kind, text, created_at, updated_at`,
		item.UserID, string(item.Kind), item.Text)
	stored, err := scanMemoryItem(row)
	if err != nil {
		return err
	}
	*item = *stored
	return nil
}

// UpdateText changes the text of one of the user's items.
func (s *MemoryStore) UpdateText(ctx context.Context, userID, itemID, text string) (*domain.MemoryItem, error) {
	row := s.db.QueryRowContext(ctx, `UPDATE commercial_memory_items SET text = $3, updated_at = NOW()
	                                  WHERE id = $1 AND user_id = $2
	                                  RETURNING id, user_id, kind, text, created_at, updated_at`, itemID, userID, text)
	item, err := scanMemoryItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrMemoryItemNotFound
	}
	return item, err
}

// Delete removes one of the user's items.
func (s *MemoryStore) Delete(ctx context.Context, userID, itemID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM commercial_memory_items WHERE id = $1 AND user_id = $2`, itemID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete memory item: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("failed to delete memory item: %w", err)
	} else if n == 0 {
		return domain.ErrMemoryItemNotFound
	}
	return nil
}

// DeleteAll removes all the user's items (the setting stays).
func (s *MemoryStore) DeleteAll(ctx context.Context, userID string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM commercial_memory_items WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("failed to clear memory: %w", err)
	}
	return nil
}

// Enabled is true unless the user switched the memory off.
func (s *MemoryStore) Enabled(ctx context.Context, userID string) (bool, error) {
	var enabled bool
	err := s.db.QueryRowContext(ctx, `SELECT enabled FROM commercial_memory_settings WHERE user_id = $1`, userID).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to read memory setting: %w", err)
	}
	return enabled, nil
}

// SetEnabled switches the user's memory on or off.
func (s *MemoryStore) SetEnabled(ctx context.Context, userID string, enabled bool) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO commercial_memory_settings (user_id, enabled, updated_at) VALUES ($1, $2, NOW())
	                                 ON CONFLICT (user_id) DO UPDATE SET enabled = EXCLUDED.enabled, updated_at = NOW()`, userID, enabled)
	if err != nil {
		return fmt.Errorf("failed to change memory setting: %w", err)
	}
	return nil
}


func scanMemoryItem(row rowScanner) (*domain.MemoryItem, error) {
	var item domain.MemoryItem
	var kind string
	var created, updated time.Time
	if err := row.Scan(&item.ID, &item.UserID, &kind, &item.Text, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to read memory item: %w", err)
	}
	item.Kind = domain.MemoryKind(kind)
	item.CreatedAt, item.UpdatedAt = created.Unix(), updated.Unix()
	return &item, nil
}
