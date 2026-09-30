package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

// createListingUseCases returns err from CreateListing, or sets the vertical the
// catalogue would have resolved.
type createListingUseCases struct {
	mockListingUseCases
	err      error
	resolved domain.Vertical
}

func (m *createListingUseCases) CreateListing(ctx context.Context, actorID string, listing *domain.Listing, attributes []*domain.AttributeValue, media []*domain.ListingMedia, availability *domain.ListingAvailability, locations []*domain.ListingLocation) error {
	if m.err != nil {
		return m.err
	}
	listing.Vertical = m.resolved
	return nil
}

func postListing(t *testing.T, uc ListingUseCases, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := asQueryUser(httptest.NewRequest(http.MethodPost, "/v1/listings?user_id=user-123", bytes.NewReader([]byte(body))))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	NewListingHandler(uc).CreateListingH(w, req)
	return w
}

func TestCreateListing_VerticalErrorsAreClientErrors(t *testing.T) {
	body := `{"listing":{"StoreID":"s","SellerID":"x","CategoryID":"c","Title":"Room","ListingType":"real_estate","Vertical":"SPACESHIPS"}}`
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{fmt.Errorf("%w: unknown vertical %q", domain.ErrInvalidParameter, "SPACESHIPS"), http.StatusBadRequest, "INVALID_PARAMETER"},
		{domain.ErrInsufficientPermissions, http.StatusForbidden, "FORBIDDEN"},
		{errors.New("store not found"), http.StatusInternalServerError, "LISTING_CREATE_FAILED"},
	} {
		w := postListing(t, &createListingUseCases{err: tc.err}, body)
		var resp ErrorResponse
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if w.Code != tc.status || resp.Error.Code != tc.code {
			t.Errorf("%v: got %d %s, want %d %s", tc.err, w.Code, resp.Error.Code, tc.status, tc.code)
		}
	}
}

func TestCreateListing_ResponseCarriesResolvedVertical(t *testing.T) {
	w := postListing(t, &createListingUseCases{resolved: domain.VerticalHotel},
		`{"listing":{"StoreID":"s","SellerID":"x","CategoryID":"c","Title":"Room","ListingType":"real_estate","Vertical":"hotel"}}`)
	var created map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusCreated || created["Vertical"] != "HOTEL" {
		t.Fatalf("got %d %v", w.Code, created["Vertical"])
	}
}
