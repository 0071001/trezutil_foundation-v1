package conversation

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

// capturingClassifier records the request it was given and stays unsure (keyword routing takes over).
type capturingClassifier struct{ last *IntentClassificationRequest }

func (c *capturingClassifier) ClassifyIntent(_ context.Context, req *IntentClassificationRequest) (*IntentClassificationResult, error) {
	c.last = req
	return &IntentClassificationResult{Fallback: domain.NewConversationResponse("unsure", "fr")}, nil
}

type staticHints struct{ asked []string }

func (h *staticHints) Hints(_ context.Context, userID string) []string {
	h.asked = append(h.asked, userID)
	return []string{"brand: Dell"}
}

func TestTheInterpreterReceivesTheSignedInPersonsMemoryOnly(t *testing.T) {
	classifier, hints := &capturingClassifier{}, &staticHints{}
	p := NewConversationProcessor(ProcessorConfig{
		Logger:            zerolog.Nop(),
		ConversationStore: &mockConvStore{conversations: make(map[string]*domain.Conversation)},
		ContextStore:      &mockContextStore{contexts: make(map[string]domain.ConversationContext)},
		MessageStore:      &mockMessageStore{},
		IntentClassifier:  classifier,
		ActionResolver:    NewActionResolver(NewDefaultIntentValidator()),
		Runtime:           &recordingRuntime{},
		PackResolver:      newMockPackResolver(),
		Memory:            hints,
	})

	say(t, p, "", "des ordinateurs portables")
	if classifier.last == nil || len(classifier.last.Memory) != 1 || classifier.last.Memory[0] != "brand: Dell" {
		t.Fatalf("a signed-in person's memory must reach the interpreter, got %+v", classifier.last)
	}
	if len(hints.asked) != 1 || hints.asked[0] != "user-1" {
		t.Fatalf("the memory must be read for the verified actor only, asked for %v", hints.asked)
	}

	_, err := p.ProcessMessage(context.Background(), &ProcessRequest{
		Channel: domain.ChannelWeb, SenderID: "guest-sender", SenderExternalID: "guest-ext",
		MessageText: "des ordinateurs portables", Actor: domain.NewCommerceActor("", domain.ActorTypeGuest),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(classifier.last.Memory) != 0 || len(hints.asked) != 1 {
		t.Fatalf("a guest has no memory: got %v, asked %v", classifier.last.Memory, hints.asked)
	}
}
