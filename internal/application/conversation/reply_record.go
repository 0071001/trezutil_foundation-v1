package conversation

import (
	"encoding/json"

	"github.com/trezutil/trezutil/internal/domain"
)

// ReplyRecordKey is the key, in a stored answer's MessageContent.Data, of what the answer showed besides its text:
// its cards and the parts of its metadata the interface draws (results, suggestions, the action it reports). Kept so
// a conversation read back later (GET /conversations/{id}/messages, another device) shows its offers again.
//
// A confirmation is never kept: its token is a one-time secret, and a confirmation read back later has expired anyway.
const ReplyRecordKey = "reply"

// replyMetadataKept are the only metadata keys stored with an answer.
var replyMetadataKept = []string{"action_type", "source", "result_data", "suggestions"}

// replyRecord builds the Data of a stored answer, or nil when the answer showed nothing besides its text. The value
// is a plain JSON tree (maps, slices, strings, numbers), the shape the message store writes and reads back.
func replyRecord(resp *domain.ConversationResponse) map[string]interface{} {
	if resp == nil {
		return nil
	}
	record := map[string]interface{}{}
	if len(resp.Cards) > 0 {
		record["cards"] = resp.Cards
	}
	meta := map[string]interface{}{}
	for _, k := range replyMetadataKept {
		if v, ok := resp.Metadata[k]; ok && v != nil {
			meta[k] = v
		}
	}
	if len(meta) > 0 {
		record["metadata"] = meta
	}
	if len(record) == 0 {
		return nil
	}
	plain, ok := toJSONTree(record)
	if !ok {
		return nil
	}
	return map[string]interface{}{ReplyRecordKey: plain}
}

// StoredReply returns what a stored answer showed besides its text, shaped like the "response" of a live answer
// ({"cards": [...], "metadata": {...}}), or nil.
func StoredReply(data map[string]interface{}) map[string]interface{} {
	record, _ := data[ReplyRecordKey].(map[string]interface{})
	if len(record) == 0 {
		return nil
	}
	out := map[string]interface{}{}
	if cards, ok := record["cards"].([]interface{}); ok && len(cards) > 0 {
		out["cards"] = cards
	}
	if meta, ok := record["metadata"].(map[string]interface{}); ok {
		kept := map[string]interface{}{}
		for _, k := range replyMetadataKept {
			if v, ok := meta[k]; ok && v != nil {
				kept[k] = v
			}
		}
		if len(kept) > 0 {
			out["metadata"] = kept
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func toJSONTree(v interface{}) (map[string]interface{}, bool) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, false
	}
	return out, true
}
