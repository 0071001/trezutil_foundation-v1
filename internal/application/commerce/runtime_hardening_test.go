package commerce

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

type echoActorHandler struct{ calls int }

func (h *echoActorHandler) Handle(ctx context.Context, actor domain.CommerceActor, p map[string]interface{}) (interface{}, error) {
	h.calls++
	return map[string]interface{}{"owner": actor.ID}, nil
}
func (h *echoActorHandler) ActionType() domain.CommerceActionType { return domain.ActionGetOrder }

// One actor replaying another actor's idempotency key must not receive the
// other actor's cached result.
func TestRuntime_IdempotencyCacheIsScopedPerActor(t *testing.T) {
	registry := NewHandlerRegistry()
	h := &echoActorHandler{}
	registry.Register(h)
	rt := NewCommerceActionRuntime(RuntimeConfig{Logger: zerolog.Nop(), HandlerRegistry: registry, IdempotencyStore: NewInMemoryIdempotencyStore()})

	alice := domain.NewCommerceActor("alice", domain.ActorTypeCustomer)
	bob := domain.NewCommerceActor("bob", domain.ActorTypeCustomer)

	a := domain.NewCommerceAction(domain.ActionGetOrder, alice, "c1")
	a.IdempotencyKey = "shared-key"
	if _, err := rt.Execute(context.Background(), a, alice); err != nil {
		t.Fatal(err)
	}
	b := domain.NewCommerceAction(domain.ActionGetOrder, bob, "c2")
	b.IdempotencyKey = "shared-key"
	res, err := rt.Execute(context.Background(), b, bob)
	if err != nil {
		t.Fatal(err)
	}
	if res.MessageKey == "ACTION_IDEMPOTENT_CACHE_HIT" {
		t.Fatal("bob must not hit alice's idempotency cache entry")
	}
	if data, _ := res.Data["data"].(map[string]interface{}); data["owner"] != "bob" {
		t.Fatalf("expected bob's own result, got %v", res.Data)
	}
}

func TestRuntime_MissingParameterIsNamed(t *testing.T) {
	rt := NewCommerceActionRuntime(RuntimeConfig{Logger: zerolog.Nop(), HandlerRegistry: NewHandlerRegistry(), ParameterValidator: NewDefaultParameterValidator()})
	actor := domain.NewCommerceActor("u1", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionSearchListings, actor, "c1")
	res, err := rt.Execute(context.Background(), action, actor)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != domain.ActionResultValidationFailed || len(res.Errors) != 1 || res.Errors[0].Field != "text" {
		t.Fatalf("expected validation failure naming 'text', got %+v", res)
	}
}

func TestIntParam_AcceptsJSONAndStringNumbers(t *testing.T) {
	p := map[string]interface{}{"a": 2, "b": float64(3), "c": "4", "d": "5.0", "e": "x"}
	for k, want := range map[string]int{"a": 2, "b": 3, "c": 4, "d": 5} {
		if got, ok := intParam(p, k); !ok || got != want {
			t.Errorf("%s: got %d/%v want %d", k, got, ok, want)
		}
	}
	if _, ok := intParam(p, "e"); ok {
		t.Error("non-numeric must not parse")
	}
}
