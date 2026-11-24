package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

// ExchangeRateStore implements domain.ExchangeRateStore (migration 000036).
type ExchangeRateStore struct {
	db *sql.DB
}

// NewExchangeRateStore creates the store.
func NewExchangeRateStore(db *sql.DB) *ExchangeRateStore {
	return &ExchangeRateStore{db: db}
}

const exchangeRateColumns = `id, base, quote, rate::float8, note, set_by, set_at`

// Latest returns the most recently entered USD -> CDF rate.
func (s *ExchangeRateStore) Latest(ctx context.Context) (*domain.ExchangeRate, error) {
	var r domain.ExchangeRate
	err := s.db.QueryRowContext(ctx, `SELECT `+exchangeRateColumns+` FROM exchange_rates
		WHERE base = 'USD' AND quote = 'CDF' ORDER BY set_at DESC, id DESC LIMIT 1`).
		Scan(&r.ID, &r.Base, &r.Quote, &r.Rate, &r.Note, &r.SetBy, &r.SetAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNoExchangeRate
	}
	if err != nil {
		return nil, fmt.Errorf("read exchange rate: %w", err)
	}
	return &r, nil
}

// Add records a new rate.
func (s *ExchangeRateStore) Add(ctx context.Context, r *domain.ExchangeRate) error {
	err := s.db.QueryRowContext(ctx, `INSERT INTO exchange_rates (base, quote, rate, note, set_by)
		VALUES ($1, $2, $3, $4, $5) RETURNING id, set_at`, r.Base, r.Quote, r.Rate, r.Note, r.SetBy).Scan(&r.ID, &r.SetAt)
	if err != nil {
		return fmt.Errorf("record exchange rate: %w", err)
	}
	return nil
}

// History returns the most recent rates first.
func (s *ExchangeRateStore) History(ctx context.Context, limit int) ([]domain.ExchangeRate, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+exchangeRateColumns+` FROM exchange_rates
		WHERE base = 'USD' AND quote = 'CDF' ORDER BY set_at DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list exchange rates: %w", err)
	}
	defer rows.Close()
	out := []domain.ExchangeRate{}
	for rows.Next() {
		var r domain.ExchangeRate
		if err := rows.Scan(&r.ID, &r.Base, &r.Quote, &r.Rate, &r.Note, &r.SetBy, &r.SetAt); err != nil {
			return nil, fmt.Errorf("scan exchange rate: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
