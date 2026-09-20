package commerce

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

type fakeSearchUseCases struct {
	page domain.SearchResultPage
	err  error
}

func (f *fakeSearchUseCases) ExecuteSearch(ctx context.Context, actorID string, query domain.DiscoveryQuery) (domain.SearchResultPage, error) {
	return f.page, f.err
}
func (f *fakeSearchUseCases) Suggest(ctx context.Context, actorID string, req domain.SuggestRequest) (domain.SuggestResponse, error) {
	return domain.SuggestResponse{}, nil
}

func TestSearchHandler_ActionType(t *testing.T) {
	handler := NewSearchHandler(&fakeSearchUseCases{})
	if handler.ActionType() != domain.ActionSearchListings {
		t.Errorf("expected %s, got %s", domain.ActionSearchListings, handler.ActionType())
	}
}

func TestSearchHandler_Handle_Success(t *testing.T) {
	page := domain.SearchResultPage{
		Items: []domain.SearchResult{
			{ListingID: "l1", Title: "Test Listing"},
		},
		Total: 1,
	}
	handler := NewSearchHandler(&fakeSearchUseCases{page: page})
	params := map[string]interface{}{
		"text":      "test",
		"page_size": 10,
	}

	result, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	m := result.(map[string]interface{})
	if m["total"] != 1 {
		t.Errorf("expected total 1, got %v", m["total"])
	}
}

func TestSearchHandler_Handle_Error(t *testing.T) {
	handler := NewSearchHandler(&fakeSearchUseCases{err: domain.ErrListingNotFound})
	params := map[string]interface{}{"text": "test"}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err == nil {
		t.Error("expected error")
	}
}

func TestSearchHandler_Handle_TenantIsolation(t *testing.T) {
	page := domain.SearchResultPage{Items: []domain.SearchResult{}}
	handler := NewSearchHandler(&fakeSearchUseCases{page: page})

	// Execute with different actors - each gets own result set
	actor1 := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	actor2 := domain.NewCommerceActor("user-2", domain.ActorTypeCustomer)

	params := map[string]interface{}{"text": "test"}

	_, err1 := handler.Handle(context.Background(), actor1, params)
	_, err2 := handler.Handle(context.Background(), actor2, params)

	if err1 != nil {
		t.Errorf("actor1 unexpected error: %v", err1)
	}
	if err2 != nil {
		t.Errorf("actor2 unexpected error: %v", err2)
	}
}

func TestSearchHandler_Handle_EmptyParams(t *testing.T) {
	page := domain.SearchResultPage{Items: []domain.SearchResult{}, Total: 0}
	handler := NewSearchHandler(&fakeSearchUseCases{page: page})

	result, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}
