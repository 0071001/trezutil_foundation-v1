package conversation

import (
	"context"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func searchData(criteria map[string]interface{}, titles ...string) map[string]interface{} {
	items := make([]interface{}, 0, len(titles))
	for _, t := range titles {
		items = append(items, map[string]interface{}{"id": "id-" + t, "title": t})
	}
	data := map[string]interface{}{"items": items}
	if criteria != nil {
		data["criteria"] = criteria
	}
	return data
}

func TestPickIsStableAndSpreadsAcrossOptions(t *testing.T) {
	if pick("same", "a", "b", "c") != pick("same", "a", "b", "c") {
		t.Fatal("the same request must always get the same wording")
	}
	seen := map[string]bool{}
	for _, seed := range []string{"x1", "x2", "x3", "x4", "x5", "x6", "x7", "x8", "x9", "x10"} {
		seen[pick(seed, "a", "b", "c")] = true
	}
	if len(seen) < 2 {
		t.Fatalf("wording never varies: %v", seen)
	}
	if pick("seed") != "" {
		t.Fatal("no options, no text")
	}
}

func TestSearchPhraseReadsLikeSpeech(t *testing.T) {
	got := searchPhrase("en", map[string]interface{}{"vertical": string(domain.VerticalHotel), "location": "Gombe", "nights": 2, "party_size": 1})
	if got != "hotels & accommodation in Gombe for 2 nights, 1 guest" {
		t.Fatalf("phrase = %q", got)
	}
	if searchPhrase("en", nil) != "" || searchPhrase("en", map[string]interface{}{}) != "" {
		t.Fatal("no criteria, no phrase")
	}
}

func TestSearchTextCountsAndInvitesAFollowUp(t *testing.T) {
	data := searchData(nil, "Alpha", "Beta", "Gamma")
	text := successText("en", domain.ActionSearchListings, data)
	for _, want := range []string{"3 options", "1. Alpha", "2. Beta", "3. Gamma"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
	if !strings.Contains(text, "number") {
		t.Fatalf("the reply must tell the person how to continue: %q", text)
	}
}

func TestSearchTextWhenNothingMatchesSuggestsWhatToTry(t *testing.T) {
	text := successText("en", domain.ActionSearchListings, searchData(nil))
	if !strings.Contains(text, "Gombe") && !strings.Contains(text, "category") {
		t.Fatalf("an empty result must help the person move on: %q", text)
	}
	if len(suggestionsFor("en", domain.ActionSearchListings, searchData(nil))) < 3 {
		t.Fatal("an empty result offers things to try")
	}
}

func TestSuggestionsAreShortAndContextual(t *testing.T) {
	hotel := suggestionsFor("en", domain.ActionSearchListings, searchData(map[string]interface{}{"vertical": string(domain.VerticalHotel)}, "A", "B"))
	food := suggestionsFor("en", domain.ActionSearchListings, searchData(map[string]interface{}{"vertical": string(domain.VerticalRestaurant)}, "A", "B"))
	if strings.Join(hotel, "|") == strings.Join(food, "|") {
		t.Fatalf("hotel and restaurant results must not share follow-ups: %v", hotel)
	}
	for _, list := range [][]string{hotel, food, suggestionsFor("en", domain.ActionCreateOrder, nil), clarificationSuggestions("en", "nights")} {
		if len(list) == 0 || len(list) > 4 {
			t.Fatalf("1 to 4 suggestions expected, got %v", list)
		}
		for _, s := range list {
			if len(s) > 60 || strings.TrimSpace(s) == "" {
				t.Fatalf("a suggestion must be a short message: %q", s)
			}
		}
	}
	if suggestionsFor("en", domain.ActionPauseListing, nil) != nil {
		t.Fatal("no follow-up for seller housekeeping")
	}
}

func TestListingTextAsksTheNaturalNextQuestion(t *testing.T) {
	desc := "River view"
	product := &domain.Listing{Title: "Phone", Description: &desc, ListingType: domain.ListingTypePhysicalProduct, PriceAmount: 10, PriceCurrency: "USD"}
	if !strings.Contains(listingText("en", product), "buy it") || strings.Contains(listingText("en", product), "book it") {
		t.Fatalf("a product is bought, not booked: %q", listingText("en", product))
	}
	service := &domain.Listing{Title: "Plumber", ListingType: domain.ListingTypeService}
	if !strings.Contains(listingText("en", service), "book it") {
		t.Fatalf("a service is booked: %q", listingText("en", service))
	}
}

func TestClarificationPromptsStayConversational(t *testing.T) {
	for name, prompt := range friendlyParamPrompts {
		if strings.Contains(strings.ToLower(prompt), "i need") || strings.TrimSpace(prompt) == "" {
			t.Fatalf("%s: %q reads like a form error", name, prompt)
		}
	}
}

func TestRepliesSpeakFrench(t *testing.T) {
	text := successText("fr-CD", domain.ActionSearchListings, searchData(map[string]interface{}{"vertical": string(domain.VerticalHotel), "location": "Gombe", "nights": 2, "party_size": 2}, "Alpha", "Beta"))
	for _, want := range []string{"hôtels et hébergements à Gombe pour 2 nuits, 2 voyageurs", "1. Alpha", "2. Beta"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in %q", want, text)
		}
	}
	if strings.Contains(text, "options for") || strings.Contains(text, "Reply with") {
		t.Fatalf("a French conversation must not get English wording: %q", text)
	}
	if got := successText("fr", domain.ActionCreateReservation, nil); !strings.Contains(got, "confirmée") {
		t.Fatalf("booking reply = %q", got)
	}
	if len(suggestionsFor("fr", domain.ActionSearchListings, searchData(nil))) < 3 || suggestionsFor("fr", domain.ActionSearchListings, searchData(nil))[0] != "Des hôtels à Gombe" {
		t.Fatal("French suggestions expected")
	}
	if clarificationSuggestions("fr", "nights")[1] != "2 nuits" {
		t.Fatal("French answers expected for the nights question")
	}
	if friendlyParamPromptsFR["nights"] == "" {
		t.Fatal("French prompt for nights expected")
	}
}

func TestLanguageFollowsTheLocaleAndFallsBackToTheDefault(t *testing.T) {
	old := DefaultLanguage
	defer func() { DefaultLanguage = old }()
	DefaultLanguage = "fr"
	cases := []struct{ locale, current, want string }{
		{"en-US", "fr", "en"}, {"fr-CD", "en", "fr"}, {"FR", "", "fr"}, {"", "en", "en"}, {"", "", "fr"}, {"sw", "", "fr"},
	}
	for _, c := range cases {
		if got := resolveLanguage(c.locale, c.current); got != c.want {
			t.Errorf("resolveLanguage(%q,%q) = %q, want %q", c.locale, c.current, got, c.want)
		}
	}
}

func TestKeywordSearchUnderstandsFrenchRequests(t *testing.T) {
	p := keywordSearchParams("Je cherche un hôtel à Gombe")
	if p["vertical"] != "HOTEL" || p["location"] != "Gombe" || p["text"] != nil {
		t.Fatalf("params = %v", p)
	}
	stay := keywordSearchParams("Je cherche un hôtel à Gombe pour 2 nuits pour 3 personnes")
	if stay["nights"] != 2 || stay["party_size"] != 3 || stay["location"] != "Gombe" || stay["text"] != nil {
		t.Fatalf("stay params = %v", stay)
	}
	if food := keywordSearchParams("Montre-moi un restaurant à Gombe"); food["vertical"] != "RESTAURANT" || food["location"] != "Gombe" {
		t.Fatalf("restaurant params = %v", food)
	}
	if got := resolveActionTypeKeyword("Montre-moi des produits Samsung"); got != domain.ActionSearchListings {
		t.Fatalf("a French show request is a search, got %q", got)
	}
	if got := resolveActionTypeKeyword("Je voudrais réserver"); got != domain.ActionCreateReservation {
		t.Fatalf("réserver is a booking, got %q", got)
	}
	if got := resolveActionTypeKeyword("Achète-le"); got != domain.ActionCreateOrder {
		t.Fatalf("achète-le is an order, got %q", got)
	}
}

func TestKeywordRoutingTreatsANamedKindOfPlaceAsASearch(t *testing.T) {
	p := &ConversationProcessor{}
	for _, text := range []string{"Un plombier disponible aujourd’hui", "Un resto congolais pas cher du côté de Limete"} {
		intent := keywordIntent(p, context.Background(), &domain.Conversation{}, nil, &ProcessRequest{MessageText: text})
		if intent == nil || intent.ActionType != domain.ActionSearchListings {
			t.Fatalf("%q -> %+v", text, intent)
		}
	}
}
