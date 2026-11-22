package ai

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/application/conversation"
	"github.com/trezutil/trezutil/internal/domain"
)

// streamingProvider is a provider that streams its pieces, or fails with err.
type streamingProvider struct {
	pieces []string
	err    error
	calls  int
	last   domain.AIRequest
}

func (p *streamingProvider) ID() string                      { return "stream-test" }
func (p *streamingProvider) Name() string                    { return "Stream test" }
func (p *streamingProvider) Status() domain.AIProviderStatus { return domain.AIProviderStatusActive }
func (p *streamingProvider) IsUsable() bool                  { return true }
func (p *streamingProvider) Models() []domain.AIModel {
	return []domain.AIModel{{ModelID: "m-1", ProviderID: "stream-test", Name: "m-1", Status: domain.AIModelStatusActive, Priority: 1,
		Capabilities: []domain.AIModelCapability{domain.AIModelCapabilityTextGeneration, domain.AIModelCapabilityTextClassification}}}
}
func (p *streamingProvider) Generate(context.Context, domain.AIRequest) (domain.AIResponse, error) {
	return domain.AIResponse{}, domain.NewAIError(domain.AIErrInvalidResponse, "not used")
}
func (p *streamingProvider) GenerateStream(_ context.Context, req domain.AIRequest, onText func(string)) (domain.AIResponse, error) {
	p.calls++
	p.last = req
	if p.err != nil {
		return domain.AIResponse{}, p.err
	}
	for _, s := range p.pieces {
		onText(s)
	}
	return domain.AIResponse{Content: strings.Join(p.pieces, "")}, nil
}

func writerWith(p *streamingProvider) *ReplyWriter {
	router := NewModelRouter(testLogger())
	router.RegisterProvider(p)
	return NewReplyWriter(setupTestGateway(router), nil, testLogger())
}

var writingInput = conversation.ReplyWritingInput{
	Language: "fr", CustomerMessage: "des ordinateurs", ActionType: "SEARCH_LISTINGS",
	Draft: "J’ai trouvé 2 offres.", Facts: `{"cards":[{"title":"Dell"}]}`,
	Actor: domain.NewCommerceActor("u-1", domain.ActorTypeCustomer),
}

func TestReplyWriterStreamsThroughTheGateway(t *testing.T) {
	p := &streamingProvider{pieces: []string{"Voici ", "2 offres."}}
	var got []string
	text, err := writerWith(p).WriteReply(context.Background(), writingInput, func(s string) { got = append(got, s) })
	if err != nil || text != "Voici 2 offres." || strings.Join(got, "|") != "Voici |2 offres." {
		t.Fatalf("text %q pieces %q err %v", text, got, err)
	}
	if p.last.Task != domain.AITaskResponseGeneration || !strings.Contains(p.last.SystemPrompt, "Never invent") {
		t.Fatalf("the request must be a response generation framed by the rules: %+v", p.last.Task)
	}
	if !strings.Contains(p.last.Messages[0].Content, "DRAFT:\nJ’ai trouvé 2 offres.") {
		t.Fatalf("the draft must be given to the model: %q", p.last.Messages[0].Content)
	}
}

func TestReplyWriterPausesAfterAnOutageAndTriesAgainLater(t *testing.T) {
	p := &streamingProvider{err: domain.NewAIError(domain.AIErrProviderUnavailable, "AI provider account is out of credit")}
	w := writerWith(p)
	now := time.Now()
	w.now = func() time.Time { return now }

	if _, err := w.WriteReply(context.Background(), writingInput, nil); err == nil {
		t.Fatal("an outage is an error")
	}
	if _, err := w.WriteReply(context.Background(), writingInput, nil); err == nil || p.calls != 1 {
		t.Fatalf("while paused, the provider is not called again (calls=%d)", p.calls)
	}

	// Credit was added: after the pause, the next answer tries again and writing resumes.
	now = now.Add(replyWriterPause + time.Second)
	p.err, p.pieces = nil, []string{"De retour."}
	if text, err := w.WriteReply(context.Background(), writingInput, nil); err != nil || text != "De retour." || p.calls != 2 {
		t.Fatalf("writing must come back on its own: %q %v calls=%d", text, err, p.calls)
	}
}

func TestOneBadAnswerDoesNotPauseWriting(t *testing.T) {
	p := &streamingProvider{err: domain.NewAIError(domain.AIErrInvalidResponse, "model output was truncated")}
	w := writerWith(p)
	_, _ = w.WriteReply(context.Background(), writingInput, nil)
	_, _ = w.WriteReply(context.Background(), writingInput, nil)
	if p.calls != 2 {
		t.Fatalf("a single bad answer must not pause writing (calls=%d)", p.calls)
	}
}
