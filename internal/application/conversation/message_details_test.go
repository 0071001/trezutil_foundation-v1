package conversation

import (
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestABookingTakesTheNumberedOfferAndTheDatesFromTheMessage(t *testing.T) {
	cx := &domain.ConversationContext{ResultItems: []domain.ResultItemRef{{ID: "l1", RefType: "listing"}, {ID: "l2", RefType: "listing"}}}
	action := &domain.CommerceAction{ActionType: domain.ActionCreateReservation}
	completeFromMessage(action, "Réserve le numéro 2 du 2026-12-15 14:00 au 2026-12-17 11:00", cx)
	p := action.Parameters
	if p["listing_id"] != "l2" || p["start_at"] != "2026-12-15 14:00" || p["end_at"] != "2026-12-17 11:00" {
		t.Fatalf("parameters %+v", p)
	}
}

func TestWhatTheInterpreterGaveIsKept(t *testing.T) {
	cx := &domain.ConversationContext{ResultItems: []domain.ResultItemRef{{ID: "l1", RefType: "listing"}, {ID: "l2", RefType: "listing"}}}
	action := &domain.CommerceAction{ActionType: domain.ActionCreateReservation, Parameters: map[string]interface{}{"listing_id": "l1", "start_at": "2026-12-01 14:00"}}
	completeFromMessage(action, "Réserve le numéro 2 du 2026-12-15 14:00 au 2026-12-17 11:00", cx)
	if action.Parameters["listing_id"] != "l1" || action.Parameters["start_at"] != "2026-12-01 14:00" || action.Parameters["end_at"] != nil {
		t.Fatalf("parameters %+v", action.Parameters)
	}
}

func TestNothingIsGuessed(t *testing.T) {
	cx := &domain.ConversationContext{ResultItems: []domain.ResultItemRef{{ID: "l1", RefType: "listing"}, {ID: "l2", RefType: "listing"}}}
	action := &domain.CommerceAction{ActionType: domain.ActionCreateReservation}
	// No offer pointed at, dates the wrong way round.
	completeFromMessage(action, "Réserve du 2026-12-17 14:00 au 2026-12-15 11:00", cx)
	if len(action.Parameters) != 0 {
		t.Fatalf("parameters %+v", action.Parameters)
	}
	search := &domain.CommerceAction{ActionType: domain.ActionSearchListings}
	completeFromMessage(search, "numéro 2", cx)
	if len(search.Parameters) != 0 {
		t.Fatalf("a search takes no listing: %+v", search.Parameters)
	}
}
