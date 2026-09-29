package conversation

import (
	"context"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/domain"
)

// recordingRuntime records executed actions and returns a listing for GET_LISTING.
type recordingRuntime struct {
	listing *domain.Listing
	actions []*domain.CommerceAction
}

func (r *recordingRuntime) Execute(ctx context.Context, action *domain.CommerceAction, actor domain.CommerceActor) (*commerce.ExecuteResult, error) {
	if domain.AIActionRequiresConfirmation(action.ActionType) && !action.IsConfirmed() {
		// Same contract as the real runtime's confirmation policy.
		return &commerce.ExecuteResult{ActionID: action.ActionID, Status: domain.ActionResultRequiresConfirmation,
			RequiresConfirmation: true, MessageKey: "Please confirm", Data: map[string]interface{}{}}, nil
	}
	r.actions = append(r.actions, action)
	res := &commerce.ExecuteResult{ActionID: action.ActionID, Status: domain.ActionResultSuccess, MessageKey: "ACTION_SUCCESS", Data: map[string]interface{}{}}
	if action.ActionType == domain.ActionGetListing {
		res.Data = map[string]interface{}{"data": r.listing}
	}
	return res, nil
}

func (r *recordingRuntime) IsSupported(domain.CommerceActionType) bool { return true }

func (r *recordingRuntime) last() *domain.CommerceAction {
	if len(r.actions) == 0 {
		return nil
	}
	return r.actions[len(r.actions)-1]
}

// seqClassifier opens the listing on the first message and is unsure
// afterwards, so later messages go through keyword routing and context.
type seqClassifier struct {
	calls     int
	listingID string
}

func (c *seqClassifier) ClassifyIntent(ctx context.Context, req *IntentClassificationRequest) (*IntentClassificationResult, error) {
	c.calls++
	if c.calls == 1 {
		intent := domain.NewAIIntent()
		intent.Intent = domain.ActionGetListing
		intent.Confidence = 0.95
		intent.Parameters = map[string]interface{}{"listing_id": c.listingID}
		return &IntentClassificationResult{Intent: intent}, nil
	}
	return &IntentClassificationResult{Fallback: domain.NewConversationResponse("unsure", "en")}, nil
}

func newContextualProcessor(rt *recordingRuntime, listingID string) *ConversationProcessor {
	return NewConversationProcessor(ProcessorConfig{
		Logger:            zerolog.Nop(),
		ConversationStore: &mockConvStore{conversations: make(map[string]*domain.Conversation)},
		ContextStore:      &mockContextStore{contexts: make(map[string]domain.ConversationContext)},
		MessageStore:      &mockMessageStore{},
		IntentClassifier:  &seqClassifier{listingID: listingID},
		ActionResolver:    NewActionResolver(NewDefaultIntentValidator()),
		Runtime:           rt,
		PackResolver:      newMockPackResolver(),
	})
}

func say(t *testing.T, p *ConversationProcessor, convID, text string) *ProcessResult {
	t.Helper()
	actor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	actor.OrgID = "org-1"
	sender := "sender-1"
	if convID != "" {
		sender = convID
	}
	res, err := p.ProcessMessage(context.Background(), &ProcessRequest{
		Channel: domain.ChannelWeb, SenderID: sender, SenderExternalID: "ext-1",
		MessageText: text, TenantID: "org-1", Actor: actor,
	})
	if err != nil {
		t.Fatalf("%q: %v", text, err)
	}
	return res
}

func TestContextualFlow_PauseThisListing(t *testing.T) {
	rt := &recordingRuntime{listing: &domain.Listing{ID: "lst-42", Title: "Sea-view flat", PriceAmount: 80, PriceCurrency: "USD"}}
	p := newContextualProcessor(rt, "lst-42")

	opened := say(t, p, "", "show me the sea-view flat")
	if !strings.Contains(opened.Response.Text, "Sea-view flat") || !strings.Contains(opened.Response.Text, "book it") {
		t.Fatalf("listing reply = %q", opened.Response.Text)
	}

	say(t, p, opened.ConversationID, "pause this listing")
	a := rt.last()
	if a.ActionType != domain.ActionPauseListing || a.Parameters["listing_id"] != "lst-42" {
		t.Fatalf("pause action = %s %v", a.ActionType, a.Parameters)
	}
}

func TestContextualFlow_BookItNeedsConfirmation(t *testing.T) {
	rt := &recordingRuntime{listing: &domain.Listing{ID: "lst-7", Title: "Studio", PriceAmount: 40, PriceCurrency: "USD"}}
	p := newContextualProcessor(rt, "lst-7")

	opened := say(t, p, "", "open the studio")
	before := len(rt.actions)
	res := say(t, p, opened.ConversationID, "buy it")
	if !res.IsConfirmation {
		t.Fatalf("an order must be confirmed first, got %q", res.Response.Text)
	}
	if len(rt.actions) != before {
		t.Fatal("nothing may execute before confirmation")
	}
	say(t, p, opened.ConversationID, "yes")
	a := rt.last()
	if a == nil || a.ActionType != domain.ActionCreateOrder || a.Parameters["listing_id"] != "lst-7" {
		t.Fatalf("confirmed action = %+v", a)
	}
}
