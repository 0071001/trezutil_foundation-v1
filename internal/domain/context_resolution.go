package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ReferenceResolutionStrategy determines how contextual references are resolved.
type ReferenceResolutionStrategy string

const (
	// StrategyOrdinal resolves "the first one", "the second one", etc.
	StrategyOrdinal ReferenceResolutionStrategy = "ordinal"
	// StrategyCurrent resolves "the current one", "this one", "it".
	StrategyCurrent ReferenceResolutionStrategy = "current"
	// StrategyLatest resolves "the latest", "most recent".
	StrategyLatest ReferenceResolutionStrategy = "latest"
	// StrategyNone indicates no contextual reference.
	StrategyNone ReferenceResolutionStrategy = "none"
)

// ContextualReference represents a resolved reference to a prior entity in the conversation.
type ContextualReference struct {
	Strategy       ReferenceResolutionStrategy
	Ordinal        int
	EntityType     string // "listing", "order", "reservation"
	RawText        string
	ResolvedItemID string
}

var ordinalPattern = regexp.MustCompile(`(?i)\b(first|second|third|fourth|fifth|last|previous)\b`)

var ordinalMap = map[string]int{
	"first":    1,
	"second":   2,
	"third":    3,
	"fourth":   4,
	"fifth":    5,
	"last":     -1,
	"latest":   -1,
	"previous": -2,
}

var currentPattern = regexp.MustCompile(`(?i)\b(current|this|it|the\s+one)\b`)

// ResolveContextualReference parses a parameter value for contextual references
// like "the first one", "second listing", or "that one".
// If a reference is found, it attempts to resolve it against the provided context.
func ResolveContextualReference(value string, ctx *ConversationContext) *ContextualReference {
	ref := &ContextualReference{
		RawText: value,
	}

	lower := strings.ToLower(strings.TrimSpace(value))

	// Check for "current"/"this"/"it" patterns first (word-boundary match)
	if currentPattern.MatchString(lower) {
		ref.Strategy = StrategyCurrent
		ref.ResolvedItemID = resolveFromContext(ctx, StrategyCurrent, 0)
		return ref
	}

	// Check for ordinal patterns: "the first one", "second listing", etc.
	matches := ordinalPattern.FindStringSubmatch(lower)
	if len(matches) > 1 {
		word := matches[1]
		if ord, ok := ordinalMap[word]; ok {
			ref.Strategy = StrategyOrdinal
			ref.Ordinal = ord
			ref.EntityType = detectEntityType(lower)
			ref.ResolvedItemID = resolveFromContext(ctx, ref.Strategy, ref.Ordinal)
		}
		return ref
	}

	// Check for "latest"/"recent" patterns
	if strings.Contains(lower, "latest") || strings.Contains(lower, "most recent") || strings.Contains(lower, "recent") {
		ref.Strategy = StrategyLatest
		ref.ResolvedItemID = resolveFromContext(ctx, StrategyLatest, 0)
		return ref
	}

	return nil
}

func detectEntityType(text string) string {
	if strings.Contains(text, "listing") || strings.Contains(text, "product") {
		return "listing"
	}
	if strings.Contains(text, "order") {
		return "order"
	}
	if strings.Contains(text, "reservation") || strings.Contains(text, "booking") {
		return "reservation"
	}
	return ""
}

func resolveFromContext(ctx *ConversationContext, strategy ReferenceResolutionStrategy, ordinal int) string {
	if ctx == nil {
		return ""
	}

	// For StrategyCurrent, check current entity refs first
	if strategy == StrategyCurrent {
		if ctx.CurrentListing != nil && ctx.CurrentListing.ID != "" {
			return ctx.CurrentListing.ID
		}
		if ctx.CurrentOrder != nil && ctx.CurrentOrder.ID != "" {
			return ctx.CurrentOrder.ID
		}
		if ctx.CurrentReservation != nil && ctx.CurrentReservation.ID != "" {
			return ctx.CurrentReservation.ID
		}
	}

	items := ctx.ResultItems
	if len(items) == 0 {
		return ""
	}

	switch strategy {
	case StrategyCurrent:
		return ""
	case StrategyOrdinal:
		if ordinal > 0 && ordinal <= len(items) {
			return items[ordinal-1].ID
		}
		if ordinal == -1 {
			return items[len(items)-1].ID
		}
		if ordinal == -2 {
			if len(items) >= 2 {
				return items[len(items)-2].ID
			}
		}
	case StrategyLatest:
		if len(items) > 0 {
			return items[len(items)-1].ID
		}
	}

	return ""
}

// ResolveParameterValue resolves a parameter value using conversational context.
// If the value contains a contextual reference (e.g., "the first one"),
// it resolves the reference against the conversation context's result items.
func ResolveParameterValue(name, value string, ctx *ConversationContext) (string, bool) {
	if ctx == nil || value == "" {
		return value, false
	}

	ref := ResolveContextualReference(value, ctx)
	if ref == nil || ref.ResolvedItemID == "" {
		return value, false
	}

	return ref.ResolvedItemID, true
}

// ParseOrdinal parses an ordinal from text like "first", "2nd", "third".
func ParseOrdinal(text string) (int, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))

	// Word ordinals
	if ord, ok := ordinalMap[lower]; ok {
		return ord, true
	}

	// Numeric ordinals: "1st", "2nd", "3rd", "4th"
	numStr := lower
	numStr = strings.TrimRight(numStr, "stndrh")
	if num, err := strconv.Atoi(numStr); err == nil {
		return num, true
	}

	return 0, false
}

// ResolveOrdinalReference resolves an ordinal reference against result items.
func ResolveOrdinalReference(ordinalStr string, ctx *ConversationContext) (*ResultItemRef, error) {
	if ctx == nil {
		return nil, fmt.Errorf("conversation context is nil")
	}

	ord, ok := ParseOrdinal(ordinalStr)
	if !ok {
		return nil, fmt.Errorf("could not parse ordinal: %s", ordinalStr)
	}

	items := ctx.ResultItems
	if len(items) == 0 {
		return nil, fmt.Errorf("no result items in conversation context")
	}

	if ord > 0 && ord <= len(items) {
		return &items[ord-1], nil
	}

	if ord == -1 {
		return &items[len(items)-1], nil
	}

	if ord == -2 && len(items) >= 2 {
		return &items[len(items)-2], nil
	}

	return nil, fmt.Errorf("ordinal %d out of range (have %d items)", ord, len(items))
}
