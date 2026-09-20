package commerce

import (
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestHandlerRegistry_New(t *testing.T) {
	r := NewHandlerRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
	if len(r.handlers) != 0 {
		t.Error("expected empty handlers map")
	}
}

func TestHandlerRegistry_RegisterAndGet(t *testing.T) {
	r := NewHandlerRegistry()
	handler := &fakeHandler{actionType: domain.ActionSearchListings}

	r.Register(handler)

	h, ok := r.Get(domain.ActionSearchListings)
	if !ok {
		t.Fatal("expected handler to be found")
	}
	if h.ActionType() != domain.ActionSearchListings {
		t.Errorf("expected %s, got %s", domain.ActionSearchListings, h.ActionType())
	}
}

func TestHandlerRegistry_GetNotFound(t *testing.T) {
	r := NewHandlerRegistry()
	h, ok := r.Get(domain.ActionCreateOrder)
	if ok {
		t.Error("expected handler not found")
	}
	if h != nil {
		t.Error("expected nil handler")
	}
}

func TestHandlerRegistry_Has(t *testing.T) {
	r := NewHandlerRegistry()
	r.Register(&fakeHandler{actionType: domain.ActionSearchListings})

	if !r.Has(domain.ActionSearchListings) {
		t.Error("expected Has to return true")
	}
	if r.Has(domain.ActionGetListing) {
		t.Error("expected Has to return false")
	}
}

func TestHandlerRegistry_Overwrite(t *testing.T) {
	r := NewHandlerRegistry()
	r.Register(&fakeHandler{actionType: domain.ActionSearchListings})
	r.Register(&fakeHandler{actionType: domain.ActionSearchListings})

	h, ok := r.Get(domain.ActionSearchListings)
	if !ok {
		t.Fatal("expected handler after overwrite")
	}
	if h.ActionType() != domain.ActionSearchListings {
		t.Errorf("expected %s, got %s", domain.ActionSearchListings, h.ActionType())
	}
}

func TestHandlerRegistry_MultipleHandlers(t *testing.T) {
	r := NewHandlerRegistry()
	r.Register(&fakeHandler{actionType: domain.ActionSearchListings})
	r.Register(&fakeHandler{actionType: domain.ActionGetListing})
	r.Register(&fakeHandler{actionType: domain.ActionGetOrder})

	for _, at := range []domain.CommerceActionType{
		domain.ActionSearchListings,
		domain.ActionGetListing,
		domain.ActionGetOrder,
	} {
		h, ok := r.Get(at)
		if !ok {
			t.Fatalf("expected handler for %s", at)
		}
		if h.ActionType() != at {
			t.Errorf("expected %s, got %s", at, h.ActionType())
		}
	}
}
