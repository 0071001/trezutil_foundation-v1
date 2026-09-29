package conversation

import (
	"context"
	"time"

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
		// The user's own words are the search query.
		intent.Parameters["text"] = req.Messages[len(req.Messages)-1].Content.Text
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
