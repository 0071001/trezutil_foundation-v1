package conversation

import (
	"context"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/domain"
)

// Structured stay intents run through the real CommerceActionRuntime
// (parameter validation with the vertical catalogue, confirmation policy)
// and the real search action; only the interpreter and search engine are
// scripted.

// scriptedInterpreter answers like the model: one structured intent per message.
type scriptedInterpreter map[string]*domain.AIIntent

func (s scriptedInterpreter) ClassifyIntent(ctx context.Context, req *IntentClassificationRequest) (*IntentClassificationResult, error) {
	text := req.Messages[len(req.Messages)-1].Content.Text
	if intent, ok := s[text]; ok {
		return &IntentClassificationResult{Intent: intent}, nil
	}
	return &IntentClassificationResult{Fallback: domain.NewConversationResponse("unsure", "en")}, nil
}

func aiIntent(action domain.CommerceActionType, params map[string]interface{}) *domain.AIIntent {
	i := domain.NewAIIntent()
	i.Intent = action
	i.Confidence = 0.95
	i.Parameters = params
	return i
}

// stayEngine records search queries; with emptyForPlace, place-filtered
// searches find nothing (a store without an address).
type stayEngine struct {
	queries       []domain.DiscoveryQuery
	emptyForPlace bool
}

func (e *stayEngine) ExecuteSearch(ctx context.Context, actorID string, q domain.DiscoveryQuery) (domain.SearchResultPage, error) {
	e.queries = append(e.queries, q)
	if e.emptyForPlace && q.Place != "" {
		return domain.SearchResultPage{}, nil
	}
	return domain.SearchResultPage{Total: 1, Items: []domain.SearchResult{{ListingID: "room-1", Title: "Deluxe room", Vertical: "HOTEL"}}}, nil
}

func (e *stayEngine) Suggest(ctx context.Context, actorID string, req domain.SuggestRequest) (domain.SuggestResponse, error) {
	return domain.SuggestResponse{}, nil
}

// bookingRecorder stands in for the reservation use case.
type bookingRecorder struct{ params []map[string]interface{} }

func (b *bookingRecorder) ActionType() domain.CommerceActionType {
	return domain.ActionCreateReservation
}
func (b *bookingRecorder) Handle(ctx context.Context, actor domain.CommerceActor, p map[string]interface{}) (interface{}, error) {
	b.params = append(b.params, p)
	return map[string]interface{}{"reservation_id": "res-1", "status": "confirmed"}, nil
}

type listingStub struct{}

func (listingStub) ActionType() domain.CommerceActionType { return domain.ActionGetListing }
func (listingStub) Handle(ctx context.Context, actor domain.CommerceActor, p map[string]interface{}) (interface{}, error) {
	return &domain.Listing{ID: "room-1", Title: "Deluxe room", Vertical: domain.VerticalHotel, PriceAmount: 90, PriceCurrency: "USD"}, nil
}

func stayProcessor(engine *stayEngine, bookings *bookingRecorder, script scriptedInterpreter) (*ConversationProcessor, *mockContextStore) {
	registry := commerce.NewHandlerRegistry()
	registry.Register(commerce.NewSearchHandler(engine))
	registry.Register(listingStub{})
	registry.Register(bookings)
	runtime := commerce.NewCommerceActionRuntime(commerce.RuntimeConfig{
		Logger:             zerolog.Nop(),
		HandlerRegistry:    registry,
		IdempotencyStore:   commerce.NewInMemoryIdempotencyStore(),
		ConfirmationPolicy: commerce.NewDefaultConfirmationPolicy(),
		ParameterValidator: commerce.NewDefaultParameterValidator(),
	})
	contexts := &mockContextStore{contexts: make(map[string]domain.ConversationContext)}
	return NewConversationProcessor(ProcessorConfig{
		Logger:            zerolog.Nop(),
		ConversationStore: &mockConvStore{conversations: make(map[string]*domain.Conversation)},
		ContextStore:      contexts,
		MessageStore:      &mockMessageStore{},
		IntentClassifier:  script,
		ActionResolver:    NewActionResolver(NewDefaultIntentValidator()),
		Runtime:           runtime,
		PackResolver:      newMockPackResolver(),
	}), contexts
}

const hotelRequest = "Find me a hotel in Gombe for two nights for two people"

func hotelScript() scriptedInterpreter {
	return scriptedInterpreter{
		// JSON numbers arrive as float64 from the model.
		hotelRequest:              aiIntent(domain.ActionSearchListings, map[string]interface{}{"vertical": "HOTEL", "location": "Gombe", "nights": float64(2), "party_size": float64(2)}),
		"casino in Gombe":         aiIntent(domain.ActionSearchListings, map[string]interface{}{"vertical": "CASINO", "location": "Gombe"}),
		"book it from 2026-10-10": aiIntent(domain.ActionCreateReservation, map[string]interface{}{"start_at": "2026-10-10"}),
	}
}

func TestHotelIntent_StructuredSearch(t *testing.T) {
	engine := &stayEngine{}
	p, contexts := stayProcessor(engine, &bookingRecorder{}, hotelScript())

	res := say(t, p, "", hotelRequest)
	if len(engine.queries) != 1 {
		t.Fatalf("searches = %d", len(engine.queries))
	}
	q := engine.queries[0]
	if q.Vertical != domain.VerticalHotel || q.Place != "Gombe" || q.Text != "" {
		t.Fatalf("query = %+v", q)
	}
	if !strings.Contains(res.Response.Text, "hotels & accommodation in Gombe for 2 nights, 2 guests") {
		t.Fatalf("reply = %q", res.Response.Text)
	}
	cx := contexts.contexts[res.ConversationID]
	if cx.SearchCriteria == nil || *cx.SearchCriteria != (domain.SearchCriteria{Vertical: domain.VerticalHotel, Location: "Gombe", Nights: 2, PartySize: 2}) {
		t.Fatalf("criteria = %+v", cx.SearchCriteria)
	}
}

func TestHotelIntent_UnknownVerticalIsRefusedNotOther(t *testing.T) {
	engine := &stayEngine{}
	p, _ := stayProcessor(engine, &bookingRecorder{}, hotelScript())

	res := say(t, p, "", "casino in Gombe")
	if len(engine.queries) != 0 {
		t.Fatalf("an unknown vertical must not reach search: %+v", engine.queries)
	}
	if !res.IsClarification || len(res.MissingParams) == 0 || res.MissingParams[0].Name != "vertical" {
		t.Fatalf("expected a clarification about the vertical, got %+v", res)
	}
}

func TestHotelIntent_PlaceRelaxedVisibly(t *testing.T) {
	engine := &stayEngine{emptyForPlace: true}
	p, _ := stayProcessor(engine, &bookingRecorder{}, hotelScript())

	res := say(t, p, "", hotelRequest)
	if len(engine.queries) != 2 || engine.queries[1].Place != "" || engine.queries[1].Text != "Gombe" ||
		engine.queries[1].Vertical != domain.VerticalHotel {
		t.Fatalf("relaxed search = %+v", engine.queries)
	}
	if !strings.Contains(res.Response.Text, "I didn't find anything listed in Gombe yet") {
		t.Fatalf("the relaxation must be stated: %q", res.Response.Text)
	}
}

func TestHotelIntent_BookingReusesStayCriteria(t *testing.T) {
	bookings := &bookingRecorder{}
	p, _ := stayProcessor(&stayEngine{}, bookings, hotelScript())

	res := say(t, p, "", hotelRequest)
	say(t, p, res.ConversationID, "1") // opens the room from the results
	booking := say(t, p, res.ConversationID, "book it from 2026-10-10")
	if !booking.IsConfirmation {
		t.Fatalf("a booking must be confirmed first: %q", booking.Response.Text)
	}
	say(t, p, res.ConversationID, "yes")
	if len(bookings.params) != 1 {
		t.Fatalf("bookings = %d", len(bookings.params))
	}
	b := bookings.params[0]
	checkIn := int64(1791590400) // 2026-10-10T00:00:00Z
	if b["listing_id"] != "room-1" || b["party_size"] != 2 || b["start_at"] != checkIn || b["end_at"] != checkIn+2*86400 {
		t.Fatalf("booking parameters = %v", b)
	}
}

func TestKeywordFallback_DetectsVertical(t *testing.T) {
	classify := func(text string) map[string]interface{} {
		res, err := (&KeywordClassifier{}).ClassifyIntent(context.Background(), &IntentClassificationRequest{
			Messages: []domain.Message{{Content: &domain.MessageContent{Text: text}}},
		})
		if err != nil || res.Intent == nil {
			t.Fatalf("%q: %v", text, err)
		}
		return res.Intent.Parameters
	}
	if p := classify("je cherche un hôtel à Gombe"); p["vertical"] != "HOTEL" || p["location"] != "Gombe" || p["text"] != nil {
		t.Fatalf("hotel: %v", p)
	}
	if p := classify("find a restaurant"); p["vertical"] != "RESTAURANT" || p["text"] != nil {
		t.Fatalf("restaurant: %v", p)
	}
	// Two verticals named at once: no guess, plain text search.
	if p := classify("find a hotel with a restaurant"); p["vertical"] != nil || p["text"] != "find a hotel with a restaurant" {
		t.Fatalf("ambiguous: %v", p)
	}
	if p := classify("find a phone"); p["vertical"] != nil || p["text"] != "find a phone" {
		t.Fatalf("no vertical: %v", p)
	}
}
