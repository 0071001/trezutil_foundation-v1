package commerce

import (
	"context"
	"strings"

	"github.com/trezutil/trezutil/internal/domain"
)

// SearchHandler handles SEARCH_LISTINGS actions.
type SearchHandler struct {
	searchUseCases SearchUseCases
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(searchUseCases SearchUseCases) *SearchHandler {
	return &SearchHandler{searchUseCases: searchUseCases}
}

// Handle executes a search listings action.
func (h *SearchHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	query := domain.DiscoveryQuery{}

	if text, ok := parameters["text"].(string); ok {
		query.Text = text
	}

	if sellerID, ok := parameters["seller_id"].(string); ok && sellerID != "" {
		query.SellerID = sellerID
	}

	if storeID, ok := parameters["store_id"].(string); ok && storeID != "" {
		query.StoreID = storeID
	}

	// Vertical and location arrive validated and normalized by the runtime
	// parameter validator. nights and party_size do not filter search: a party
	// of four can take two double rooms; they are used when checking availability.
	if v, ok := parameters["vertical"].(string); ok && v != "" {
		query.Vertical = domain.NormalizeVertical(v)
	}
	if place, ok := parameters["location"].(string); ok {
		query.Place = strings.TrimSpace(place)
	}

	applySearchFilters(&query, parameters)

	if pageStr, ok := parameters["page"].(string); ok && pageStr != "" {
		query.Pagination.Cursor = pageStr
	}

	if pageSize, ok := intParam(parameters, "page_size"); ok && pageSize > 0 {
		query.Pagination.PageSize = pageSize
	}

	if query.Pagination.PageSize <= 0 || query.Pagination.PageSize > 50 {
		query.Pagination.PageSize = 10
	}
	query.Pagination.Mode = domain.PaginationModeCursor

	result, err := h.searchUseCases.ExecuteSearch(ctx, actor.ID, query)
	if err != nil {
		return nil, err
	}

	// Place data is often incomplete (a store without a city). When the place
	// filter finds nothing, search again with the place as ranking words and
	// say so, rather than answering "nothing" or dropping it silently. The
	// vertical is never relaxed.
	relaxed := false
	if result.Total == 0 && query.Place != "" {
		wider := query
		wider.Text = strings.TrimSpace(query.Text + " " + query.Place)
		wider.Place = ""
		if again, err := h.searchUseCases.ExecuteSearch(ctx, actor.ID, wider); err == nil && again.Total > 0 {
			result, relaxed = again, true
		}
	}

	out := map[string]interface{}{
		"items":       result.Items,
		"total":       result.Total,
		"has_next":    result.HasNext,
		"next_cursor": result.NextCursor,
	}
	if criteria := searchCriteria(query, parameters); len(criteria) > 0 {
		out["criteria"] = criteria
	}
	if relaxed {
		out["relaxed"] = []string{"location"}
	}
	return out, nil
}

// searchCriteria echoes the structured criteria the search ran with, so the
// reply can state them ("Hotels in Gombe · 2 nights · 2 guests").
func searchCriteria(query domain.DiscoveryQuery, parameters map[string]interface{}) map[string]interface{} {
	c := map[string]interface{}{}
	if query.Vertical != "" {
		c["vertical"] = string(query.Vertical)
	}
	if query.Place != "" {
		c["location"] = query.Place
	}
	for _, k := range []string{"nights", "party_size"} {
		if n, ok := intParam(parameters, k); ok && n > 0 {
			c[k] = n
		}
	}
	for _, k := range []string{"price_min", "price_max"} {
		if f, ok := floatParam(parameters, k); ok && f > 0 {
			c[k] = f
		}
	}
	if s, ok := parameters["sort"].(string); ok && s != "" && s != "relevance" {
		c["sort"] = s
	}
	if s, ok := parameters["category_id"].(string); ok && s != "" {
		c["category_id"] = s
	}
	return c
}

// ActionType returns the action type this handler handles.
func (h *SearchHandler) ActionType() domain.CommerceActionType {
	return domain.ActionSearchListings
}
