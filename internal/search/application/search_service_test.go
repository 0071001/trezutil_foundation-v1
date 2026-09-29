package application

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestSearchService_ExecuteSearch(t *testing.T) {
	engine := NewInMemorySearchEngine()
	engine.AddListing("test", domain.SearchResult{
		ListingID:     "1",
		Title:         "Test Listing",
		PriceAmount:   100.0,
		PriceCurrency: "USD",
		Score:         1.0,
	})

	service := NewSearchService(
		engine,
		NewNoopVisibilityResolver(),
		NewAllowAllAuthorizationResolver(),
		nil,
		NewNoopCursorCodec(),
		NewSystemClock(),
	)

	result, err := service.ExecuteSearch(context.Background(), "user1", domain.DiscoveryQuery{
		Text: "test",
		Pagination: domain.PaginationCursor{
			Mode:     domain.PaginationModeCursor,
			PageSize: 10,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Items))
	}

	if result.Items[0].ListingID != "1" {
		t.Errorf("expected listing ID '1', got '%s'", result.Items[0].ListingID)
	}
}

func TestSearchService_Suggest(t *testing.T) {
	service := NewSearchService(
		NewInMemorySearchEngine(),
		NewNoopVisibilityResolver(),
		NewAllowAllAuthorizationResolver(),
		nil,
		NewNoopCursorCodec(),
		NewSystemClock(),
	)

	result, err := service.Suggest(context.Background(), "user1", domain.SuggestRequest{
		Text:  "test",
		Limit: 10,
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Suggestions) != 0 {
		t.Errorf("expected 0 suggestions, got %d", len(result.Suggestions))
	}
}

func TestInMemorySearchEngine_ExecuteSearch(t *testing.T) {
	engine := NewInMemorySearchEngine()
	engine.AddListing("phone", domain.SearchResult{
		ListingID:     "1",
		Title:         "Phone",
		PriceAmount:   500.0,
		PriceCurrency: "USD",
		Score:         1.0,
	})
	engine.AddListing("phone", domain.SearchResult{
		ListingID:     "2",
		Title:         "Smartphone",
		PriceAmount:   800.0,
		PriceCurrency: "USD",
		Score:         0.8,
	})

	result, err := engine.ExecuteSearch(context.Background(), domain.DiscoveryQuery{
		Text: "phone",
		Pagination: domain.PaginationCursor{
			Mode:     domain.PaginationModeCursor,
			PageSize: 10,
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("expected total 2, got %d", result.Total)
	}

	if len(result.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(result.Items))
	}
}

func TestNoopCursorCodec_EncodeDecode(t *testing.T) {
	codec := NewNoopCursorCodec()
	cursor := domain.PaginationCursor{
		Mode:     domain.PaginationModeCursor,
		Cursor:   "test-cursor",
		PageSize: 10,
	}

	encoded, err := codec.Encode(cursor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	decoded, err := codec.Decode(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decoded.Cursor != cursor.Cursor {
		t.Errorf("expected cursor '%s', got '%s'", cursor.Cursor, decoded.Cursor)
	}

	if decoded.PageSize != cursor.PageSize {
		t.Errorf("expected page size %d, got %d", cursor.PageSize, decoded.PageSize)
	}
}

func TestSystemClock_Now(t *testing.T) {
	clock := NewSystemClock()
	now := clock.Now()

	if now <= 0 {
		t.Error("expected positive timestamp")
	}
}

// recordingEngine captures the query passed to the engine.
type recordingEngine struct{ last domain.DiscoveryQuery }

func (e *recordingEngine) ExecuteSearch(ctx context.Context, q domain.DiscoveryQuery) (domain.SearchResultPage, error) {
	e.last = q
	return domain.SearchResultPage{}, nil
}

// Regression: the searching user's ID used to be injected as a seller_id
// filter, so public searches returned nothing for real users.
func TestSearchService_DoesNotScopeToSearchingUser(t *testing.T) {
	engine := &recordingEngine{}
	service := NewSearchService(engine, NewNoopVisibilityResolver(), NewAllowAllAuthorizationResolver(), nil, NewNoopCursorCodec(), NewSystemClock())

	if _, err := service.ExecuteSearch(context.Background(), "user-42", domain.DiscoveryQuery{Text: "hotel"}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if engine.last.SellerID != "" || engine.last.StoreID != "" {
		t.Fatalf("search must not be scoped to the caller, got seller=%q store=%q", engine.last.SellerID, engine.last.StoreID)
	}

	if _, err := service.ExecuteSearch(context.Background(), "", domain.DiscoveryQuery{Text: "hotel", SellerID: "seller-9"}); err != nil {
		t.Fatalf("search: %v", err)
	}
	if engine.last.SellerID != "seller-9" {
		t.Fatalf("explicit seller filter must be preserved, got %q", engine.last.SellerID)
	}
}
