package conversation

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestReplyRecordKeepsCardsAndResultsButNeverAConfirmation(t *testing.T) {
	resp := &domain.ConversationResponse{
		Text:  "J’ai trouvé 1 offre.",
		Cards: []domain.ConversationCard{{ID: "lst-1", Type: "listing", Title: "Dell Latitude", Data: map[string]interface{}{"position": 1}}},
		Confirmation: &domain.ConversationConfirmation{Summary: "Commander ?"},
		Metadata: map[string]interface{}{
			"action_type": "SEARCH_LISTINGS",
			"result_data": map[string]interface{}{"items": []interface{}{map[string]interface{}{"id": "lst-1"}}},
			"suggestions": []string{"Moins cher"},
			"action_id":   "internal",
		},
	}
	data := replyRecord(resp)
	raw, _ := json.Marshal(data)
	if strings.Contains(string(raw), "Commander") || strings.Contains(string(raw), "internal") {
		t.Fatalf("a confirmation or an unlisted key was stored: %s", raw)
	}
	got := StoredReply(data)
	if got == nil || len(got["cards"].([]interface{})) != 1 {
		t.Fatalf("cards not kept: %v", got)
	}
	meta := got["metadata"].(map[string]interface{})
	if meta["action_type"] != "SEARCH_LISTINGS" || meta["result_data"] == nil || meta["suggestions"] == nil {
		t.Fatalf("metadata not kept: %v", meta)
	}
}

func TestReplyRecordIsEmptyForATextOnlyAnswer(t *testing.T) {
	if data := replyRecord(&domain.ConversationResponse{Text: "Bonjour"}); data != nil {
		t.Fatalf("want nil, got %v", data)
	}
	if StoredReply(map[string]interface{}{"locale": "fr"}) != nil {
		t.Fatal("data without a record must read back as nil")
	}
}
