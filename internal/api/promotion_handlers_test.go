package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

type fakePromoter struct {
	setErr, endErr error
	setCalls       []float64
	actor, listing string
	ended          int
}

func (f *fakePromoter) SetListingPromotion(_ context.Context, actorID, listingID string, salePrice float64) (*domain.Listing, error) {
	f.actor, f.listing = actorID, listingID
	f.setCalls = append(f.setCalls, salePrice)
	if f.setErr != nil {
		return nil, f.setErr
	}
	l := &domain.Listing{ID: listingID, PriceAmount: salePrice}
	return l, nil
}

func (f *fakePromoter) EndListingPromotion(_ context.Context, actorID, listingID string) (*domain.Listing, error) {
	f.actor, f.listing = actorID, listingID
	f.ended++
	if f.endErr != nil {
		return nil, f.endErr
	}
	return &domain.Listing{ID: listingID, PriceAmount: 200}, nil
}

func promoRequest(path, body, user string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if user != "" {
		req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: user}))
	}
	return req
}

func TestSetPromotionH_StartsASaleForTheSignedInUser(t *testing.T) {
	f := &fakePromoter{}
	w := httptest.NewRecorder()
	SetPromotionH(f).ServeHTTP(w, promoRequest("/v1/listings/promotion?listing_id=l-1", `{"sale_price": 79.5}`, "user-1"))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	if f.actor != "user-1" || f.listing != "l-1" || len(f.setCalls) != 1 || f.setCalls[0] != 79.5 {
		t.Fatalf("actor %q listing %q calls %v", f.actor, f.listing, f.setCalls)
	}
	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out["PriceAmount"] != 79.5 {
		t.Fatalf("response must return the updated listing: %s", w.Body)
	}
}

func TestPromotionHandlersRequireSignIn(t *testing.T) {
	f := &fakePromoter{}
	for name, h := range map[string]http.HandlerFunc{"set": SetPromotionH(f), "end": EndPromotionH(f)} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, promoRequest("/v1/listings/promotion?listing_id=l-1", `{"sale_price": 10}`, ""))
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s: status %d, want 401", name, w.Code)
		}
	}
	if len(f.setCalls) != 0 || f.ended != 0 {
		t.Fatal("an anonymous caller must never reach the use case")
	}
}

func TestSetPromotionH_RequiresASalePrice(t *testing.T) {
	for _, body := range []string{``, `{}`, `{"sale_price": null}`, `not json`} {
		f := &fakePromoter{}
		w := httptest.NewRecorder()
		SetPromotionH(f).ServeHTTP(w, promoRequest("/v1/listings/promotion?listing_id=l-1", body, "user-1"))
		if w.Code != http.StatusBadRequest || len(f.setCalls) != 0 {
			t.Errorf("body %q: status %d calls %v, want 400 and no call", body, w.Code, f.setCalls)
		}
	}
}

func TestPromotionHandlers_MapFailuresToStatusAndCode(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrListingNotFound, http.StatusNotFound, "LISTING_NOT_FOUND"},
		{domain.ErrInsufficientPermissions, http.StatusForbidden, "FORBIDDEN"},
		{domain.ErrInvalidStatusTransition, http.StatusConflict, "LISTING_NOT_EDITABLE"},
		{domain.ErrInvalidParameter, http.StatusBadRequest, "INVALID_PARAMETER"},
		{context.DeadlineExceeded, http.StatusInternalServerError, "PROMOTION_FAILED"},
	}
	for _, c := range cases {
		for name, run := range map[string]func(*fakePromoter) *httptest.ResponseRecorder{
			"set": func(f *fakePromoter) *httptest.ResponseRecorder {
				f.setErr = c.err
				w := httptest.NewRecorder()
				SetPromotionH(f).ServeHTTP(w, promoRequest("/x?listing_id=l-1", `{"sale_price": 5}`, "u"))
				return w
			},
			"end": func(f *fakePromoter) *httptest.ResponseRecorder {
				f.endErr = c.err
				w := httptest.NewRecorder()
				EndPromotionH(f).ServeHTTP(w, promoRequest("/x?listing_id=l-1", ``, "u"))
				return w
			},
		} {
			w := run(&fakePromoter{})
			var out ErrorResponse
			_ = json.Unmarshal(w.Body.Bytes(), &out)
			if w.Code != c.status || out.Error.Code != c.code {
				t.Errorf("%s/%v: %d %q, want %d %q", name, c.err, w.Code, out.Error.Code, c.status, c.code)
			}
			if strings.Contains(w.Body.String(), "deadline") {
				t.Errorf("%s: internal error text leaked to the client: %s", name, w.Body)
			}
		}
	}
}

func TestSearchH_OnPromotionAndDiscountSort(t *testing.T) {
	s := &capturingSearch{}
	w := postSearch(t, s, `{"on_promotion": true, "sort_field": "discount", "vertical": "retail"}`)
	if w.Code != http.StatusOK || s.got == nil {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	if !s.got.OnPromotion || s.got.Sort == nil || s.got.Sort.Field != domain.SearchSortFieldDiscount || s.got.Vertical != domain.VerticalRetail {
		t.Fatalf("query = %+v", s.got)
	}

	// Not asked for: ordinary search is unchanged.
	s = &capturingSearch{}
	postSearch(t, s, `{"text": "phone"}`)
	if s.got == nil || s.got.OnPromotion {
		t.Fatalf("ordinary search must not be restricted to promotions: %+v", s.got)
	}
}
