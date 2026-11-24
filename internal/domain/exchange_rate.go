package domain

import (
	"context"
	"errors"
	"time"
)

// ExchangeRate is the CDF / USD rate a Trezutil administrator entered: how many Congolese francs make one US dollar.
// It is never fetched or estimated. It only serves an indicative conversion ("≈") beside a price and the reading of
// a budget given in francs; an order is always priced and paid in the currency the supplier set.
type ExchangeRate struct {
	ID    string
	Base  string // "USD"
	Quote string // "CDF"
	Rate  float64
	Note  string
	SetBy string
	SetAt time.Time
}

// ErrNoExchangeRate means no administrator has entered a rate yet.
var ErrNoExchangeRate = errors.New("no exchange rate has been set")

// ExchangeRateStore keeps every rate entered (migration 000036); the latest one is the current rate.
type ExchangeRateStore interface {
	// Latest returns the current USD -> CDF rate, or ErrNoExchangeRate.
	Latest(ctx context.Context) (*ExchangeRate, error)
	// Add records a new rate (ID and SetAt are filled in).
	Add(ctx context.Context, r *ExchangeRate) error
	// History returns the most recent rates first.
	History(ctx context.Context, limit int) ([]ExchangeRate, error)
}
