package audit

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/observability"
)

func TestRecordWritesAStructuredLineWithTheRequestID(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(zerolog.New(&buf))
	ctx := observability.ContextWithRequestID(context.Background(), "req-9")
	err := l.Record(ctx, domain.AuditEntry{ActorID: "u1", Action: "order.created", TargetType: "order", TargetID: "o1",
		Metadata: map[string]interface{}{"items": 3}})
	if err != nil {
		t.Fatal(err)
	}
	line := buf.String()
	for _, want := range []string{`"event":"audit"`, `"action":"order.created"`, `"actor_id":"u1"`, `"target_id":"o1"`, `"request_id":"req-9"`, `"items":3`} {
		if !strings.Contains(line, want) {
			t.Errorf("missing %s in %s", want, line)
		}
	}
}
