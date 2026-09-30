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

// capturingSearch records the query the handler built.
type capturingSearch struct {
	mockSearchUseCases
	got   *domain.DiscoveryQuery
	calls int
}

func (c *capturingSearch) ExecuteSearch(ctx context.Context, actorID string, q domain.DiscoveryQuery) (domain.SearchResultPage, error) {
	c.calls++
	c.got = &q
	return domain.SearchResultPage{Items: []domain.SearchResult{{ListingID: "room-1", Title: "Deluxe room", Vertical: "HOTEL"}}, Total: 1}, nil
}

func postSearch(t *testing.T, s SearchUseCases, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/search", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	NewSearchHandler(s).SearchH(w, req)
	return w
}

func TestSearchH_VerticalAndLocation(t *testing.T) {
	s := &capturingSearch{}
	w := postSearch(t, s, `{"vertical":"hotel","location":" Gombe ","price_min":20,"price_max":150}`)
	if w.Code != http.StatusOK || s.got == nil {
		t.Fatalf("status %d", w.Code)
	}
	if s.got.Vertical != domain.VerticalHotel || s.got.Place != "Gombe" ||
		s.got.PriceRange == nil || *s.got.PriceRange.Min != 20 || *s.got.PriceRange.Max != 150 {
		t.Fatalf("query = %+v", s.got)
	}
	var resp map[string][]map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp["items"]) != 1 || resp["items"][0]["Vertical"] != "HOTEL" {
		t.Fatalf("results must carry the vertical: %s", w.Body.String())
	}
}

func TestSearchH_RejectsInvalidFilters(t *testing.T) {
	for body, code := range map[string]string{
		`{"vertical":"SPACESHIPS"}`:                       "INVALID_VERTICAL",
		`{"vertical":"hotel rooms"}`:                      "INVALID_VERTICAL",
		`{"location":"` + strings.Repeat("x", 101) + `"}`: "INVALID_LOCATION",
		`{"price_min":-1}`:                                "INVALID_PRICE_RANGE",
		`{"price_min":200,"price_max":100}`:               "INVALID_PRICE_RANGE",
	} {
		s := &capturingSearch{}
		w := postSearch(t, s, body)
		var resp ErrorResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if w.Code != http.StatusBadRequest || resp.Error.Code != code || s.calls != 0 {
			t.Errorf("%s: got %d %s (search called %d times), want 400 %s", body, w.Code, resp.Error.Code, s.calls, code)
		}
	}
	// Without filters the contract is unchanged.
	s := &capturingSearch{}
	if w := postSearch(t, s, `{"text":"phone"}`); w.Code != http.StatusOK || s.got.Vertical != "" || s.got.Place != "" || s.got.PriceRange != nil {
		t.Fatalf("plain text search changed: %d %+v", w.Code, s.got)
	}
}
