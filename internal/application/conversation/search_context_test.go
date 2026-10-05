package conversation

import (
	"strings"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestBudgetAndOrderingAreReadInFrenchAndEnglish(t *testing.T) {
	cases := []struct {
		text      string
		wantMax   float64
		wantMin   float64
		wantSort  string
		wantVert  string
		wantWords string
	}{
		{"un restaurant à moins de 20 $", 20, 0, "", "RESTAURANT", ""},
		{"un resto pas cher", 0, 0, "price_asc", "RESTAURANT", ""},
		{"a restaurant under $15", 15, 0, "", "RESTAURANT", ""},
		{"un hôtel entre 80 et 150 dollars", 150, 80, "", "HOTEL", ""},
		{"un restaurant autour de 20 $", 24, 16, "", "RESTAURANT", ""},
		{"un électricien à partir de 30 $", 0, 30, "", "SERVICES", ""},
		{"les hôtels haut de gamme", 0, 0, "price_desc", "HOTEL", ""},
		{"un hôtel à 150 dollars maximum", 150, 0, "", "HOTEL", ""},
		{"a restaurant 20 $ max", 20, 0, "", "RESTAURANT", ""},
	}
	for _, c := range cases {
		p := keywordSearchParams(c.text)
		got := func(k string) float64 {
			f, _ := p[k].(float64)
			return f
		}
		if got("price_max") != c.wantMax || got("price_min") != c.wantMin {
			t.Errorf("%q: budget = %v..%v, want %v..%v (%v)", c.text, got("price_min"), got("price_max"), c.wantMin, c.wantMax, p)
		}
		if s, _ := p["sort"].(string); s != c.wantSort {
			t.Errorf("%q: sort = %q, want %q", c.text, s, c.wantSort)
		}
		if v, _ := p["vertical"].(string); v != c.wantVert {
			t.Errorf("%q: vertical = %q, want %q", c.text, v, c.wantVert)
		}
		if txt, _ := p["text"].(string); txt != c.wantWords {
			t.Errorf("%q: words = %q, want %q (price words must not stay in the search text)", c.text, txt, c.wantWords)
		}
	}
	// A bare number is not a price: it could be a size or a count.
	if p := keywordSearchParams("un appartement avec 2 chambres"); p["price_max"] != nil || p["price_min"] != nil {
		t.Fatalf("a count of rooms must not become a budget: %v", p)
	}
}

func TestAFollowUpRefinesThePreviousSearch(t *testing.T) {
	prev := &domain.SearchCriteria{Vertical: domain.VerticalHotel, Location: "Gombe", Nights: 2, PartySize: 2, Text: "wifi"}

	cases := map[string]func(m map[string]interface{}) bool{
		"moins cher": func(m map[string]interface{}) bool {
			return m["sort"] == "price_asc" && m["vertical"] == "HOTEL" && m["location"] == "Gombe" && m["nights"] == 2 && m["text"] == "wifi"
		},
		"seulement à Limete": func(m map[string]interface{}) bool { return m["location"] == "Limete" && m["vertical"] == "HOTEL" },
		"Limete":             func(m map[string]interface{}) bool { return m["location"] == "Limete" && m["party_size"] == 2 },
		"moins de 100 $":     func(m map[string]interface{}) bool { return m["price_max"] == 100.0 && m["location"] == "Gombe" },
		"cheapest first":     func(m map[string]interface{}) bool { return m["sort"] == "price_asc" && m["vertical"] == "HOTEL" },
	}
	for text, ok := range cases {
		merged, isRefinement := refineSearch(prev, text)
		if !isRefinement || !ok(merged) {
			t.Errorf("%q -> %v (refinement=%v)", text, merged, isRefinement)
		}
	}

	// A new request is not a refinement: it names its own kind of offer.
	for _, text := range []string{"je cherche un restaurant à Limete", "un plombier", "find a laptop"} {
		if _, isRefinement := refineSearch(prev, text); isRefinement {
			t.Errorf("%q starts a new search", text)
		}
	}
	if _, isRefinement := refineSearch(nil, "moins cher"); isRefinement {
		t.Error("with no previous search there is nothing to refine")
	}
}

func TestClassifierContextTellsTheModelWhatThePersonJustSaw(t *testing.T) {
	cx := &domain.ConversationContext{
		SearchCriteria: &domain.SearchCriteria{Vertical: domain.VerticalRestaurant, Location: "Limete", PriceMax: 20, Sort: "price_asc"},
		ResultItems: []domain.ResultItemRef{
			{RefType: "listing", ID: "id-1", Title: "Café Mokili", Subtitle: "10 USD"},
			{RefType: "listing", ID: "id-2", Title: "Sushi Kin", Subtitle: "22 USD"},
		},
		MissingParameters: []domain.MissingParameter{{Name: "party_size"}},
	}
	got := classifierContext(cx, false, "fr-CD", time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC))
	for _, want := range []string{
		"Saturday 2026-10-03", "French", "guest",
		"vertical RESTAURANT", "place Limete", "budget up to 20 USD", "cheapest first",
		"1. Café Mokili", "2. Sushi Kin", "[listing id id-2]",
		"party_size",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("context lacks %q:\n%s", want, got)
		}
	}
	if single := classifierContext(&domain.ConversationContext{ResultItems: cx.ResultItems[:1]}, true, "en", time.Now()); !strings.Contains(single, "Listing in focus") || !strings.Contains(single, "Signed in: yes") {
		t.Errorf("a single listing is the one in focus:\n%s", single)
	}
	if empty := classifierContext(nil, true, "en", time.Now()); !strings.Contains(empty, "Today is") {
		t.Errorf("even a new conversation knows today's date: %q", empty)
	}
}

func TestSearchPhraseStatesTheBudgetAndOrdering(t *testing.T) {
	c := map[string]interface{}{"vertical": "RESTAURANT", "location": "Limete", "price_max": 20.0, "sort": "price_asc"}
	if got := searchPhrase("fr", c); got != "restaurants à Limete — à moins de 20 $, du moins cher au plus cher" {
		t.Errorf("fr: %q", got)
	}
	if got := searchPhrase("en", c); got != "restaurants in Limete — under $20, cheapest first" {
		t.Errorf("en: %q", got)
	}
	if got := searchPhrase("en", map[string]interface{}{"sort": "newest"}); got != "your search — newest first" {
		t.Errorf("sort only: %q", got)
	}
}

func TestAModelsSearchIsCompletedWithWhatTheTextStates(t *testing.T) {
	prev := &domain.SearchCriteria{Vertical: domain.VerticalRestaurant, Location: "Limete"}

	// The model returned only the vertical, as models often do.
	got := enrichSearchParams(map[string]interface{}{"vertical": "RESTAURANT"}, "moins cher", prev)
	if got["sort"] != "price_asc" || got["location"] != "Limete" || got["vertical"] != "RESTAURANT" {
		t.Errorf("moins cher: %v", got)
	}

	got = enrichSearchParams(map[string]interface{}{"vertical": "RESTAURANT"}, "plutôt à Gombe", prev)
	if got["location"] != "Gombe" {
		t.Errorf("a new place replaces the old one: %v", got)
	}

	got = enrichSearchParams(map[string]interface{}{"vertical": "RETAIL"}, "un téléphone Samsung max 300 dollars", prev)
	if got["price_max"] != 300.0 || got["location"] != nil {
		t.Errorf("a new request does not inherit the old place: %v", got)
	}

	// What the model did return is kept.
	got = enrichSearchParams(map[string]interface{}{"vertical": "HOTEL", "location": "Ngaliema", "price_max": 90.0}, "moins de 200 $ à Gombe", prev)
	if got["location"] != "Ngaliema" || got["price_max"] != 90.0 {
		t.Errorf("the model's values win: %v", got)
	}

	// No previous search and nothing to read: unchanged.
	if got := enrichSearchParams(map[string]interface{}{"vertical": "HOTEL"}, "un hôtel", nil); len(got) != 1 {
		t.Errorf("nothing to add: %v", got)
	}
	if got := enrichSearchParams(nil, "", nil); got == nil || len(got) != 0 {
		t.Errorf("nil params: %v", got)
	}
}

func TestProductWordsAreKeptWhenTheModelLeavesThemOut(t *testing.T) {
	got := enrichSearchParams(map[string]interface{}{"vertical": "RETAIL"}, "un téléphone Samsung max 300 dollars", nil)
	if got["text"] != "téléphone Samsung" || got["price_max"] != 300.0 {
		t.Errorf("product words: %v", got)
	}
	// Time words are not a thing to search for.
	if got := enrichSearchParams(map[string]interface{}{}, "une chambre disponible ce soir", nil); got["text"] != "chambre" {
		t.Errorf("time words dropped: %v", got)
	}
	// A named vertical already carries the meaning: no words are added.
	if got := enrichSearchParams(map[string]interface{}{"vertical": "SERVICES"}, "un plombier disponible aujourd’hui", nil); got["text"] != nil {
		t.Errorf("a plumber search has no extra words: %v", got)
	}
}

func TestAnAvailabilityQuestionNeedsNoAccount(t *testing.T) {
	if !guestAllowedActions[domain.ActionCheckAvailability] {
		t.Fatal("asking whether something is free is browsing")
	}
	if guestAllowedActions[domain.ActionCreateReservation] || guestAllowedActions[domain.ActionCreateOrder] {
		t.Fatal("booking and ordering still need an account")
	}
}

func TestAReplyIsAnAnswerOnlyWhenItFitsTheQuestion(t *testing.T) {
	yes := map[string][]string{
		"end_at":     {"2026-10-12 11:00", "demain", "samedi soir", "next friday"},
		"nights":     {"2", "deux nuits", "une semaine", "3 nights"},
		"party_size": {"4 personnes", "just two", "seul"},
		"location":   {"Gombe", "à Limete", "du côté de Ngaliema"},
		"vertical":   {"un hôtel", "a restaurant"},
		"listing_id": {"2", "le deuxième", "the second one"},
	}
	for param, replies := range yes {
		for _, r := range replies {
			if !answersMissingParam(param, r) {
				t.Errorf("%s: %q should be an answer", param, r)
			}
		}
	}
	no := map[string][]string{
		"end_at":   {"un téléphone Samsung max 300 dollars", "merci", "je cherche un restaurant à Gombe"},
		"nights":   {"plutôt un restaurant", "je cherche un hôtel pas cher à Gombe pour ce soir"},
		"location": {"je cherche un téléphone samsung", "find me a cheap hotel please now"},
	}
	for param, replies := range no {
		for _, r := range replies {
			if answersMissingParam(param, r) {
				t.Errorf("%s: %q starts a new request", param, r)
			}
		}
	}
}
