package commerce

import (
	"context"

	"github.com/trezutil/trezutil/internal/domain"
)

// SuggestHandler handles SUGGEST actions.
type SuggestHandler struct {
	searchUseCases SearchUseCases
}

// NewSuggestHandler creates a new SuggestHandler.
func NewSuggestHandler(searchUseCases SearchUseCases) *SuggestHandler {
	return &SuggestHandler{searchUseCases: searchUseCases}
}

// Handle executes a suggest action.
func (h *SuggestHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	text, ok := parameters["text"].(string)
	if !ok || text == "" {
		return nil, domain.ErrInvalidParameter
	}

	var categoryID *string
	if v, ok := parameters["category_id"].(string); ok && v != "" {
		categoryID = &v
	}

	limit := 10
	if v, ok := intParam(parameters, "limit"); ok && v > 0 {
		limit = v
	}

	req := domain.SuggestRequest{
		Text:       text,
		CategoryID: categoryID,
		Limit:      limit,
	}

	resp, err := h.searchUseCases.Suggest(ctx, actor.ID, req)
	if err != nil {
		return nil, err
	}

	suggestions := make([]map[string]interface{}, len(resp.Suggestions))
	for i, s := range resp.Suggestions {
		suggestions[i] = map[string]interface{}{
			"text":        s.Text,
			"category_id": s.CategoryID,
			"type":        s.Type,
		}
	}

	return map[string]interface{}{
		"suggestions": suggestions,
	}, nil
}

// ActionType returns the action type this handler handles.
func (h *SuggestHandler) ActionType() domain.CommerceActionType {
	return domain.ActionSuggest
}
