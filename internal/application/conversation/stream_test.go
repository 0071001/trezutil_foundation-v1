package conversation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

// recordingSink records what a stream would carry, in order.
type recordingSink struct{ events []string }

func (s *recordingSink) Stage(stage, detail string) { s.events = append(s.events, "stage:"+stage+":"+detail) }
func (s *recordingSink) Delta(text string)          { s.events = append(s.events, "delta:"+text) }
func (s *recordingSink) Reset()                     { s.events = append(s.events, "reset") }

// fakeWriter streams the given pieces, then returns their text (or err).
type fakeWriter struct {
	pieces []string
	err    error
	calls  int
	got    ReplyWritingInput
	before func(ctx context.Context)
}

func (w *fakeWriter) WriteReply(ctx context.Context, in ReplyWritingInput, onText func(string)) (string, error) {
	w.calls++
	w.got = in
	if w.before != nil {
		w.before(ctx)
	}
	for _, p := range w.pieces {
		onText(p)
	}
	if w.err != nil {
		return "", w.err
	}
	return strings.Join(w.pieces, ""), nil
}

// searchClassifier always reads a search.
type searchClassifier struct{}

func (searchClassifier) ClassifyIntent(context.Context, *IntentClassificationRequest) (*IntentClassificationResult, error) {
	intent := domain.NewAIIntent()
	intent.Intent = domain.ActionSearchListings
	intent.Confidence = 0.95
	intent.Parameters = map[string]interface{}{"query": "ordinateurs"}
	return &IntentClassificationResult{Intent: intent}, nil
}

// dedupMessages refuses a second message with the same idempotency key, like the PostgreSQL store.
type dedupMessages struct{ mockMessageStore }

func (d *dedupMessages) Create(ctx context.Context, msg *domain.Message) error {
	for _, m := range d.messages {
		if m.IdempotencyKey != "" && m.IdempotencyKey == msg.IdempotencyKey && m.ConversationID == msg.ConversationID {
			return domain.ErrDuplicateMessage
		}
	}
	return d.mockMessageStore.Create(ctx, msg)
}

func streamProcessor(writer ReplyWriter, msgs domain.MessageStore) *ConversationProcessor {
	return NewConversationProcessor(ProcessorConfig{
		Logger:            zerolog.Nop(),
		ConversationStore: &mockConvStore{conversations: make(map[string]*domain.Conversation)},
		ContextStore:      &mockContextStore{contexts: make(map[string]domain.ConversationContext)},
		MessageStore:      msgs,
		IntentClassifier:  searchClassifier{},
		ActionResolver:    NewActionResolver(NewDefaultIntentValidator()),
		Runtime:           &recordingRuntime{},
		PackResolver:      newMockPackResolver(),
		ReplyWriter:       writer,
	})
}

func streamSay(t *testing.T, ctx context.Context, p *ConversationProcessor, key string) *ProcessResult {
	t.Helper()
	actor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	res, err := p.ProcessMessage(ctx, &ProcessRequest{
		Channel: domain.ChannelWeb, SenderID: "sender-1", SenderExternalID: "ext-1", MessageText: "des ordinateurs portables",
		MessageIdempotencyKey: key, Locale: "fr", Actor: actor,
	})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func storedReply(msgs *mockMessageStore, key string) string {
	for _, m := range msgs.messages {
		if m.IdempotencyKey == "reply:"+key {
			return m.Content.Text
		}
	}
	return "<none>"
}

func TestStreamedAnswerIsWrittenStagedAndStored(t *testing.T) {
	msgs := &mockMessageStore{}
	writer := &fakeWriter{pieces: []string{"Voici ", "ce que ", "j’ai trouvé."}}
	sink := &recordingSink{}
	res := streamSay(t, WithStreamSink(context.Background(), sink), streamProcessor(writer, msgs), "k1")

	want := []string{"stage:interpreting:", "stage:acting:SEARCH_LISTINGS", "stage:writing:", "delta:Voici ", "delta:ce que ", "delta:j’ai trouvé."}
	if strings.Join(sink.events, "|") != strings.Join(want, "|") {
		t.Fatalf("events:\n got %v\nwant %v", sink.events, want)
	}
	if res.Response.Text != "Voici ce que j’ai trouvé." || storedReply(msgs, "k1") != res.Response.Text {
		t.Fatalf("the written answer must be the one returned and stored: %q / %q", res.Response.Text, storedReply(msgs, "k1"))
	}
	if writer.got.Draft == "" || writer.got.ActionType != "SEARCH_LISTINGS" || writer.got.CustomerMessage != "des ordinateurs portables" {
		t.Fatalf("the writer must get the draft and the facts: %+v", writer.got)
	}
}

func TestAnAnswerWithAnInventedNumberIsDropped(t *testing.T) {
	msgs := &mockMessageStore{}
	sink := &recordingSink{}
	p := streamProcessor(&fakeWriter{pieces: []string{"J’ai trouvé ", "37 offres à 120 USD."}}, msgs)
	res := streamSay(t, WithStreamSink(context.Background(), sink), p, "k2")
	if sink.events[len(sink.events)-1] != "reset" {
		t.Fatalf("text already shown must be withdrawn: %v", sink.events)
	}
	if strings.Contains(res.Response.Text, "37") || storedReply(msgs, "k2") != res.Response.Text {
		t.Fatalf("the draft must be kept and stored, got %q", res.Response.Text)
	}
}

func TestAFailedWritingKeepsTheDraft(t *testing.T) {
	msgs := &mockMessageStore{}
	sink := &recordingSink{}
	p := streamProcessor(&fakeWriter{pieces: []string{"Voici "}, err: errors.New("provider down")}, msgs)
	res := streamSay(t, WithStreamSink(context.Background(), sink), p, "k3")
	if sink.events[len(sink.events)-1] != "reset" || res.Response.Text == "Voici " || res.Response.Text == "" {
		t.Fatalf("a writer failure keeps the draft: events %v text %q", sink.events, res.Response.Text)
	}
}

func TestStoppingTheStreamStillStoresTheAnswer(t *testing.T) {
	msgs := &mockMessageStore{}
	ctx, cancel := context.WithCancel(context.Background())
	writer := &fakeWriter{pieces: []string{"Voi"}, before: func(context.Context) { cancel() }, err: context.Canceled}
	res := streamSay(t, ctx, streamProcessor(writer, msgs), "k4")
	if got := storedReply(msgs, "k4"); got == "<none>" || got != res.Response.Text {
		t.Fatalf("the answer must be stored even when the person stopped waiting: %q", got)
	}
}

func TestAConfirmationIsNeverWordedByTheModel(t *testing.T) {
	for _, result := range []*ProcessResult{
		{Response: &domain.ConversationResponse{Text: "Confirmer ?", Metadata: map[string]interface{}{"action_type": "CREATE_ORDER"}}, IsConfirmation: true},
		{Response: &domain.ConversationResponse{Text: "Combien ?", Metadata: map[string]interface{}{"action_type": "SEARCH_LISTINGS"}}, IsClarification: true},
		{Response: &domain.ConversationResponse{Text: "Commande créée", Metadata: map[string]interface{}{"action_type": "CREATE_ORDER"}}},
	} {
		if writingEligible(result) {
			t.Fatalf("must keep the server's words: %+v", result.Response)
		}
	}
	if !writingEligible(&ProcessResult{Response: &domain.ConversationResponse{Text: "2 offres", Metadata: map[string]interface{}{"action_type": "SEARCH_LISTINGS"}}}) {
		t.Fatal("a search answer may be worded")
	}
}

func TestARedeliveredMessageGetsTheStoredAnswerOrWaits(t *testing.T) {
	msgs := &dedupMessages{}
	p := streamProcessor(&fakeWriter{pieces: []string{"Voici ce que j’ai trouvé."}}, msgs)
	first := streamSay(t, context.Background(), p, "k5")

	again := streamSay(t, context.Background(), p, "k5")
	if again.Response.Text != first.Response.Text || again.Response.Metadata["duplicate"] != true || again.Response.Metadata["in_progress"] != nil {
		t.Fatalf("a redelivery gets the stored answer: %+v", again.Response)
	}

	// The question is stored but its answer is not yet (the first delivery is still running).
	conv := msgs.messages[0].ConversationID
	_ = msgs.mockMessageStore.Create(context.Background(), &domain.Message{ConversationID: conv, Direction: domain.MessageDirectionInbound,
		IdempotencyKey: "k6", Content: &domain.MessageContent{Text: "des ordinateurs portables"}})
	waiting := streamSay(t, context.Background(), p, "k6")
	if waiting.Response.Metadata["in_progress"] != true {
		t.Fatalf("an answer still being written is reported as in progress: %+v", waiting.Response.Metadata)
	}
}

func TestNumbersAreGrounded(t *testing.T) {
	cases := []struct {
		written string
		ok      bool
	}{
		{"Le sac est à 1500 USD", true},       // "1 500" in the facts
		{"Le sac est à 1 500 USD", true},      // same number, other spacing
		{"Livraison en 3 jours", false},       // nobody said 3
		{"Aucune offre pour l’instant.", true}, // no number at all
		{"Deux offres à 12,50 USD", true},
	}
	for _, c := range cases {
		if got := numbersAreGrounded(c.written, "Prix : 1 500 USD le lot, ou 12,50 USD l’unité", `{"total":2}`); got != c.ok {
			t.Errorf("%q: want %v, got %v", c.written, c.ok, got)
		}
	}
}
