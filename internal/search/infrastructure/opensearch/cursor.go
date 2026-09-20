package opensearch

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/trezutil/trezutil/internal/domain"
)

// encodeCursor encodes pagination cursor state to an opaque string.
func encodeCursor(state map[string]interface{}) (string, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cursor: %w", err)
	}
	encoded := url.QueryEscape(string(data))
	return encoded, nil
}

// decodeCursor decodes a cursor string back to state.
func decodeCursor(encoded string) (map[string]interface{}, error) {
	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to unescape cursor: %w", err)
	}

	var state map[string]interface{}
	if err := json.Unmarshal([]byte(decoded), &state); err != nil {
		return nil, fmt.Errorf("failed to decode cursor: %w", err)
	}

	return state, nil
}

// ValidateCursor validates a cursor for the given sort mode.
func ValidateCursor(cursor string, sort domain.SearchSortField) error {
	if cursor == "" {
		return nil // empty cursor is valid (first page)
	}

	state, err := decodeCursor(cursor)
	if err != nil {
		return fmt.Errorf("invalid cursor format: %w", err)
	}

	// Validate cursor fields match sort requirements
	switch sort {
	case domain.SearchSortFieldPriceAsc, domain.SearchSortFieldPriceDesc:
		if _, ok := state["price_amount"]; !ok {
			return fmt.Errorf("cursor invalid for price sort: missing price_amount")
		}
	case domain.SearchSortFieldNewest, domain.SearchSortFieldOldest:
		if _, ok := state["published_at"]; !ok {
			return fmt.Errorf("cursor invalid for date sort: missing published_at")
		}
	case domain.SearchSortFieldDistance:
		if _, ok := state["distance"]; !ok {
			return fmt.Errorf("cursor invalid for distance sort: missing distance")
		}
		// relevance and availability don't require specific fields
	}

	return nil
}

// CursorFromResult creates a cursor from the last result in a page.
func CursorFromResult(result domain.SearchResult, sort domain.SearchSortField) (string, error) {
	state := map[string]interface{}{
		"listing_id": result.ListingID,
	}

	switch sort {
	case domain.SearchSortFieldPriceAsc, domain.SearchSortFieldPriceDesc:
		state["price_amount"] = result.PriceAmount
	case domain.SearchSortFieldNewest:
		if result.PublishedAt > 0 {
			state["published_at"] = result.PublishedAt
		}
	case domain.SearchSortFieldOldest:
		if result.PublishedAt > 0 {
			state["published_at"] = result.PublishedAt
		}
	}

	return encodeCursor(state)
}
