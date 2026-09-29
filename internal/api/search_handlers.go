package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
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

// Page-size bounds for public search.
const (
	defaultSearchPageSize = 20
	maxSearchPageSize     = 100
)

// SearchH handles POST /v1/search. Discovery is public: anonymous callers
// (guests) search published listings; an authenticated caller is identified
// only through the verified principal.
func (h *SearchHandler) SearchH(w http.ResponseWriter, r *http.Request) {
	actorID := auth.UserID(r.Context())

	var req SearchRequest
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}

	if req.PageSize <= 0 {
		req.PageSize = defaultSearchPageSize
	}
	if req.PageSize > maxSearchPageSize {
		req.PageSize = maxSearchPageSize
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
		writeJSON(w, ErrorResponse{Error: Error{Code: "SEARCH_FAILED", Message: "Search is temporarily unavailable"}}, http.StatusServiceUnavailable)
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

// SuggestH handles GET /v1/search/suggest (public, like search).
func (h *SearchHandler) SuggestH(w http.ResponseWriter, r *http.Request) {
	actorID := auth.UserID(r.Context())

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

	if limit > 20 {
		limit = 20
	}
	req := domain.SuggestRequest{
		Text:  text,
		Limit: limit,
	}
	if categoryID != "" {
		req.CategoryID = &categoryID
	}

	result, err := h.searchUseCases.Suggest(r.Context(), actorID, req)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "SUGGEST_FAILED", Message: err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeJSON(w, SuggestResponseDTO{Suggestions: result.Suggestions}, http.StatusOK)
}
