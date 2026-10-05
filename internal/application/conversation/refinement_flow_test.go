package conversation

import (
	"context"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

// contextSpy records the conversation state the interpreter is given.
type contextSpy struct {
	inner    scriptedInterpreter
	contexts []string
}

func (s *contextSpy) ClassifyIntent(ctx context.Context, req *IntentClassificationRequest) (*IntentClassificationResult, error) {
	s.contexts = append(s.contexts, req.Context)
	return s.inner.ClassifyIntent(ctx, req)
}

func TestFollowUpsAreReadWithTheContextOfTheLastSearch(t *testing.T) {
	engine := &stayEngine{}
	spy := &contextSpy{inner: hotelScript()}
	p, _ := stayProcessor(engine, &bookingRecorder{}, nil)
	p.classifier = spy

	first := say(t, p, "", hotelRequest)
	cid := first.ConversationID

	// The model is unsure about "moins cher": keyword routing refines the search.
	second := say(t, p, cid, "moins cher")
	if len(engine.queries) != 2 {
		t.Fatalf("searches = %d", len(engine.queries))
	}
	q := engine.queries[1]
	if q.Vertical != domain.VerticalHotel || q.Place != "Gombe" {
		t.Fatalf("the refinement must keep the previous vertical and place: %+v", q)
	}
	if q.Sort == nil || q.Sort.Field != domain.SearchSortFieldPriceAsc {
		t.Fatalf("cheapest first expected: %+v", q.Sort)
	}
	if !strings.Contains(second.Response.Text, "cheapest first") {
		t.Fatalf("the reply states the new ordering: %q", second.Response.Text)
	}

	// The interpreter was given what had just been shown.
	if len(spy.contexts) < 2 {
		t.Fatalf("interpreter calls = %d", len(spy.contexts))
	}
	for _, want := range []string{"Last search: vertical HOTEL", "place Gombe", "2 night(s)", "Listing in focus"} {
		if !strings.Contains(spy.contexts[1], want) {
			t.Errorf("context for the follow-up lacks %q:\n%s", want, spy.contexts[1])
		}
	}
	if strings.Contains(spy.contexts[0], "Last search") {
		t.Errorf("a new conversation has no previous search:\n%s", spy.contexts[0])
	}

	// A budget is a refinement too, and is remembered for the next one.
	say(t, p, cid, "moins de 100 $")
	last := engine.queries[2]
	if last.PriceRange == nil || last.PriceRange.Max == nil || *last.PriceRange.Max != 100 || last.Place != "Gombe" {
		t.Fatalf("budget refinement: %+v", last)
	}
	if last.Sort == nil {
		t.Fatalf("the ordering asked earlier is kept: %+v", last)
	}
}
