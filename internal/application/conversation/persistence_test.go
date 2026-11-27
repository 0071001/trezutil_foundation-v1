package conversation

import (
	"context"
	"errors"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

// brokenMessages fails every write, as an unreachable database would.
type brokenMessages struct{ mockMessageStore }

func (*brokenMessages) Create(context.Context, *domain.Message) error {
	return errors.New("database unavailable")
}

// Without the stored request, a redelivery would run the action again: nothing runs.
func TestNothingRunsWhenTheMessageCannotBeStored(t *testing.T) {
	p := streamProcessor(nil, &brokenMessages{})
	runtime := &recordingRuntime{}
	p.runtime = runtime
	_, err := p.ProcessMessage(context.Background(), &ProcessRequest{Channel: domain.ChannelWeb, SenderID: "s", SenderExternalID: "e",
		MessageText: "des ordinateurs portables", Locale: "fr", Actor: domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)})
	if !errors.Is(err, domain.ErrMessageNotStored) {
		t.Fatalf("expected ErrMessageNotStored, got %v", err)
	}
	if got := len(runtime.actions); got != 0 {
		t.Fatalf("%d action(s) ran", got)
	}
}
