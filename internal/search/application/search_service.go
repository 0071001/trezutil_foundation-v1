package application

import (
	"context"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// SearchService handles search and discovery operations.
type SearchService struct {
	searchEngine            domain.SearchEnginePort
	visibilityResolver      domain.VisibilityResolver
	authorizationResolver   domain.AuthorizationResolver
	listingProjectionReader domain.ListingProjectionReader
	cursorCodec             domain.CursorCodec
	clock                   domain.Clock
}

// SearchUseCases defines the interface for search operations.
type SearchUseCases interface {
	ExecuteSearch(ctx context.Context, actorID string, query domain.DiscoveryQuery) (domain.SearchResultPage, error)
	Suggest(ctx context.Context, actorID string, req domain.SuggestRequest) (domain.SuggestResponse, error)
}

// NewSearchService creates a new SearchService.
func NewSearchService(
	searchEngine domain.SearchEnginePort,
	visibilityResolver domain.VisibilityResolver,
	authorizationResolver domain.AuthorizationResolver,
	listingProjectionReader domain.ListingProjectionReader,
	cursorCodec domain.CursorCodec,
	clock domain.Clock,
) *SearchService {
	return &SearchService{
		searchEngine:            searchEngine,
		visibilityResolver:      visibilityResolver,
		authorizationResolver:   authorizationResolver,
		listingProjectionReader: listingProjectionReader,
		cursorCodec:             cursorCodec,
		clock:                   clock,
	}
}

// ExecuteSearch performs a search query.
func (s *SearchService) ExecuteSearch(ctx context.Context, actorID string, query domain.DiscoveryQuery) (domain.SearchResultPage, error) {
	// Resolve visibility
	visibility, err := s.visibilityResolver.ResolveVisibility(ctx, actorID)
	if err != nil {
		return domain.SearchResultPage{}, fmt.Errorf("failed to resolve visibility: %w", err)
	}

	// Authorize search
	if err := s.authorizationResolver.AuthorizeSearch(ctx, actorID, visibility); err != nil {
		return domain.SearchResultPage{}, fmt.Errorf("unauthorized: %w", err)
	}

	// Inject visibility constraints into query
	query.SellerID = actorID
	if visibility.OrganizationID != nil {
		query.StoreID = *visibility.OrganizationID
	}

	// Execute search via engine
	result, err := s.searchEngine.ExecuteSearch(ctx, query)
	if err != nil {
		return domain.SearchResultPage{}, fmt.Errorf("search engine error: %w", err)
	}

	return result, nil
}

// Suggest performs a suggestion/autocomplete query.
func (s *SearchService) Suggest(ctx context.Context, actorID string, req domain.SuggestRequest) (domain.SuggestResponse, error) {
	// Resolve visibility
	visibility, err := s.visibilityResolver.ResolveVisibility(ctx, actorID)
	if err != nil {
		return domain.SuggestResponse{}, fmt.Errorf("failed to resolve visibility: %w", err)
	}

	// Authorize search
	if err := s.authorizationResolver.AuthorizeSearch(ctx, actorID, visibility); err != nil {
		return domain.SuggestResponse{}, fmt.Errorf("unauthorized: %w", err)
	}

	// For now, return empty suggestions
	// In the future, this will delegate to a suggest engine
	return domain.SuggestResponse{
		Suggestions: []domain.Suggestion{},
	}, nil
}

// InMemorySearchEngine is a stub implementation of SearchEnginePort for testing.
type InMemorySearchEngine struct {
	listings map[string][]domain.SearchResult
}

// NewInMemorySearchEngine creates a new InMemorySearchEngine.
func NewInMemorySearchEngine() *InMemorySearchEngine {
	return &InMemorySearchEngine{
		listings: make(map[string][]domain.SearchResult),
	}
}

// ExecuteSearch implements SearchEnginePort.
func (e *InMemorySearchEngine) ExecuteSearch(ctx context.Context, query domain.DiscoveryQuery) (domain.SearchResultPage, error) {
	results := e.listings[query.Text]
	if results == nil {
		results = []domain.SearchResult{}
	}

	// Apply pagination
	pageSize := query.Pagination.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	start := 0
	if query.Pagination.Cursor != "" {
		// Simple cursor implementation for stub
		start = len(results) // For now, return empty if cursor is provided
	}

	end := start + pageSize
	if end > len(results) {
		end = len(results)
	}

	pagedResults := results[start:end]
	hasNext := end < len(results)

	var nextCursor *string
	if hasNext {
		cursor := fmt.Sprintf("%d", end)
		nextCursor = &cursor
	}

	return domain.SearchResultPage{
		Items:      pagedResults,
		Total:      len(results),
		HasNext:    hasNext,
		NextCursor: nextCursor,
	}, nil
}

// AddListing adds a listing to the in-memory engine for testing.
func (e *InMemorySearchEngine) AddListing(text string, result domain.SearchResult) {
	e.listings[text] = append(e.listings[text], result)
}

// NoopVisibilityResolver is a stub implementation that returns public visibility.
type NoopVisibilityResolver struct{}

// NewNoopVisibilityResolver creates a new NoopVisibilityResolver.
func NewNoopVisibilityResolver() *NoopVisibilityResolver {
	return &NoopVisibilityResolver{}
}

// ResolveVisibility implements VisibilityResolver.
func (r *NoopVisibilityResolver) ResolveVisibility(ctx context.Context, actorID string) (domain.SearchVisibility, error) {
	return domain.SearchVisibility{
		Mode: domain.SearchVisibilityModePublic,
	}, nil
}

// AllowAllAuthorizationResolver is a stub implementation that allows all searches.
type AllowAllAuthorizationResolver struct{}

// NewAllowAllAuthorizationResolver creates a new AllowAllAuthorizationResolver.
func NewAllowAllAuthorizationResolver() *AllowAllAuthorizationResolver {
	return &AllowAllAuthorizationResolver{}
}

// AuthorizeSearch implements AuthorizationResolver.
func (r *AllowAllAuthorizationResolver) AuthorizeSearch(ctx context.Context, actorID string, visibility domain.SearchVisibility) error {
	return nil
}

// NoopCursorCodec is a stub implementation of CursorCodec.
type NoopCursorCodec struct{}

// NewNoopCursorCodec creates a new NoopCursorCodec.
func NewNoopCursorCodec() *NoopCursorCodec {
	return &NoopCursorCodec{}
}

// Encode implements CursorCodec.
func (c *NoopCursorCodec) Encode(cursor domain.PaginationCursor) (string, error) {
	return cursor.Cursor, nil
}

// Decode implements CursorCodec.
func (c *NoopCursorCodec) Decode(encoded string) (domain.PaginationCursor, error) {
	return domain.PaginationCursor{
		Mode:     domain.PaginationModeCursor,
		Cursor:   encoded,
		PageSize: 10,
	}, nil
}

// SystemClock is a real implementation of Clock.
type SystemClock struct{}

// NewSystemClock creates a new SystemClock.
func NewSystemClock() *SystemClock {
	return &SystemClock{}
}

// Now implements Clock.
func (c *SystemClock) Now() int64 {
	return time.Now().Unix()
}
