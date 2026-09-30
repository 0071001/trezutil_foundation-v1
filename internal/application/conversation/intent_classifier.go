package conversation

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/trezutil/trezutil/internal/application/vertical"
	"github.com/trezutil/trezutil/internal/domain"
)

// IntentClassificationResult is defined on processor.go.
// This file provides an adapter for wrapping AIOrchestrator.
// Actual adapter wiring happens in the cmd package to avoid import cycles.

// noopClassifier is a fallback classifier that returns no intent.
type noopClassifier struct{}

func (n *noopClassifier) ClassifyIntent(ctx context.Context, req *IntentClassificationRequest) (*IntentClassificationResult, error) {
	return &IntentClassificationResult{}, nil
}

// NewNoopClassifier returns a classifier that always returns no intent
// (used when no AI provider is configured — falls back to keyword resolution).
func NewNoopClassifier() IntentClassifier {
	return &noopClassifier{}
}

// KeywordClassifier does keyword-based intent classification without AI.
type KeywordClassifier struct{}

func (k *KeywordClassifier) ClassifyIntent(ctx context.Context, req *IntentClassificationRequest) (*IntentClassificationResult, error) {
	actionType := resolveActionTypeKeyword(req.Messages[len(req.Messages)-1].Content.Text)
	if actionType == "" {
		// Naming a vertical is a search ("un hôtel à Gombe").
		if v, _ := vertical.DefaultCatalog().DetectVertical(req.Messages[len(req.Messages)-1].Content.Text); v != "" {
			actionType = domain.ActionSearchListings
		}
	}
	if actionType == "" {
		return &IntentClassificationResult{}, nil
	}

	specialist := domain.GetSpecialistsForAction(actionType)
	intent := &domain.AIIntent{
		Intent:      actionType,
		Confidence:  1.0,
		Parameters:  make(map[string]interface{}),
		Explanation: "keyword-based classification",
	}
	if actionType == domain.ActionSearchListings {
		// The user's own words are the search query. A vertical named by a
		// catalogue keyword ("hotel", "restaurant") becomes a filter and its
		// keyword leaves the text. Place, nights and party size need the AI
		// interpreter; here they stay in the text, where they still rank results.
		text := req.Messages[len(req.Messages)-1].Content.Text
		if v, matched := vertical.DefaultCatalog().DetectVertical(text); v != "" {
			intent.Parameters["vertical"] = string(v)
			text = withoutWords(text, append(matched, searchFillers...))
		}
		if text != "" {
			intent.Parameters["text"] = text
		}
	}
	if len(specialist) > 0 {
		_ = specialist[0]
	}

	return &IntentClassificationResult{
		Intent: intent,
	}, nil
}

func init() {
	_ = time.Now
}

// searchFillers are request words that carry no search meaning.
var searchFillers = []string{"find", "me", "a", "an", "the", "i", "am", "want", "need", "looking", "for", "search",
	"show", "please", "some", "any", "in", "at", "je", "cherche", "un", "une", "des", "le", "la", "les", "pour", "à", "dans"}

// withoutWords removes the given words (case-insensitive) from text.
func withoutWords(text string, words []string) string {
	drop := map[string]bool{}
	for _, w := range words {
		drop[strings.ToLower(w)] = true
	}
	var kept []string
	for _, w := range strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' }) {
		if !drop[strings.ToLower(w)] {
			kept = append(kept, w)
		}
	}
	return strings.Join(kept, " ")
}
