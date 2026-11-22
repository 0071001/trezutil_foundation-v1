package conversation

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/trezutil/trezutil/internal/domain"
)

// StreamSink receives what happens while a message is processed, at the moment it happens: the stages of the work
// (interpreting the request, running the action, writing the answer) and the text of the answer as the model writes
// it. It is how POST /api/v1/conversations/message/stream shows real progress; nothing here is simulated.
type StreamSink interface {
	// Stage reports a step of the work: "interpreting", "acting" (detail: the action type) or "writing".
	Stage(stage, detail string)
	// Delta is a piece of the answer's text, in order, as it is generated.
	Delta(text string)
	// Reset discards the text sent so far: the written answer was not kept and the reply's text replaces it.
	Reset()
}

type streamSinkKey struct{}

// WithStreamSink attaches a sink to the processing of one message.
func WithStreamSink(ctx context.Context, sink StreamSink) context.Context {
	return context.WithValue(ctx, streamSinkKey{}, sink)
}

// SinkFrom returns the sink attached to ctx, or one that drops everything.
func SinkFrom(ctx context.Context) StreamSink {
	if s, ok := ctx.Value(streamSinkKey{}).(StreamSink); ok && s != nil {
		return s
	}
	return nopSink{}
}

type nopSink struct{}

func (nopSink) Stage(string, string) {}
func (nopSink) Delta(string)         {}
func (nopSink) Reset()               {}

// ReplyWriter has the model write the answer the person reads, from the facts the server already established (the
// reply drafted from the action's result, the cards and results). It streams the text through onText as it is
// generated and returns the whole text. An error means nothing usable was written: the drafted reply is kept.
type ReplyWriter interface {
	WriteReply(ctx context.Context, in ReplyWritingInput, onText func(string)) (string, error)
}

// ReplyWritingInput is everything the writer may use. Nothing else: the model is never given anything it could
// present as a fact that the server did not establish.
type ReplyWritingInput struct {
	OrganizationID  string
	Channel         domain.Channel
	Actor           domain.CommerceActor
	Language        string
	CustomerMessage string
	ActionType      string
	// Draft is the reply the server built from the action's result: correct, complete, possibly stiff.
	Draft string
	// Facts is a compact JSON of the cards and result items the answer shows.
	Facts string
}

// writableActions are the answers the model may word: they only report what was found. A confirmation (an order, a
// booking, a payment about to happen), a question for a missing detail, or an error keeps the server's exact words.
var writableActions = map[string]bool{"SEARCH_LISTINGS": true, "GET_LISTING": true}

// writingEligible reports whether the model may word this reply.
func writingEligible(result *ProcessResult) bool {
	if result == nil || result.Response == nil || strings.TrimSpace(result.Response.Text) == "" {
		return false
	}
	if result.IsConfirmation || result.IsClarification || result.Response.Confirmation != nil || len(result.MissingParams) > 0 {
		return false
	}
	actionType, _ := result.Response.Metadata["action_type"].(string)
	if !writableActions[actionType] {
		return false
	}
	if dup, _ := result.Response.Metadata["duplicate"].(bool); dup {
		return false
	}
	return true
}

// replyFacts is what the answer shows, for the writer: card titles and descriptions, and the main fields of result
// items, within a few kilobytes.
func replyFacts(resp *domain.ConversationResponse) string {
	type item struct {
		Title    string      `json:"title,omitempty"`
		Detail   string      `json:"detail,omitempty"`
		Price    interface{} `json:"price,omitempty"`
		Currency interface{} `json:"currency,omitempty"`
		Place    interface{} `json:"place,omitempty"`
	}
	out := struct {
		Cards   []item `json:"cards,omitempty"`
		Results []item `json:"results,omitempty"`
		Total   any    `json:"total,omitempty"`
	}{}
	for i, c := range resp.Cards {
		if i == 10 {
			break
		}
		out.Cards = append(out.Cards, item{Title: c.Title, Detail: c.Description})
	}
	if data, ok := resp.Metadata["result_data"].(map[string]interface{}); ok {
		out.Total = data["total"]
		if items, ok := data["items"].([]interface{}); ok {
			for i, raw := range items {
				if i == 10 {
					break
				}
				m, _ := raw.(map[string]interface{})
				out.Results = append(out.Results, item{
					Title: firstString(m, "title", "Title", "name"), Detail: firstString(m, "description", "Description"),
					Price: first(m, "price_amount", "PriceAmount", "price"), Currency: first(m, "price_currency", "PriceCurrency", "currency"),
					Place: first(m, "city", "location", "Location", "address"),
				})
			}
		}
	} else if data, ok := resp.Metadata["result_data"]; ok && data != nil {
		if raw, err := json.Marshal(data); err == nil && len(raw) < 2500 {
			return string(raw)
		}
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return "{}"
	}
	if len(raw) > 4000 {
		raw = raw[:4000]
	}
	return string(raw)
}

func first(m map[string]interface{}, keys ...string) interface{} {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil && v != "" {
			return v
		}
	}
	return nil
}

func firstString(m map[string]interface{}, keys ...string) string {
	if s, ok := first(m, keys...).(string); ok {
		r := []rune(s)
		if len(r) > 200 {
			return string(r[:200])
		}
		return s
	}
	return ""
}

var numberPattern = regexp.MustCompile(`\d+(?:[.,\s\x{202F}\x{00A0}]\d+)*`)

// numbersAreGrounded reports whether every number in the written text appears in what the writer was given (the
// draft, the facts, the person's message). A written answer that states a number nobody established — a price, a
// quantity, a delay — is not kept: the drafted reply is shown instead. Formatting is ignored ("1 500", "1500").
func numbersAreGrounded(written string, sources ...string) bool {
	allowed := map[string]bool{}
	for _, s := range sources {
		for _, n := range numberPattern.FindAllString(s, -1) {
			for _, v := range numberVariants(n) {
				allowed[v] = true
			}
		}
	}
	for _, n := range numberPattern.FindAllString(written, -1) {
		ok := false
		for _, v := range numberVariants(n) {
			if allowed[v] {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// numberVariants is a number written several ways: digits only, and with the decimal part dropped when it is zero.
func numberVariants(n string) []string {
	clean := strings.Map(func(r rune) rune {
		if r == ' ' || r == ' ' || r == ' ' {
			return -1
		}
		return r
	}, n)
	digits := strings.NewReplacer(",", "", ".", "").Replace(clean)
	out := []string{digits, clean}
	for _, sep := range []string{".", ","} {
		if i := strings.LastIndex(clean, sep); i > 0 {
			whole, frac := clean[:i], clean[i+1:]
			if strings.Trim(frac, "0") == "" {
				out = append(out, strings.NewReplacer(",", "", ".", "").Replace(whole))
			}
		}
	}
	return out
}
