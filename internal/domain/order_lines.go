package domain

import (
	"errors"
	"fmt"
)

// Limits for a multi-item order (a cart).
const (
	MaxOrderLines   = 30
	MaxLineQuantity = 99
)

var (
	// ErrInvalidOrderLines is returned for an empty, oversized or malformed cart.
	ErrInvalidOrderLines = errors.New("invalid order lines")
	// ErrOrderMixedStores is returned when a cart spans stores or currencies.
	ErrOrderMixedStores = errors.New("an order can only contain items from one store and one currency")
)

// OrderLine is one requested item of an order: a listing and a quantity.
type OrderLine struct {
	ListingID string
	Quantity  int
}

// NormalizeOrderLines validates a cart and merges repeated listings by
// summing their quantities, keeping first-appearance order.
func NormalizeOrderLines(lines []OrderLine) ([]OrderLine, error) {
	if len(lines) == 0 {
		return nil, fmt.Errorf("%w: at least one item is required", ErrInvalidOrderLines)
	}
	index := make(map[string]int, len(lines))
	out := make([]OrderLine, 0, len(lines))
	for _, l := range lines {
		if l.ListingID == "" {
			return nil, fmt.Errorf("%w: listing_id is required", ErrInvalidOrderLines)
		}
		if l.Quantity <= 0 {
			return nil, fmt.Errorf("%w: quantity must be positive", ErrInvalidOrderLines)
		}
		if i, ok := index[l.ListingID]; ok {
			out[i].Quantity += l.Quantity
		} else {
			index[l.ListingID] = len(out)
			out = append(out, l)
		}
	}
	if len(out) > MaxOrderLines {
		return nil, fmt.Errorf("%w: at most %d different items", ErrInvalidOrderLines, MaxOrderLines)
	}
	for _, l := range out {
		if l.Quantity > MaxLineQuantity {
			return nil, fmt.Errorf("%w: at most %d of one item", ErrInvalidOrderLines, MaxLineQuantity)
		}
	}
	return out, nil
}

// SameLines reports whether an order's items are exactly the given lines
// (used to decide whether an idempotent retry repeats the same order).
func (o *Order) SameLines(lines []OrderLine) bool {
	if len(o.Items) != len(lines) {
		return false
	}
	want := make(map[string]int, len(lines))
	for _, l := range lines {
		want[l.ListingID] = l.Quantity
	}
	for _, it := range o.Items {
		if q, ok := want[it.ListingID]; !ok || q != it.Quantity {
			return false
		}
	}
	return true
}
