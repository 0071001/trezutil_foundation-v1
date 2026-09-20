package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/trezutil/trezutil/internal/domain"
)

// SearchUseCases interface for search operations.
type SearchUseCases interface {
	ExecuteSearch(ctx context.Context, actorID string, query domain.DiscoveryQuery) (domain.SearchResultPage, error)
	Suggest(ctx context.Context, actorID string, req domain.SuggestRequest) (domain.SuggestResponse, error)
}

// SearchHandler handles search HTTP requests.
type SearchHandler struct {
	searchUseCases SearchUseCases
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(searchUseCases SearchUseCases) *SearchHandler {
	return &SearchHandler{searchUseCases: searchUseCases}
}

// SearchRequest represents a search request DTO.
type SearchRequest struct {
	Text          string `json:"text"`
	CategoryID    string `json:"category_id"`
	PageSize      int    `json:"page_size"`
	Cursor        string `json:"cursor"`
	SortField     string `json:"sort_field"`
	SortDirection string `json:"sort_direction"`
}

// SearchResponse represents a search response DTO.
type SearchResponse struct {
	Items      []domain.SearchResult `json:"items"`
	Total      int                   `json:"total"`
	HasNext    bool                  `json:"has_next"`
	NextCursor *string               `json:"next_cursor,omitempty"`
}

// SuggestRequestDTO represents a suggest request DTO.
type SuggestRequestDTO struct {
	Text       string `json:"text"`
	CategoryID string `json:"category_id"`
	Limit      int    `json:"limit"`
}

// SuggestResponseDTO represents a suggest response DTO.
type SuggestResponseDTO struct {
	Suggestions []domain.Suggestion `json:"suggestions"`
}

// SearchH handles POST /v1/search
func (h *SearchHandler) SearchH(w http.ResponseWriter, r *http.Request) {
	actorID := r.URL.Query().Get("user_id")
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	var req SearchRequest
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}

	// Build discovery query
	query := domain.DiscoveryQuery{
		Text: req.Text,
		Pagination: domain.PaginationCursor{
			Mode:     domain.PaginationModeCursor,
			Cursor:   req.Cursor,
			PageSize: req.PageSize,
		},
	}

	if req.SortField != "" {
		sortField := domain.SearchSortField(req.SortField)
		sortDir := domain.SearchSortDirectionDesc
		if req.SortDirection == "asc" {
			sortDir = domain.SearchSortDirectionAsc
		}
		sort := domain.NewSearchSort(sortField, sortDir)
		query.Sort = &sort
	}

	if req.CategoryID != "" {
		query.Category = &domain.CategoryFilter{
			CategoryID: req.CategoryID,
		}
	}

	// Execute search
	result, err := h.searchUseCases.ExecuteSearch(r.Context(), actorID, query)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "SEARCH_FAILED", Message: err.Error()}}, http.StatusInternalServerError)
		return
	}

	response := SearchResponse{
		Items:   result.Items,
		Total:   result.Total,
		HasNext: result.HasNext,
	}

	if result.NextCursor != nil {
		response.NextCursor = result.NextCursor
	}

	writeJSON(w, response, http.StatusOK)
}

// SuggestH handles GET /v1/search/suggest
func (h *SearchHandler) SuggestH(w http.ResponseWriter, r *http.Request) {
	actorID := r.URL.Query().Get("user_id")
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "User ID required"}}, http.StatusUnauthorized)
		return
	}

	text := r.URL.Query().Get("text")
	if text == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_TEXT", Message: "Text parameter required"}}, http.StatusBadRequest)
		return
	}

	categoryID := r.URL.Query().Get("category_id")
	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	req := domain.SuggestRequest{
		Text:       text,
		CategoryID: &categoryID,
		Limit:      limit,
	}

	result, err := h.searchUseCases.Suggest(r.Context(), actorID, req)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "SUGGEST_FAILED", Message: err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeJSON(w, SuggestResponseDTO{Suggestions: result.Suggestions}, http.StatusOK)
}
