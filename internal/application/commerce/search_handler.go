package commerce

import (
	"context"

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

	if pageStr, ok := parameters["page"].(string); ok && pageStr != "" {
		query.Pagination.Cursor = pageStr
	}

	if pageSize, ok := parameters["page_size"].(int); ok && pageSize > 0 {
		query.Pagination.PageSize = pageSize
	}

	result, err := h.searchUseCases.ExecuteSearch(ctx, actor.ID, query)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"items":       result.Items,
		"total":       result.Total,
		"has_next":    result.HasNext,
		"next_cursor": result.NextCursor,
	}, nil
}

// ActionType returns the action type this handler handles.
func (h *SearchHandler) ActionType() domain.CommerceActionType {
	return domain.ActionSearchListings
}
