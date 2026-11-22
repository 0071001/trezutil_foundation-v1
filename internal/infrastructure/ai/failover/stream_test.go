package failover

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

type streamer struct {
	id     string
	pieces []string
	err    error
	calls  int
}

func (s *streamer) ID() string                      { return s.id }
func (s *streamer) Name() string                    { return s.id }
func (s *streamer) Status() domain.AIProviderStatus { return domain.AIProviderStatusActive }
func (s *streamer) IsUsable() bool                  { return true }
func (s *streamer) Models() []domain.AIModel        { return []domain.AIModel{{ModelID: s.id + "-m"}} }
func (s *streamer) Generate(context.Context, domain.AIRequest) (domain.AIResponse, error) {
	return domain.AIResponse{}, nil
}
func (s *streamer) GenerateStream(_ context.Context, _ domain.AIRequest, onText func(string)) (domain.AIResponse, error) {
	s.calls++
	for _, p := range s.pieces {
		onText(p)
	}
	if s.err != nil {
		return domain.AIResponse{}, s.err
	}
	return domain.AIResponse{Content: "ok"}, nil
}

func TestStreamFallsBackOnlyBeforeAnyText(t *testing.T) {
	down := domain.NewAIError(domain.AIErrProviderUnavailable, "down")

	// Nothing sent yet: the next provider answers.
	first, second := &streamer{id: "a", err: down}, &streamer{id: "b", pieces: []string{"ok"}}
	p, _ := New(zerolog.Nop(), first, second)
	if _, err := p.GenerateStream(context.Background(), domain.AIRequest{}, func(string) {}); err != nil || second.calls != 1 {
		t.Fatalf("a provider failing before any text must hand over: err %v, second called %d", err, second.calls)
	}

	// Text already shown: the failure is returned, never answered again by another provider.
	first, second = &streamer{id: "a", pieces: []string{"Voi"}, err: down}, &streamer{id: "b", pieces: []string{"ok"}}
	p, _ = New(zerolog.Nop(), first, second)
	if _, err := p.GenerateStream(context.Background(), domain.AIRequest{}, func(string) {}); err == nil || second.calls != 0 {
		t.Fatalf("after text was sent, no other provider may answer: err %v, second called %d", err, second.calls)
	}
}
