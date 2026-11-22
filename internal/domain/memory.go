package domain

import (
	"context"
	"errors"
	"regexp"
)

// MemoryKind is one of the six kinds of commercial memory.
type MemoryKind string

const (
	MemoryPreference MemoryKind = "preference"
	MemoryBrand      MemoryKind = "brand"
	MemoryBudget     MemoryKind = "budget"
	MemoryConstraint MemoryKind = "constraint"
	MemorySupplier   MemoryKind = "supplier"
	MemoryProject    MemoryKind = "project"
)

// MemoryKinds lists the kinds in the order they are shown and given to the assistant.
var MemoryKinds = []MemoryKind{MemoryPreference, MemoryBrand, MemoryBudget, MemoryConstraint, MemorySupplier, MemoryProject}

// IsValid reports whether k is one of the six kinds.
func (k MemoryKind) IsValid() bool {
	for _, known := range MemoryKinds {
		if k == known {
			return true
		}
	}
	return false
}

// MemoryItem is one thing a person chose to let Trezutil remember.
type MemoryItem struct {
	ID        string
	UserID    string
	Kind      MemoryKind
	Text      string
	CreatedAt int64
	UpdatedAt int64
}

// MemoryStore keeps a person's commercial memory. Every method is scoped by the user: an item of somebody else is
// never read, changed or deleted (ErrMemoryItemNotFound).
type MemoryStore interface {
	List(ctx context.Context, userID string) ([]MemoryItem, error)
	Count(ctx context.Context, userID string) (int, error)
	Add(ctx context.Context, item *MemoryItem) error
	UpdateText(ctx context.Context, userID, itemID, text string) (*MemoryItem, error)
	Delete(ctx context.Context, userID, itemID string) error
	DeleteAll(ctx context.Context, userID string) error
	// Enabled is true when the person never switched the memory off.
	Enabled(ctx context.Context, userID string) (bool, error)
	SetEnabled(ctx context.Context, userID string, enabled bool) error
}

// ErrMemoryItemNotFound: no item with that id belongs to that user.
var ErrMemoryItemNotFound = errors.New("memory item not found")

// sensitivePatterns are what commercial memory never keeps: payment data, codes and secrets.
var sensitivePatterns = []struct {
	pattern *regexp.Regexp
	what    string
}{
	{regexp.MustCompile(`\b(?:\d[ -]?){13,19}\b`), "card_number"},
	{regexp.MustCompile(`(?i)\b(cvv|cvc|code pin|pin|mot de passe|password|otp|code de v[ée]rification)\b`), "secret"},
	{regexp.MustCompile(`(?i)\b(iban|swift|bic)\b`), "bank_details"},
}

// MemorySensitiveReason returns why a text must not be remembered ("card_number", "secret", "bank_details"), or "".
func MemorySensitiveReason(text string) string {
	for _, s := range sensitivePatterns {
		if s.pattern.MatchString(text) {
			return s.what
		}
	}
	return ""
}
