package conversation

import (
	"regexp"
	"strings"

	"github.com/trezutil/trezutil/internal/domain"
)

// A date and time written in full, as the web app's date pickers send them: "2026-12-01 14:00".
var fullDateTime = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}\b`)

// completeFromMessage adds what the message states plainly and the interpreter left out: the listing it points at in
// the last results ("Réserve le numéro 1") and, for a booking written with both dates, its start and end. It never
// replaces a value the interpreter gave.
func completeFromMessage(action *domain.CommerceAction, text string, cx *domain.ConversationContext) {
	if action == nil || !listingScopedActions[action.ActionType] {
		return
	}
	if action.Parameters == nil {
		action.Parameters = map[string]interface{}{}
	}
	missing := func(name string) bool {
		v, ok := action.Parameters[name]
		return !ok || v == nil || v == ""
	}
	if missing("listing_id") {
		// The words after a colon are the offer's own title.
		ask := text
		if i := strings.Index(ask, " : "); i >= 0 {
			ask = ask[:i]
		}
		if id := referencedResultID(ask, cx); id != "" && isListingResult(cx, id) {
			action.Parameters["listing_id"] = id
		}
	}
	if action.ActionType != domain.ActionCreateReservation && action.ActionType != domain.ActionCheckAvailability {
		return
	}
	if dates := fullDateTime.FindAllString(text, -1); len(dates) == 2 && dates[0] < dates[1] && missing("start_at") && missing("end_at") {
		action.Parameters["start_at"], action.Parameters["end_at"] = dates[0], dates[1]
	}
}

func isListingResult(cx *domain.ConversationContext, id string) bool {
	for _, item := range cx.ResultItems {
		if item.ID == id {
			return item.RefType == "listing"
		}
	}
	return false
}
