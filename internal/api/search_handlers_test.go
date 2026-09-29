package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

type mockSearchUseCases struct{}

func (m *mockSearchUseCases) ExecuteSearch(ctx context.Context, actorID string, query domain.DiscoveryQuery) (domain.SearchResultPage, error) {
	return domain.SearchResultPage{
		Items: []domain.SearchResult{
			{
				ListingID:     "1",
				Title:         "Test",
				PriceAmount:   100.0,
				PriceCurrency: "USD",
				Score:         1.0,
			},
		},
		Total:   1,
		HasNext: false,
	}, nil
}

func (m *mockSearchUseCases) Suggest(ctx context.Context, actorID string, req domain.SuggestRequest) (domain.SuggestResponse, error) {
	return domain.SuggestResponse{
		Suggestions: []domain.Suggestion{
			{
				Text: "test suggestion",
				Type: "query",
			},
		},
	}, nil
}

func TestSearchHandler_SearchH(t *testing.T) {
	handler := NewSearchHandler(&mockSearchUseCases{})

	body := `{"text":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/search?user_id=user1", strings.NewReader(body))
	req = asQueryUser(req)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.SearchH(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response SearchResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(response.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(response.Items))
	}
}

func TestSearchHandler_SuggestH(t *testing.T) {
	handler := NewSearchHandler(&mockSearchUseCases{})

	req := httptest.NewRequest(http.MethodGet, "/v1/search/suggest?user_id=user1&text=test", nil)
	req = asQueryUser(req)
	w := httptest.NewRecorder()

	handler.SuggestH(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var response SuggestResponseDTO
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if len(response.Suggestions) != 1 {
		t.Errorf("expected 1 suggestion, got %d", len(response.Suggestions))
	}
}

func TestSearchHandler_SearchH_GuestAllowed(t *testing.T) {
	// Discovery is public: guests may search published listings.
	handler := NewSearchHandler(&mockSearchUseCases{})

	req := httptest.NewRequest(http.MethodPost, "/v1/search", strings.NewReader(`{"text":"hotel"}`))
	w := httptest.NewRecorder()

	handler.SearchH(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d for guest search, got %d", http.StatusOK, w.Code)
	}
}

func TestSearchHandler_SuggestH_MissingText(t *testing.T) {
	handler := NewSearchHandler(&mockSearchUseCases{})

	req := httptest.NewRequest(http.MethodGet, "/v1/search/suggest?user_id=user1", nil)
	req = asQueryUser(req)
	w := httptest.NewRecorder()

	handler.SuggestH(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}
