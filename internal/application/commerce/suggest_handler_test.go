package commerce

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

type fakeSuggestUseCases struct {
	resp domain.SuggestResponse
	err  error
}

func (f *fakeSuggestUseCases) ExecuteSearch(ctx context.Context, actorID string, query domain.DiscoveryQuery) (domain.SearchResultPage, error) {
	return domain.SearchResultPage{}, nil
}

func (f *fakeSuggestUseCases) Suggest(ctx context.Context, actorID string, req domain.SuggestRequest) (domain.SuggestResponse, error) {
	return f.resp, f.err
}

func TestSuggestHandler_ActionType(t *testing.T) {
	handler := NewSuggestHandler(&fakeSuggestUseCases{})
	if handler.ActionType() != domain.ActionSuggest {
		t.Errorf("expected %s, got %s", domain.ActionSuggest, handler.ActionType())
	}
}

func TestSuggestHandler_Handle_Success(t *testing.T) {
	resp := domain.SuggestResponse{
		Suggestions: []domain.Suggestion{
			{Text: "test product", CategoryID: strPtr("cat-1"), Type: "product"},
		},
	}
	handler := NewSuggestHandler(&fakeSuggestUseCases{resp: resp})
	params := map[string]interface{}{"text": "test"}

	result, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	m := result.(map[string]interface{})
	suggestions, ok := m["suggestions"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected suggestions array, got %T", m["suggestions"])
	}
	if len(suggestions) != 1 {
		t.Errorf("expected 1 suggestion, got %d", len(suggestions))
	}
}

func TestSuggestHandler_Handle_EmptyText(t *testing.T) {
	handler := NewSuggestHandler(&fakeSuggestUseCases{})
	params := map[string]interface{}{"text": ""}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err != domain.ErrInvalidParameter {
		t.Errorf("expected ErrInvalidParameter, got %v", err)
	}
}

func TestSuggestHandler_Handle_MissingText(t *testing.T) {
	handler := NewSuggestHandler(&fakeSuggestUseCases{})

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), map[string]interface{}{})
	if err != domain.ErrInvalidParameter {
		t.Errorf("expected ErrInvalidParameter, got %v", err)
	}
}

func TestSuggestHandler_Handle_SearchError(t *testing.T) {
	handler := NewSuggestHandler(&fakeSuggestUseCases{err: domain.ErrListingNotFound})
	params := map[string]interface{}{"text": "test"}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err != domain.ErrListingNotFound {
		t.Errorf("expected ErrListingNotFound, got %v", err)
	}
}

func TestSuggestHandler_Handle_WithCategoryAndLimit(t *testing.T) {
	resp := domain.SuggestResponse{Suggestions: []domain.Suggestion{}}
	handler := NewSuggestHandler(&fakeSuggestUseCases{resp: resp})
	params := map[string]interface{}{
		"text":        "test",
		"category_id": "cat-1",
		"limit":       5,
	}

	result, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func strPtr(s string) *string {
	return &s
}
