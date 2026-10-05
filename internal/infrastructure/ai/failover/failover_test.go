package failover

import (
	"context"
	"errors"
	"testing"

	"github.com/rs/zerolog"

	"github.com/trezutil/trezutil/internal/domain"
)

type stub struct {
	id     string
	model  string
	err    error
	usable bool
	calls  int
	gotMdl string
}

func (s *stub) ID() string                      { return s.id }
func (s *stub) Name() string                    { return s.id }
func (s *stub) Status() domain.AIProviderStatus { return domain.AIProviderStatusActive }
func (s *stub) IsUsable() bool                  { return s.usable }
func (s *stub) Models() []domain.AIModel {
	return []domain.AIModel{{ModelID: s.model, ProviderID: s.id, Status: domain.AIModelStatusActive}}
}
func (s *stub) Generate(ctx context.Context, req domain.AIRequest) (domain.AIResponse, error) {
	s.calls++
	s.gotMdl = req.ModelID
	if s.err != nil {
		return domain.AIResponse{}, s.err
	}
	return domain.AIResponse{Content: "ok from " + s.id, ProviderID: s.id}, nil
}

func TestFailsOverToTheNextProviderWithItsOwnModel(t *testing.T) {
	a := &stub{id: "a", model: "model-a", usable: true, err: domain.NewAIError(domain.AIErrInvalidResponse, "no credit")}
	b := &stub{id: "b", model: "model-b", usable: true}
	p, err := New(zerolog.Nop(), a, b)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := p.Generate(context.Background(), domain.AIRequest{ModelID: "model-a"})
	if err != nil || resp.ProviderID != "b" {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
	if a.calls != 1 || b.calls != 1 || b.gotMdl != "model-b" {
		t.Fatalf("a=%d b=%d model sent to b=%q", a.calls, b.calls, b.gotMdl)
	}
}

func TestFirstProviderWinsWhenItWorks(t *testing.T) {
	a := &stub{id: "a", model: "model-a", usable: true}
	b := &stub{id: "b", model: "model-b", usable: true}
	p, _ := New(zerolog.Nop(), a, b)
	if resp, err := p.Generate(context.Background(), domain.AIRequest{}); err != nil || resp.ProviderID != "a" || b.calls != 0 {
		t.Fatalf("resp=%+v err=%v b.calls=%d", resp, err, b.calls)
	}
}

func TestReturnsTheLastErrorWhenEveryProviderFails(t *testing.T) {
	first := domain.NewAIError(domain.AIErrProviderRateLimited, "first")
	last := domain.NewAIError(domain.AIErrProviderUnavailable, "last")
	p, _ := New(zerolog.Nop(), &stub{id: "a", model: "m", usable: true, err: first}, &stub{id: "b", model: "m", usable: true, err: last})
	_, err := p.Generate(context.Background(), domain.AIRequest{})
	var ae domain.AIError
	if !errors.As(err, &ae) || ae.Message != "last" {
		t.Fatalf("err = %v", err)
	}
}

func TestSkipsUnusableProvidersAndRefusesAnEmptyList(t *testing.T) {
	if _, err := New(zerolog.Nop()); err == nil {
		t.Fatal("an empty chain must be refused")
	}
	p, _ := New(zerolog.Nop(), &stub{id: "a", model: "m", usable: false}, &stub{id: "b", model: "m", usable: true})
	if resp, err := p.Generate(context.Background(), domain.AIRequest{}); err != nil || resp.ProviderID != "b" {
		t.Fatalf("resp=%+v err=%v", resp, err)
	}
	if p.Models()[0].ProviderID != ID {
		t.Fatal("the composed provider advertises itself to the router")
	}
}
